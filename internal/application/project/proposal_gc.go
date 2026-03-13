package project

import (
	"context"
	"time"
)

type ProposalGC struct {
	service  *Service
	interval time.Duration
}

func NewProposalGC(service *Service, interval time.Duration) *ProposalGC {
	if interval <= 0 {
		interval = time.Minute
	}
	return &ProposalGC{
		service:  service,
		interval: interval,
	}
}

func (g *ProposalGC) RunOnce(ctx context.Context) (int, error) {
	if g == nil || g.service == nil {
		return 0, nil
	}
	return g.service.ExpireProposals(ctx)
}

func (g *ProposalGC) Run(ctx context.Context) error {
	if g == nil || g.service == nil {
		return nil
	}

	ticker := time.NewTicker(g.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := g.RunOnce(ctx); err != nil {
				return err
			}
		}
	}
}
