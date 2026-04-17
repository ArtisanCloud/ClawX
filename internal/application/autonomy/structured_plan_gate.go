package autonomy

import (
	"fmt"
	"strings"
)

func EnforceStructuredPlanGate(text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	payloads := extractStructuredPayloads(trimmed)
	if len(payloads) == 0 {
		return nil
	}

	for _, payload := range payloads {
		typ := strings.TrimSpace(strings.ToLower(toString(payload["type"])))
		switch typ {
		case "", "json_schema":
			continue
		case "control_plan":
			return fmt.Errorf("control_plan is deprecated; use action_plan")
		case "requirement_sync":
			return fmt.Errorf("requirement_sync is deprecated; use action_plan")
		case "runtime_exec_plan":
			return fmt.Errorf("runtime_exec_plan is deprecated; use action_plan")
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
				"runtime.exec":           {},
				"agent.use":              {},
				"requirement.sync":       {},
				"config.exec":            {},
				"runtime.bootstrap":      {},
				"runtime.service":        {},
				"runtime.release":        {},
				"runtime.supervisor":     {},
				"runtime.release.status": {},
				"runtime.task.status":    {},
				"runtime.task.delegate":  {},
				"runtime.task.delegates": {},
				"runtime.task.retry":     {},
				"runtime.task.cancel":    {},
				"runtime.task.control":   {},
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
		}
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
