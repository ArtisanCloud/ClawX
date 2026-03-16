package memory

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

func (s *CommandService) Digest(ctx context.Context, input DigestInput) (DigestResult, error) {
	scoped, err := s.resolveScopeContext(ctx, scopeInput{
		RouteKey:        input.RouteKey,
		ProjectID:       input.ProjectID,
		AgentID:         input.AgentID,
		UserID:          input.UserID,
		IsDirectMessage: input.IsDirectMessage,
	})
	if err != nil {
		return DigestResult{}, err
	}

	now := s.now().UTC()
	jobID := fmt.Sprintf("digest-%d", now.UnixNano())
	job := memorydomain.DigestJob{
		JobID:       jobID,
		ScopeKey:    scoped.scope,
		TriggerMode: memorydomain.DigestTriggerManual,
		Status:      memorydomain.DigestPending,
		StartedAt:   now,
	}

	if s.digestRepo != nil {
		existsRunning, checkErr := s.hasRunningDigest(ctx, scoped.projectID)
		if checkErr != nil {
			return DigestResult{}, checkErr
		}
		if existsRunning {
			return DigestResult{}, newCommandError("digest_in_progress", "another digest job is still running")
		}
		if err := s.digestRepo.Upsert(ctx, job); err != nil {
			return DigestResult{}, err
		}
		job.Status = memorydomain.DigestRunning
		if err := s.digestRepo.Upsert(ctx, job); err != nil {
			return DigestResult{}, err
		}
	}

	// Digest writes into long-term private memory, so it requires main-session ACL.
	if !scoped.classification.AllowMainPrivate || scoped.classification.Degraded || !scoped.classification.OwnerMatched {
		job.Status = memorydomain.DigestRejectedACL
		job.EndedAt = s.now().UTC()
		if s.digestRepo != nil {
			_ = s.digestRepo.Upsert(ctx, job)
		}
		s.appendAuditRecord(ctx, memorydomain.AuditRecord{
			Timestamp:   job.EndedAt,
			ScopeKey:    scoped.scope,
			DeniedFiles: []string{"memory digest|rejected_acl"},
			ACLMode:     scoped.profile.ACLMode,
			Degraded:    true,
		})
		return DigestResult{}, newCommandError("rejected_acl", "digest requires main owner session")
	}

	outputFile, err := s.writeDigestSummary(scoped, now)
	if err != nil {
		job.Status = memorydomain.DigestFailed
		job.EndedAt = s.now().UTC()
		if s.digestRepo != nil {
			_ = s.digestRepo.Upsert(ctx, job)
		}
		s.appendAuditRecord(ctx, memorydomain.AuditRecord{
			Timestamp:  job.EndedAt,
			ScopeKey:   scoped.scope,
			ErrorFiles: []string{fmt.Sprintf("digest_failed:%v", err)},
			ACLMode:    scoped.profile.ACLMode,
			Degraded:   true,
		})
		return DigestResult{}, err
	}

	job.Status = memorydomain.DigestCompleted
	job.OutputFile = outputFile
	job.EndedAt = s.now().UTC()
	if s.digestRepo != nil {
		if err := s.digestRepo.Upsert(ctx, job); err != nil {
			return DigestResult{}, err
		}
	}

	s.appendAuditRecord(ctx, memorydomain.AuditRecord{
		Timestamp:   job.EndedAt,
		ScopeKey:    scoped.scope,
		LoadedFiles: []string{outputFile},
		ACLMode:     scoped.profile.ACLMode,
		Degraded:    false,
	})

	return DigestResult{
		JobID:             job.JobID,
		Status:            job.Status,
		OutputFile:        outputFile,
		AutoDigestEnabled: s.autoDigestEnabled,
	}, nil
}

func (s *CommandService) hasRunningDigest(ctx context.Context, projectID string) (bool, error) {
	if s.digestRepo == nil {
		return false, nil
	}
	jobs, err := s.digestRepo.ListByProject(ctx, projectID, 20)
	if err != nil {
		return false, err
	}
	for _, job := range jobs {
		switch job.Status {
		case memorydomain.DigestPending, memorydomain.DigestRunning:
			return true, nil
		}
	}
	return false, nil
}

func (s *CommandService) writeDigestSummary(scoped commandScopeContext, now time.Time) (string, error) {
	agentDailyPath, err := ResolveDailyJournalWritePath(scoped.scope, scoped.guard, now, WriteScopeAgentPrivate)
	if err != nil {
		return "", err
	}
	sharedDailyPath, err := ResolveDailyJournalWritePath(scoped.scope, scoped.guard, now, WriteScopeProjectShare)
	if err != nil {
		return "", err
	}
	mainMemoryPath, err := scoped.guard.ResolveProjectSharedPath("MEMORY.md")
	if err != nil {
		return "", err
	}

	agentDaily := readOptionalText(agentDailyPath)
	sharedDaily := readOptionalText(sharedDailyPath)
	previous := readOptionalText(mainMemoryPath)

	var builder strings.Builder
	builder.WriteString(strings.TrimSpace(previous))
	if builder.Len() > 0 {
		builder.WriteString("\n\n")
	}
	builder.WriteString("## MEMORY DIGEST ")
	builder.WriteString(now.Format(time.RFC3339))
	builder.WriteString("\n")
	builder.WriteString("- route: ")
	builder.WriteString(scoped.routeKey)
	builder.WriteString("\n")
	builder.WriteString("- agent: ")
	builder.WriteString(scoped.scope.AgentID)
	builder.WriteString("\n")
	builder.WriteString("- project: ")
	builder.WriteString(scoped.projectID)
	builder.WriteString("\n")
	builder.WriteString("\n### agent-private\n")
	if strings.TrimSpace(agentDaily) == "" {
		builder.WriteString("(empty)\n")
	} else {
		builder.WriteString(truncateText(agentDaily, 1200))
		builder.WriteString("\n")
	}
	builder.WriteString("\n### project-shared\n")
	if strings.TrimSpace(sharedDaily) == "" {
		builder.WriteString("(empty)\n")
	} else {
		builder.WriteString(truncateText(sharedDaily, 1200))
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	if err := os.WriteFile(mainMemoryPath, []byte(builder.String()), 0o644); err != nil {
		return "", fmt.Errorf("write digest output: %w", err)
	}
	return mainMemoryPath, nil
}

func readOptionalText(path string) string {
	body, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(body))
}
