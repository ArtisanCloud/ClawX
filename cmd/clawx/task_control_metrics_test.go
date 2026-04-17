package main

import (
	"strings"
	"testing"
	"time"
)

func TestTaskControlMetricsPrometheusPayloadIncludesCoreSeries(t *testing.T) {
	backup := globalTaskControlMetrics
	globalTaskControlMetrics = newTaskControlMetrics()
	t.Cleanup(func() {
		globalTaskControlMetrics = backup
	})

	recordTaskControlHealthHintMetric("workspace.route_scope", "clawx-bid-all", "scope:discord:default:conv-a", "conv-a", "http://127.0.0.1:19080/healthz")
	recordTaskControlHealthHintMetric("env", "clawx-bid-all", "scope:discord:default:conv-b", "conv-b", "http://127.0.0.1:19081/healthz")
	recordTaskControlPersistMetric("bid-all", "clawx-bid-all", "scope:discord:default:conv-a", true, 3, 2, taskControlRouteHintPruneStats{
		DroppedInvalid: 1,
		DroppedExpired: 2,
		DroppedTrimmed: 3,
		TTL:            time.Hour,
		MaxEntries:     64,
	})
	recordTaskControlSelfHealMetric("bid-all", "clawx-bid-all", "applied", "health_down", 0, time.Time{}, "conv-a")
	recordTaskControlSelfHealMetric("bid-all", "clawx-bid-all", "throttled", "health_down", 2, time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC), "conv-a")

	payload := taskControlMetricsPrometheusPayload()
	expected := []string{
		"clawx_task_control_health_hint_total 2",
		"clawx_task_control_health_hint_source_total{source=\"workspace.route_scope\"} 1",
		"clawx_task_control_health_hint_source_total{source=\"env\"} 1",
		"clawx_task_control_persist_events_total 1",
		"clawx_task_control_persist_events_agent_total{agent_id=\"bid-all\"} 1",
		"clawx_task_control_persist_dropped_invalid_total 1",
		"clawx_task_control_persist_dropped_expired_total 2",
		"clawx_task_control_persist_dropped_trimmed_total 3",
		"clawx_task_control_self_heal_events_total 2",
		"clawx_task_control_self_heal_agent_total{agent_id=\"bid-all\"} 2",
		"clawx_task_control_self_heal_service_total{service=\"clawx-bid-all\"} 2",
		"clawx_task_control_self_heal_state_total{state=\"applied\"} 1",
		"clawx_task_control_self_heal_state_total{state=\"throttled\"} 1",
		"clawx_task_control_self_heal_reason_total{reason=\"health_down\"} 2",
		"clawx_task_control_last_self_heal_timestamp_seconds{agent_id=\"bid-all\",service=\"clawx-bid-all\",state=\"throttled\"}",
		"clawx_task_control_self_heal_alert_level{agent_id=\"bid-all\",service=\"clawx-bid-all\",state=\"throttled\",level=\"critical\",reason=\"health_down\"} 2",
	}
	for _, token := range expected {
		if !strings.Contains(payload, token) {
			t.Fatalf("expected token %q in payload, got:\n%s", token, payload)
		}
	}
}

