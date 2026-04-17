package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type taskControlMetrics struct {
	mu sync.RWMutex

	healthHintTotal       uint64
	healthHintBySource    map[string]uint64
	lastHealthHint        taskControlHealthHintMetric
	persistEventsTotal    uint64
	persistByAgent        map[string]uint64
	persistDroppedInvalid uint64
	persistDroppedExpired uint64
	persistDroppedTrimmed uint64
	lastPersist           taskControlPersistMetric
	selfHealEventsTotal   uint64
	selfHealByAgent       map[string]uint64
	selfHealByService     map[string]uint64
	selfHealByState       map[string]uint64
	selfHealByReason      map[string]uint64
	lastSelfHeal          taskControlSelfHealMetric
}

type taskControlHealthHintMetric struct {
	Source         string
	Service        string
	Scope          string
	ConversationID string
	URL            string
	At             string
}

type taskControlPersistMetric struct {
	AgentID        string
	Service        string
	Scope          string
	HealthURLSet   bool
	RouteBefore    int
	RouteAfter     int
	DroppedInvalid int
	DroppedExpired int
	DroppedTrimmed int
	TTL            string
	MaxEntries     int
	At             string
}

type taskControlSelfHealMetric struct {
	AgentID        string
	Service        string
	State          string
	Reason         string
	Failures       int
	CooldownUntil  string
	ConversationID string
	At             string
}

type taskControlSelfHealAlert struct {
	Active bool
	Level  string
	Value  int
	State  string
}

var globalTaskControlMetrics = newTaskControlMetrics()

func newTaskControlMetrics() *taskControlMetrics {
	return &taskControlMetrics{
		healthHintBySource: map[string]uint64{},
		persistByAgent:     map[string]uint64{},
		selfHealByAgent:    map[string]uint64{},
		selfHealByService:  map[string]uint64{},
		selfHealByState:    map[string]uint64{},
		selfHealByReason:   map[string]uint64{},
	}
}

func recordTaskControlHealthHintMetric(source, service, scope, conversationID, url string) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "unknown"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	globalTaskControlMetrics.mu.Lock()
	defer globalTaskControlMetrics.mu.Unlock()
	globalTaskControlMetrics.healthHintTotal++
	globalTaskControlMetrics.healthHintBySource[source]++
	globalTaskControlMetrics.lastHealthHint = taskControlHealthHintMetric{
		Source:         source,
		Service:        strings.TrimSpace(service),
		Scope:          strings.TrimSpace(scope),
		ConversationID: strings.TrimSpace(conversationID),
		URL:            strings.TrimSpace(url),
		At:             now,
	}
}

func recordTaskControlPersistMetric(agentID, service, scope string, healthURLSet bool, before, after int, stats taskControlRouteHintPruneStats) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "unknown"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	globalTaskControlMetrics.mu.Lock()
	defer globalTaskControlMetrics.mu.Unlock()
	globalTaskControlMetrics.persistEventsTotal++
	globalTaskControlMetrics.persistByAgent[agentID]++
	globalTaskControlMetrics.persistDroppedInvalid += uint64(maxInt(stats.DroppedInvalid, 0))
	globalTaskControlMetrics.persistDroppedExpired += uint64(maxInt(stats.DroppedExpired, 0))
	globalTaskControlMetrics.persistDroppedTrimmed += uint64(maxInt(stats.DroppedTrimmed, 0))
	globalTaskControlMetrics.lastPersist = taskControlPersistMetric{
		AgentID:        agentID,
		Service:        strings.TrimSpace(service),
		Scope:          strings.TrimSpace(scope),
		HealthURLSet:   healthURLSet,
		RouteBefore:    maxInt(before, 0),
		RouteAfter:     maxInt(after, 0),
		DroppedInvalid: maxInt(stats.DroppedInvalid, 0),
		DroppedExpired: maxInt(stats.DroppedExpired, 0),
		DroppedTrimmed: maxInt(stats.DroppedTrimmed, 0),
		TTL:            stats.TTL.String(),
		MaxEntries:     maxInt(stats.MaxEntries, 0),
		At:             now,
	}
}

