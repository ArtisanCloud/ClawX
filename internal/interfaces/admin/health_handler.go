package admin

import (
	"context"
	"encoding/json"
	"net/http"

	"clawx/internal/infrastructure/health"
)

type HealthHandler struct {
	probe           *health.Probe
	detailsProvider func(ctx context.Context) map[string]interface{}
}

func NewHealthHandler(probe *health.Probe) *HealthHandler {
	return &HealthHandler{probe: probe}
}

func (h *HealthHandler) SetDetailsProvider(provider func(ctx context.Context) map[string]interface{}) {
	if h == nil {
		return
	}
	h.detailsProvider = provider
}

func (h *HealthHandler) Register(mux *http.ServeMux, path string) {
	if mux == nil {
		return
	}
	if path == "" {
		path = "/healthz"
	}
	mux.Handle(path, h)
}

func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	report := h.probe.Check(r.Context())
	if h.detailsProvider != nil {
		if details := h.detailsProvider(r.Context()); len(details) > 0 {
			report.Details = details
		}
	}
	statusCode := mapStatus(report.Status)

	payload, err := json.Marshal(report)
	if err != nil {
		http.Error(w, `{"status":"unhealthy","failure":"marshal failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write(payload)
}

func (h *HealthHandler) Check(ctx context.Context) health.Report {
	return h.probe.Check(ctx)
}

func mapStatus(status health.Status) int {
	switch status {
	case health.StatusHealthy:
		return http.StatusOK
	case health.StatusDegraded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
