package backend

import "context"

func (r *DirectRunner) Cancel(_ context.Context, sessionID string) error {
	r.mu.Lock()
	cancel, exists := r.cancels[sessionID]
	r.mu.Unlock()
	if !exists {
		return nil
	}
	cancel()
	return nil
}

