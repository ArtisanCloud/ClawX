package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTaskControlPrometheusHandlerRespondsWithPlainText(t *testing.T) {
	handler := NewTaskControlPrometheusHandler(func(_ context.Context) string {
		return "# HELP clawx_task_control_health_hint_total total health hint picks\n" +
			"# TYPE clawx_task_control_health_hint_total counter\n" +
			"clawx_task_control_health_hint_total 3\n"
	})
	req := httptest.NewRequest(http.MethodGet, "/metrics/task-control/prometheus", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); !strings.Contains(got, "text/plain") {
		t.Fatalf("expected text/plain response, got %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "clawx_task_control_health_hint_total 3") {
		t.Fatalf("unexpected prometheus payload: %s", body)
	}
}
