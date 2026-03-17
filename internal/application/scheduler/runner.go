package scheduler

import (
	"context"
	"time"
)

type LoopRunner struct {
	service  *Service
	interval time.Duration
}

func NewLoopRunner(service *Service, interval time.Duration) *LoopRunner {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &LoopRunner{service: service, interval: interval}
}

func (r *LoopRunner) Run(ctx context.Context) error {
	if r == nil || r.service == nil {
		return nil
	}
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := r.service.Tick(ctx); err != nil {
				return err
			}
		}
	}
}
