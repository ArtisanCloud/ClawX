package admin

import (
	"context"
	"net/http"
)

type TaskControlPrometheusProvider func(ctx context.Context) string

type TaskControlPrometheusHandler struct {
	provider TaskControlPrometheusProvider
}

func NewTaskControlPrometheusHandler(provider TaskControlPrometheusProvider) *TaskControlPrometheusHandler {
	return &TaskControlPrometheusHandler{provider: provider}
}

func (h *TaskControlPrometheusHandler) Register(mux *http.ServeMux, path string) {
	if mux == nil {
		return
	}
	if path == "" {
		path = "/metrics/task-control/prometheus"
	}
	mux.Handle(path, h)
}

func (h *TaskControlPrometheusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body := ""
	if h != nil && h.provider != nil {
		body = h.provider(r.Context())
	}
	if body == "" {
		body = "# task control metrics unavailable\n"
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(body))
}
