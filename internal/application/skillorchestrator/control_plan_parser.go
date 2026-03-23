package skillorchestrator

import (
	"encoding/json"
	"regexp"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

var controlPlanJSONFencePattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func ParseControlPlanFromText(text string) (skilldomain.ControlPlan, bool, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return skilldomain.ControlPlan{}, false, nil
	}

	candidate, found, parseErr := extractControlPlanJSONCandidate(trimmed)
	if parseErr != nil {
		return skilldomain.ControlPlan{}, true, parseErr
	}
	if !found {
		return skilldomain.ControlPlan{}, false, nil
	}

	var plan skilldomain.ControlPlan
	if err := json.Unmarshal([]byte(candidate), &plan); err != nil {
		return skilldomain.ControlPlan{}, true, err
	}
	if err := plan.Validate(); err != nil {
		return skilldomain.ControlPlan{}, true, err
	}
	return plan.Normalize(), true, nil
}

func extractControlPlanJSONCandidate(text string) (string, bool, error) {
	if strings.HasPrefix(text, "{") && strings.HasSuffix(text, "}") {
		return text, true, nil
	}
	matches := controlPlanJSONFencePattern.FindStringSubmatch(text)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1]), true, nil
	}
	return "", false, nil
}