func recordTaskControlSelfHealMetric(agentID, service, state, reason string, failures int, cooldownUntil time.Time, conversationID string) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		agentID = "unknown"
	}
	service = strings.TrimSpace(strings.ToLower(strings.TrimSuffix(service, ".service")))
	if service == "" {
		service = "unknown"
	}
	state = strings.TrimSpace(strings.ToLower(state))
	if state == "" {
		state = "unknown"
	}
	reason = normalizeTaskControlSelfHealReason(reason)
	if reason == "" {
		reason = "unknown"
	}
	now := time.Now().UTC().Format(time.RFC3339)

	cooldownUntilRaw := ""
	if !cooldownUntil.IsZero() {
		cooldownUntilRaw = cooldownUntil.UTC().Format(time.RFC3339)
	}
	globalTaskControlMetrics.mu.Lock()
	defer globalTaskControlMetrics.mu.Unlock()
	globalTaskControlMetrics.selfHealEventsTotal++
	globalTaskControlMetrics.selfHealByAgent[agentID]++
	globalTaskControlMetrics.selfHealByService[service]++
	globalTaskControlMetrics.selfHealByState[state]++
	globalTaskControlMetrics.selfHealByReason[reason]++
	globalTaskControlMetrics.lastSelfHeal = taskControlSelfHealMetric{
		AgentID:        agentID,
		Service:        service,
		State:          state,
		Reason:         reason,
		Failures:       maxInt(failures, 0),
		CooldownUntil:  cooldownUntilRaw,
		ConversationID: strings.TrimSpace(conversationID),
		At:             now,
	}
}