func TestTaskControlMetricsHealthDetailsIncludesSelfHealSnapshot(t *testing.T) {
	backup := globalTaskControlMetrics
	globalTaskControlMetrics = newTaskControlMetrics()
	t.Cleanup(func() {
		globalTaskControlMetrics = backup
	})

	recordTaskControlSelfHealMetric("bid-all", "clawx-bid-all", "failed", "", 1, time.Time{}, "conv-b")
	recordTaskControlSelfHealMetric("bid-all", "clawx-bid-all", "throttled", "health_down", 2, time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC), "conv-b")

	details := taskControlMetricsHealthDetails()
	if value, ok := details["self_heal_events_total"].(uint64); !ok || value != 2 {
		t.Fatalf("expected self_heal_events_total=2, got: %+v", details["self_heal_events_total"])
	}
	byAgent, ok := details["self_heal_events_by_agent"].(map[string]uint64)
	if !ok {
		t.Fatalf("expected self_heal_events_by_agent map, got: %+v", details["self_heal_events_by_agent"])
	}
	if byAgent["bid-all"] != 2 {
		t.Fatalf("unexpected self_heal_events_by_agent: %+v", byAgent)
	}
	byService, ok := details["self_heal_events_by_service"].(map[string]uint64)
	if !ok {
		t.Fatalf("expected self_heal_events_by_service map, got: %+v", details["self_heal_events_by_service"])
	}
	if byService["clawx-bid-all"] != 2 {
		t.Fatalf("unexpected self_heal_events_by_service: %+v", byService)
	}
	byState, ok := details["self_heal_events_by_state"].(map[string]uint64)
	if !ok {
		t.Fatalf("expected self_heal_events_by_state map, got: %+v", details["self_heal_events_by_state"])
	}
	if byState["failed"] != 1 || byState["throttled"] != 1 {
		t.Fatalf("unexpected self_heal_events_by_state: %+v", byState)
	}
	byReason, ok := details["self_heal_events_by_reason"].(map[string]uint64)
	if !ok {
		t.Fatalf("expected self_heal_events_by_reason map, got: %+v", details["self_heal_events_by_reason"])
	}
	if byReason["unknown"] != 1 || byReason["health_down"] != 1 {
		t.Fatalf("unexpected self_heal_events_by_reason: %+v", byReason)
	}
	lastSelfHeal, ok := details["last_self_heal"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected last_self_heal snapshot, got: %+v", details["last_self_heal"])
	}
	if state, ok := lastSelfHeal["state"].(string); !ok || state != "throttled" {
		t.Fatalf("expected last self-heal state throttled, got: %+v", lastSelfHeal["state"])
	}
	alert, ok := details["self_heal_alert"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected self_heal_alert snapshot, got: %+v", details["self_heal_alert"])
	}
	if active, ok := alert["active"].(bool); !ok || !active {
		t.Fatalf("expected self_heal_alert active=true, got: %+v", alert["active"])
	}
	if level, ok := alert["level"].(string); !ok || level != "critical" {
		t.Fatalf("expected self_heal_alert level=critical, got: %+v", alert["level"])
	}
	if reason, ok := alert["reason"].(string); !ok || reason != "health_down" {
		t.Fatalf("expected self_heal_alert reason=health_down, got: %+v", alert["reason"])
	}
	if reasonLabel, ok := alert["reason_label"].(string); !ok || reasonLabel != "健康探针异常" {
		t.Fatalf("expected self_heal_alert reason_label=健康探针异常, got: %+v", alert["reason_label"])
	}
	if value, ok := alert["value"].(int); !ok || value != 2 {
		t.Fatalf("expected self_heal_alert value=2, got: %+v", alert["value"])
	}
	summary, ok := details["self_heal_summary"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected self_heal_summary snapshot, got: %+v", details["self_heal_summary"])
	}
	if status, ok := summary["status"].(string); !ok || status != "alerting" {
		t.Fatalf("expected self_heal_summary status=alerting, got: %+v", summary["status"])
	}
	if headline, ok := summary["headline"].(string); !ok || !strings.Contains(headline, "reason=health_down") {
		t.Fatalf("expected self_heal_summary headline to include reason=health_down, got: %+v", summary["headline"])
	}
	if nextAction, ok := summary["next_action"].(string); !ok || !strings.Contains(nextAction, "systemctl --user status clawx-bid-all.service --no-pager") {
		t.Fatalf("expected self_heal_summary next_action to include service status command, got: %+v", summary["next_action"])
	}
	nextSteps, ok := summary["next_steps"].([]string)
	if !ok || len(nextSteps) == 0 {
		t.Fatalf("expected self_heal_summary next_steps, got: %+v", summary["next_steps"])
	}
}

func TestEvaluateTaskControlSelfHealAlertWindowExpiry(t *testing.T) {
	now := time.Now().UTC()
	last := taskControlSelfHealMetric{
		State: "throttled",
		At:    now.Add(-30 * time.Minute).Format(time.RFC3339),
	}
	alert := evaluateTaskControlSelfHealAlert(last, now, 10*time.Minute)
	if alert.Active {
		t.Fatalf("expected expired throttled alert to be inactive, got: %+v", alert)
	}
	if alert.Level != "ok" || alert.Value != 0 {
		t.Fatalf("expected expired throttled alert level ok/0, got: %+v", alert)
	}
}
