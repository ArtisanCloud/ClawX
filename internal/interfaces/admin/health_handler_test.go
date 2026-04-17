package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"clawx/internal/infrastructure/health"
)

func TestHealthHandlerIncludesDetailsFromProvider(t *testing.T) {
	handler := NewHealthHandler(health.NewProbe(func(context.Context) error {
		return nil
	}))
	handler.SetDetailsProvider(func(context.Context) map[string]interface{} {
		return map[string]interface{}{
			"task_control": map[string]interface{}{
				"health_hint_total": float64(3),
			},
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	details, ok := payload["details"].(map[string]interface{})
	if !ok || len(details) == 0 {
		t.Fatalf("expected details in health payload, got: %+v", payload)
	}
	taskControl, ok := details["task_control"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected task_control details, got: %+v", details)
	}
	if value, ok := taskControl["health_hint_total"].(float64); !ok || value != 3 {
		t.Fatalf("expected health_hint_total=3, got: %+v", taskControl["health_hint_total"])
	}
}