func taskControlMetricsHealthDetails() map[string]interface{} {
	globalTaskControlMetrics.mu.RLock()
	defer globalTaskControlMetrics.mu.RUnlock()

	sourceCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.healthHintBySource {
		sourceCounts[key] = value
	}
	agentCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.persistByAgent {
		agentCounts[key] = value
	}
	selfHealAgentCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByAgent {
		selfHealAgentCounts[key] = value
	}
	selfHealServiceCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByService {
		selfHealServiceCounts[key] = value
	}
	selfHealStateCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByState {
		selfHealStateCounts[key] = value
	}
	selfHealReasonCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByReason {
		selfHealReasonCounts[key] = value
	}

	payload := map[string]interface{}{
		"health_hint_total":             globalTaskControlMetrics.healthHintTotal,
		"health_hint_by_source":         sourceCounts,
		"persist_events_total":          globalTaskControlMetrics.persistEventsTotal,
		"persist_events_by_agent":       agentCounts,
		"persist_dropped_invalid_total": globalTaskControlMetrics.persistDroppedInvalid,
		"persist_dropped_expired_total": globalTaskControlMetrics.persistDroppedExpired,
		"persist_dropped_trimmed_total": globalTaskControlMetrics.persistDroppedTrimmed,
		"self_heal_events_total":        globalTaskControlMetrics.selfHealEventsTotal,
		"self_heal_events_by_agent":     selfHealAgentCounts,
		"self_heal_events_by_service":   selfHealServiceCounts,
		"self_heal_events_by_state":     selfHealStateCounts,
		"self_heal_events_by_reason":    selfHealReasonCounts,
	}
	if strings.TrimSpace(globalTaskControlMetrics.lastHealthHint.At) != "" {
		payload["last_health_hint"] = map[string]interface{}{
			"source":          globalTaskControlMetrics.lastHealthHint.Source,
			"service":         globalTaskControlMetrics.lastHealthHint.Service,
			"scope":           globalTaskControlMetrics.lastHealthHint.Scope,
			"conversation_id": globalTaskControlMetrics.lastHealthHint.ConversationID,
			"url":             globalTaskControlMetrics.lastHealthHint.URL,
			"at":              globalTaskControlMetrics.lastHealthHint.At,
		}
	}
	if strings.TrimSpace(globalTaskControlMetrics.lastPersist.At) != "" {
		payload["last_persist"] = map[string]interface{}{
			"agent_id":        globalTaskControlMetrics.lastPersist.AgentID,
			"service":         globalTaskControlMetrics.lastPersist.Service,
			"scope":           globalTaskControlMetrics.lastPersist.Scope,
			"health_url_set":  globalTaskControlMetrics.lastPersist.HealthURLSet,
			"route_before":    globalTaskControlMetrics.lastPersist.RouteBefore,
			"route_after":     globalTaskControlMetrics.lastPersist.RouteAfter,
			"dropped_invalid": globalTaskControlMetrics.lastPersist.DroppedInvalid,
			"dropped_expired": globalTaskControlMetrics.lastPersist.DroppedExpired,
			"dropped_trimmed": globalTaskControlMetrics.lastPersist.DroppedTrimmed,
			"ttl":             globalTaskControlMetrics.lastPersist.TTL,
			"max_entries":     globalTaskControlMetrics.lastPersist.MaxEntries,
			"at":              globalTaskControlMetrics.lastPersist.At,
		}
	}
	if strings.TrimSpace(globalTaskControlMetrics.lastSelfHeal.At) != "" {
		payload["last_self_heal"] = map[string]interface{}{
			"agent_id":        globalTaskControlMetrics.lastSelfHeal.AgentID,
			"service":         globalTaskControlMetrics.lastSelfHeal.Service,
			"state":           globalTaskControlMetrics.lastSelfHeal.State,
			"reason":          globalTaskControlMetrics.lastSelfHeal.Reason,
			"failures":        globalTaskControlMetrics.lastSelfHeal.Failures,
			"cooldown_until":  globalTaskControlMetrics.lastSelfHeal.CooldownUntil,
			"conversation_id": globalTaskControlMetrics.lastSelfHeal.ConversationID,
			"at":              globalTaskControlMetrics.lastSelfHeal.At,
		}
		alert := evaluateTaskControlSelfHealAlert(globalTaskControlMetrics.lastSelfHeal, time.Now().UTC(), resolveTaskControlSelfHealAlertWindow())
		reasonLabel := strings.TrimSpace(mapTaskControlAutoRecoveryReasonLabel(globalTaskControlMetrics.lastSelfHeal.Reason))
		payload["self_heal_alert"] = map[string]interface{}{
			"active":       alert.Active,
			"level":        alert.Level,
			"value":        alert.Value,
			"state":        alert.State,
			"reason":       fallbackValue(globalTaskControlMetrics.lastSelfHeal.Reason, "unknown"),
			"reason_label": fallbackValue(reasonLabel, "未知原因"),
			"window":       resolveTaskControlSelfHealAlertWindow().String(),
		}
		payload["self_heal_summary"] = buildTaskControlSelfHealReadableSummary(globalTaskControlMetrics.lastSelfHeal, alert, resolveTaskControlSelfHealAlertWindow())
	}
	return payload
}

