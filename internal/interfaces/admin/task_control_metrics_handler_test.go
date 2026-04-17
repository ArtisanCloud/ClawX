package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTaskControlMetricsHandlerRespondsWithMetrics(t *testing.T) {
	handler := NewTaskControlMetricsHandler(func(_ context.Context) map[string]interface{} {
		return map[string]interface{}{
			"health_hint_total": float64(5),
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics/task-control", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected Cache-Control no-store, got %q", got)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("expected status ok, got %+v", payload["status"])
	}
	metrics, ok := payload["metrics"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metrics object, got: %+v", payload["metrics"])
	}
	if value, ok := metrics["health_hint_total"].(float64); !ok || value != 5 {
		t.Fatalf("expected health_hint_total=5, got %+v", metrics["health_hint_total"])
	}
}
