package project

import (
	"context"
	"fmt"
	"strings"

	projectdomain "clawx/internal/domain/project"
)

func (s *Service) SuggestProjectSwitch(
	ctx context.Context,
	routeKey, fromProjectID, toProjectID, reason string,
	confidence float64,
	createdBy string,
) (projectdomain.Proposal, error) {
	if s.proposalRepo == nil {
		return projectdomain.Proposal{}, fmt.Errorf("project proposal repository is required")
	}

	routeKey = normalizeRouteKey(routeKey)
	fromProjectID = normalizeProjectID(fromProjectID)
	toProjectID = normalizeProjectID(toProjectID)
	reason = strings.TrimSpace(reason)
	if routeKey == "" || toProjectID == "" {
		return projectdomain.Proposal{}, projectdomain.ErrInvalidProject
	}
	if reason == "" {
		reason = "intent_project_switch"
	}
	_ = strings.TrimSpace(createdBy)

	if fromProjectID == "" {
		currentProjectID, _, err := s.ResolveProject(ctx, routeKey)
		if err != nil {
			return projectdomain.Proposal{}, err
		}
		fromProjectID = normalizeProjectID(currentProjectID)
	}
	if fromProjectID == toProjectID {
		return projectdomain.Proposal{}, projectdomain.ErrInvalidProject
	}

	targetProject, err := s.GetProject(ctx, toProjectID)
	if err != nil {
		return projectdomain.Proposal{}, err
	}
	if targetProject.Status == projectdomain.StatusBroken {
		return projectdomain.Proposal{}, projectdomain.ErrInvalidProject
	}

	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	now := s.clock()
	proposal := projectdomain.Proposal{
		ID:            fmt.Sprintf("proposal-%d", now.UTC().UnixNano()),
		RouteKey:      routeKey,
		FromProjectID: fromProjectID,
		ToProjectID:   toProjectID,
		Confidence:    confidence,
		Reason:        reason,
		CreatedAt:     now,
		ExpiresAt:     now.Add(s.proposalTTL),
		Status:        projectdomain.ProposalPending,
	}
	if err := s.proposalRepo.Upsert(ctx, proposal); err != nil {
		return projectdomain.Proposal{}, err
	}
	return proposal, nil
}

func (s *Service) ConfirmProjectSwitch(ctx context.Context, proposalID, updatedBy string) (projectdomain.RouteBinding, error) {
	if s.proposalRepo == nil {
		return projectdomain.RouteBinding{}, fmt.Errorf("project proposal repository is required")
	}

	proposalID = strings.TrimSpace(proposalID)
	if proposalID == "" {
		return projectdomain.RouteBinding{}, projectdomain.ErrInvalidProject
	}

	proposal, err := s.proposalRepo.GetByID(ctx, proposalID)
	if err != nil {
		return projectdomain.RouteBinding{}, err
	}

	now := s.clock()
	if proposal.Status == "" {
		proposal.Status = projectdomain.ProposalPending
	}
	if proposal.Status != projectdomain.ProposalPending {
		return projectdomain.RouteBinding{}, projectdomain.ErrProposalInvalid
	}
	if !proposal.ExpiresAt.IsZero() && !proposal.ExpiresAt.After(now) {
		proposal.Status = projectdomain.ProposalExpired
		_ = s.proposalRepo.Upsert(ctx, proposal)
		return projectdomain.RouteBinding{}, projectdomain.ErrProposalExpired
	}

	binding, err := s.UseProject(ctx, proposal.RouteKey, proposal.ToProjectID, updatedBy)
	if err != nil {
		return projectdomain.RouteBinding{}, err
	}
	proposal.Status = projectdomain.ProposalAccepted
	if err := s.proposalRepo.Upsert(ctx, proposal); err != nil {
		return projectdomain.RouteBinding{}, err
	}
	return binding, nil
}

func (s *Service) ExpireProposals(ctx context.Context) (int, error) {
	if s.proposalRepo == nil {
		return 0, nil
	}

	proposals, err := s.proposalRepo.List(ctx)
	if err != nil {
		return 0, err
	}
	now := s.clock()
	expired := 0
	for _, proposal := range proposals {
		status := proposal.Status
		if status == "" {
			status = projectdomain.ProposalPending
		}
		if status != projectdomain.ProposalPending {
			continue
		}
		if proposal.ExpiresAt.IsZero() || proposal.ExpiresAt.After(now) {
			continue
		}

		proposal.Status = projectdomain.ProposalExpired
		if err := s.proposalRepo.Upsert(ctx, proposal); err != nil {
			return expired, err
		}
		expired++
	}
	return expired, nil
}
