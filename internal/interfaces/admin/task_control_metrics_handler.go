package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type TaskControlMetricsProvider func(ctx context.Context) map[string]interface{}

type TaskControlMetricsHandler struct {
	provider TaskControlMetricsProvider
}

func NewTaskControlMetricsHandler(provider TaskControlMetricsProvider) *TaskControlMetricsHandler {
	return &TaskControlMetricsHandler{provider: provider}
}

func (h *TaskControlMetricsHandler) Register(mux *http.ServeMux, path string) {
	if mux == nil {
		return
	}
	if path == "" {
		path = "/metrics/task-control"
	}
	mux.Handle(path, h)
}

func (h *TaskControlMetricsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	metrics := map[string]interface{}{}
	if h != nil && h.provider != nil {
		if payload := h.provider(r.Context()); len(payload) > 0 {
			metrics = payload
		}
	}

	body := map[string]interface{}{
		"status":     "ok",
		"checked_at": time.Now().UTC().Format(time.RFC3339),
		"metrics":    metrics,
	}

	encoded, err := json.Marshal(body)
	if err != nil {
		http.Error(w, `{"status":"error","failure":"marshal failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