func taskControlMetricsPrometheusPayload() string {
	globalTaskControlMetrics.mu.RLock()
	healthHintTotal := globalTaskControlMetrics.healthHintTotal
	persistEventsTotal := globalTaskControlMetrics.persistEventsTotal
	droppedInvalidTotal := globalTaskControlMetrics.persistDroppedInvalid
	droppedExpiredTotal := globalTaskControlMetrics.persistDroppedExpired
	droppedTrimmedTotal := globalTaskControlMetrics.persistDroppedTrimmed
	selfHealEventsTotal := globalTaskControlMetrics.selfHealEventsTotal
	lastHint := globalTaskControlMetrics.lastHealthHint
	lastPersist := globalTaskControlMetrics.lastPersist
	lastSelfHeal := globalTaskControlMetrics.lastSelfHeal
	sourceCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.healthHintBySource {
		sourceCounts[key] = value
	}
	agentCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.persistByAgent {
		agentCounts[key] = value
	}
	selfHealAgentCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByAgent {
		selfHealAgentCounts[key] = value
	}
	selfHealServiceCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByService {
		selfHealServiceCounts[key] = value
	}
	selfHealStateCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByState {
		selfHealStateCounts[key] = value
	}
	selfHealReasonCounts := map[string]uint64{}
	for key, value := range globalTaskControlMetrics.selfHealByReason {
		selfHealReasonCounts[key] = value
	}
	globalTaskControlMetrics.mu.RUnlock()

	var builder strings.Builder
	builder.WriteString("# HELP clawx_task_control_health_hint_total Total task control health hint selections.\n")
	builder.WriteString("# TYPE clawx_task_control_health_hint_total counter\n")
	builder.WriteString("clawx_task_control_health_hint_total ")
	builder.WriteString(strconv.FormatUint(healthHintTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_health_hint_source_total Task control health hint selections by source.\n")
	builder.WriteString("# TYPE clawx_task_control_health_hint_source_total counter\n")
	for _, source := range sortedStringKeysFromUint64Map(sourceCounts) {
		builder.WriteString("clawx_task_control_health_hint_source_total{source=\"")
		builder.WriteString(prometheusLabelEscape(source))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(sourceCounts[source], 10))
		builder.WriteByte('\n')
	}

	builder.WriteString("# HELP clawx_task_control_persist_events_total Total task control hint persist events.\n")
	builder.WriteString("# TYPE clawx_task_control_persist_events_total counter\n")
	builder.WriteString("clawx_task_control_persist_events_total ")
	builder.WriteString(strconv.FormatUint(persistEventsTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_persist_events_agent_total Task control hint persist events by agent.\n")
	builder.WriteString("# TYPE clawx_task_control_persist_events_agent_total counter\n")
	for _, agentID := range sortedStringKeysFromUint64Map(agentCounts) {
		builder.WriteString("clawx_task_control_persist_events_agent_total{agent_id=\"")
		builder.WriteString(prometheusLabelEscape(agentID))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(agentCounts[agentID], 10))
		builder.WriteByte('\n')
	}

	builder.WriteString("# HELP clawx_task_control_persist_dropped_invalid_total Total invalid route hints dropped during persist.\n")
	builder.WriteString("# TYPE clawx_task_control_persist_dropped_invalid_total counter\n")
	builder.WriteString("clawx_task_control_persist_dropped_invalid_total ")
	builder.WriteString(strconv.FormatUint(droppedInvalidTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_persist_dropped_expired_total Total expired route hints dropped during persist.\n")
	builder.WriteString("# TYPE clawx_task_control_persist_dropped_expired_total counter\n")
	builder.WriteString("clawx_task_control_persist_dropped_expired_total ")
	builder.WriteString(strconv.FormatUint(droppedExpiredTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_persist_dropped_trimmed_total Total route hints trimmed by max entries.\n")
	builder.WriteString("# TYPE clawx_task_control_persist_dropped_trimmed_total counter\n")
	builder.WriteString("clawx_task_control_persist_dropped_trimmed_total ")
	builder.WriteString(strconv.FormatUint(droppedTrimmedTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_self_heal_events_total Total runtime.task.control self-heal events.\n")
	builder.WriteString("# TYPE clawx_task_control_self_heal_events_total counter\n")
	builder.WriteString("clawx_task_control_self_heal_events_total ")
	builder.WriteString(strconv.FormatUint(selfHealEventsTotal, 10))
	builder.WriteByte('\n')

	builder.WriteString("# HELP clawx_task_control_self_heal_agent_total Runtime.task.control self-heal events by agent.\n")
	builder.WriteString("# TYPE clawx_task_control_self_heal_agent_total counter\n")
	for _, agentID := range sortedStringKeysFromUint64Map(selfHealAgentCounts) {
		builder.WriteString("clawx_task_control_self_heal_agent_total{agent_id=\"")
		builder.WriteString(prometheusLabelEscape(agentID))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(selfHealAgentCounts[agentID], 10))
		builder.WriteByte('\n')
	}

	builder.WriteString("# HELP clawx_task_control_self_heal_service_total Runtime.task.control self-heal events by service.\n")
	builder.WriteString("# TYPE clawx_task_control_self_heal_service_total counter\n")
	for _, service := range sortedStringKeysFromUint64Map(selfHealServiceCounts) {
		builder.WriteString("clawx_task_control_self_heal_service_total{service=\"")
		builder.WriteString(prometheusLabelEscape(service))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(selfHealServiceCounts[service], 10))
		builder.WriteByte('\n')
	}

	builder.WriteString("# HELP clawx_task_control_self_heal_state_total Runtime.task.control self-heal events by state.\n")
	builder.WriteString("# TYPE clawx_task_control_self_heal_state_total counter\n")
	for _, state := range sortedStringKeysFromUint64Map(selfHealStateCounts) {
		builder.WriteString("clawx_task_control_self_heal_state_total{state=\"")
		builder.WriteString(prometheusLabelEscape(state))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(selfHealStateCounts[state], 10))
		builder.WriteByte('\n')
	}

	builder.WriteString("# HELP clawx_task_control_self_heal_reason_total Runtime.task.control self-heal events by reason.\n")
	builder.WriteString("# TYPE clawx_task_control_self_heal_reason_total counter\n")
	for _, reason := range sortedStringKeysFromUint64Map(selfHealReasonCounts) {
		builder.WriteString("clawx_task_control_self_heal_reason_total{reason=\"")
		builder.WriteString(prometheusLabelEscape(reason))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.FormatUint(selfHealReasonCounts[reason], 10))
		builder.WriteByte('\n')
	}

	if ts, ok := parseMetricRFC3339Unix(lastHint.At); ok {
		builder.WriteString("# HELP clawx_task_control_last_health_hint_timestamp_seconds Unix timestamp of last health hint selection.\n")
		builder.WriteString("# TYPE clawx_task_control_last_health_hint_timestamp_seconds gauge\n")
		builder.WriteString("clawx_task_control_last_health_hint_timestamp_seconds{source=\"")
		builder.WriteString(prometheusLabelEscape(lastHint.Source))
		builder.WriteString("\",service=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastHint.Service, "unknown")))
		builder.WriteString("\"} ")
		builder.WriteString(fmt.Sprintf("%.0f", ts))
		builder.WriteByte('\n')
	}

	if ts, ok := parseMetricRFC3339Unix(lastPersist.At); ok {
		builder.WriteString("# HELP clawx_task_control_last_persist_timestamp_seconds Unix timestamp of last hint persist event.\n")
		builder.WriteString("# TYPE clawx_task_control_last_persist_timestamp_seconds gauge\n")
		builder.WriteString("clawx_task_control_last_persist_timestamp_seconds{agent_id=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastPersist.AgentID, "unknown")))
		builder.WriteString("\",service=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastPersist.Service, "unknown")))
		builder.WriteString("\"} ")
		builder.WriteString(fmt.Sprintf("%.0f", ts))
		builder.WriteByte('\n')
	}

	if ts, ok := parseMetricRFC3339Unix(lastSelfHeal.At); ok {
		builder.WriteString("# HELP clawx_task_control_last_self_heal_timestamp_seconds Unix timestamp of last self-heal event.\n")
		builder.WriteString("# TYPE clawx_task_control_last_self_heal_timestamp_seconds gauge\n")
		builder.WriteString("clawx_task_control_last_self_heal_timestamp_seconds{agent_id=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.AgentID, "unknown")))
		builder.WriteString("\",service=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.Service, "unknown")))
		builder.WriteString("\",state=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.State, "unknown")))
		builder.WriteString("\"} ")
		builder.WriteString(fmt.Sprintf("%.0f", ts))
		builder.WriteByte('\n')

		alert := evaluateTaskControlSelfHealAlert(lastSelfHeal, time.Now().UTC(), resolveTaskControlSelfHealAlertWindow())
		builder.WriteString("# HELP clawx_task_control_self_heal_alert_level Current self-heal alert level (0=ok,1=warning,2=critical).\n")
		builder.WriteString("# TYPE clawx_task_control_self_heal_alert_level gauge\n")
		builder.WriteString("clawx_task_control_self_heal_alert_level{agent_id=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.AgentID, "unknown")))
		builder.WriteString("\",service=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.Service, "unknown")))
		builder.WriteString("\",state=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(alert.State, "unknown")))
		builder.WriteString("\",level=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(alert.Level, "ok")))
		builder.WriteString("\",reason=\"")
		builder.WriteString(prometheusLabelEscape(fallbackValue(lastSelfHeal.Reason, "unknown")))
		builder.WriteString("\"} ")
		builder.WriteString(strconv.Itoa(alert.Value))
		builder.WriteByte('\n')
	}

	return builder.String()
}

func evaluateTaskControlSelfHealAlert(last taskControlSelfHealMetric, now time.Time, window time.Duration) taskControlSelfHealAlert {
	alert := taskControlSelfHealAlert{
		Active: false,
		Level:  "ok",
		Value:  0,
		State:  strings.TrimSpace(strings.ToLower(last.State)),
	}
	if alert.State == "" {
		alert.State = "unknown"
	}
	if window <= 0 {
		return alert
	}
	lastAt, ok := parseMetricRFC3339Time(last.At)
	if !ok || now.IsZero() {
		return alert
	}
	if now.Sub(lastAt) > window {
		return alert
	}
	switch alert.State {
	case "throttled":
		alert.Active = true
		alert.Level = "critical"
		alert.Value = 2
	case "failed":
		alert.Active = true
		alert.Level = "warning"
		alert.Value = 1
	}
	return alert
}

func parseMetricRFC3339Unix(raw string) (float64, bool) {
	if parsed, ok := parseMetricRFC3339Time(raw); ok {
		return float64(parsed.UTC().Unix()), true
	}
	return 0, false
}

func parseMetricRFC3339Time(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

func resolveTaskControlSelfHealAlertWindow() time.Duration {
	raw := strings.TrimSpace(os.Getenv("CLAWX_TASK_CONTROL_SELF_HEAL_ALERT_WINDOW"))
	if raw == "" {
		return 10 * time.Minute
	}
	if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
		return parsed
	}
	if seconds, err := strconv.Atoi(raw); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return 10 * time.Minute
}

func sortedStringKeysFromUint64Map(items map[string]uint64) []string {
	if len(items) == 0 {
		return nil
	}
	keys := make([]string, 0, len(items))
	for key := range items {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func prometheusLabelEscape(raw string) string {
	raw = strings.ReplaceAll(raw, "\\", "\\\\")
	raw = strings.ReplaceAll(raw, "\"", "\\\"")
	raw = strings.ReplaceAll(raw, "\n", "\\n")
	return raw
}

func maxInt(value int, fallback int) int {
	if value < 0 {
		return fallback
	}
	return value
}

func normalizeTaskControlSelfHealReason(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "health_down":
		return "health_down"
	case "service_not_active":
		return "service_not_active"
	case "release_changed":
		return "release_changed"
	default:
		return ""
	}
}

func buildTaskControlSelfHealReadableSummary(last taskControlSelfHealMetric, alert taskControlSelfHealAlert, window time.Duration) map[string]interface{} {
	service := strings.TrimSpace(strings.ToLower(strings.TrimSuffix(last.Service, ".service")))
	if service == "" {
		service = "unknown"
	}
	state := strings.TrimSpace(strings.ToLower(last.State))
	if state == "" {
		state = "unknown"
	}
	level := normalizeTaskControlSelfHealAlertLevel(alert.Level)
	if level == "" {
		level = "ok"
	}
	reason := normalizeTaskControlSelfHealReason(last.Reason)
	if reason == "" {
		reason = "unknown"
	}
	reasonLabel := strings.TrimSpace(mapTaskControlAutoRecoveryReasonLabel(reason))
	if reasonLabel == "" {
		reasonLabel = "未知原因"
	}
	status := "stable"
	if alert.Active {
		status = "alerting"
	} else if level != "ok" {
		status = "observing"
	}
	headline := fmt.Sprintf("自治自愈状态：status=%s level=%s state=%s reason=%s（%s）", status, level, state, reason, reasonLabel)
	playbook := buildLeadWorkerSelfHealAlertPlaybook(service, level, state, reason, "")
	nextAction := ""
	if len(playbook) > 0 {
		nextAction = strings.TrimSpace(playbook[0])
	}
	return map[string]interface{}{
		"status":       status,
		"headline":     headline,
		"service":      service,
		"level":        level,
		"state":        state,
		"reason":       reason,
		"reason_label": reasonLabel,
		"window":       window.String(),
		"next_action":  nextAction,
		"next_steps":   playbook,
	}
}
