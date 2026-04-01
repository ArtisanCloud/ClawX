package autonomy

import (
	"encoding/json"
	"fmt"
	"strings"
)

func EnforceStructuredPlanGate(text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "\"type\":\"control_plan\"") || strings.Contains(lower, "\"type\": \"control_plan\"") {
		return fmt.Errorf("control_plan is deprecated; use action_plan")
	}
	if strings.Contains(lower, "\"type\":\"requirement_sync\"") || strings.Contains(lower, "\"type\": \"requirement_sync\"") {
		return fmt.Errorf("requirement_sync is deprecated; use action_plan")
	}
	if strings.Contains(lower, "\"type\":\"runtime_exec_plan\"") || strings.Contains(lower, "\"type\": \"runtime_exec_plan\"") {
		return fmt.Errorf("runtime_exec_plan is deprecated; use action_plan")
	}
	if !strings.Contains(lower, "\"type\":\"action_plan\"") && !strings.Contains(lower, "\"type\": \"action_plan\"") &&
		!strings.Contains(lower, "\"type\":\"execution_blocker\"") && !strings.Contains(lower, "\"type\": \"execution_blocker\"") {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return fmt.Errorf("structured payload parse failed: %w", err)
	}
	typ := strings.TrimSpace(strings.ToLower(toString(payload["type"])))
	switch typ {
	case "action_plan":
		mode := strings.TrimSpace(strings.ToLower(toString(payload["mode"])))
		if mode != "execute" && mode != "suggest" {
			return fmt.Errorf("action_plan mode invalid")
		}
		actions, _ := payload["actions"].([]any)
		if len(actions) == 0 {
			return fmt.Errorf("action_plan actions empty")
		}
		allowed := map[string]struct{}{
			"runtime.exec":     {},
			"agent.use":        {},
			"requirement.sync": {},
			"config.exec":      {},
		}
		for _, item := range actions {
			entry, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("action_plan action invalid")
			}
			kind := strings.TrimSpace(strings.ToLower(toString(entry["kind"])))
			if _, exists := allowed[kind]; !exists {
				return fmt.Errorf("action_plan action kind not allowed")
			}
		}
	case "execution_blocker":
		needInput, _ := payload["need_user_input"].(bool)
		if needInput {
			attempted, _ := payload["attempted"].([]any)
			evidence, _ := payload["evidence"].([]any)
			execIDs, _ := payload["evidence_exec_ids"].([]any)
			if len(attempted) == 0 || len(evidence) == 0 || len(execIDs) == 0 {
				return fmt.Errorf("execution_blocker missing attempted/evidence/evidence_exec_ids")
			}
		}
	default:
		return fmt.Errorf("structured payload type not allowed")
	}
	return nil
}

func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
