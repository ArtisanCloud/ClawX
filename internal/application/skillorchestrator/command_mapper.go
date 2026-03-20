package skillorchestrator

import (
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

func MapCommandToAction(raw string) (skilldomain.SkillAction, bool, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return skilldomain.SkillAction{}, false, nil
	}
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return skilldomain.SkillAction{}, false, nil
	}
	head := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
	if head != "skill" {
		return skilldomain.SkillAction{}, false, nil
	}
	actionName := strings.ToLower(strings.TrimSpace(fields[1]))
	switch actionName {
	case "install":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 skill id。示例：`/skill install bid.collect`")
		}
		return skilldomain.SkillAction{
			Intent:               "install_skill",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            parseCommandFlags(fields[3:]),
			Confidence:           1,
			RiskLevel:            skilldomain.RiskLow,
			RequiresConfirmation: false,
			Source:               "command",
		}.Normalize(), true, nil
	case "bind":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 skill id。示例：`/skill bind bid.collect --agent bid-all`")
		}
		return skilldomain.SkillAction{
			Intent:               "bind_skill",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            parseCommandFlags(fields[3:]),
			Confidence:           1,
			RiskLevel:            skilldomain.RiskLow,
			RequiresConfirmation: false,
			Source:               "command",
		}.Normalize(), true, nil
	case "disable":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 skill id。示例：`/skill disable bid.collect`")
		}
		return skilldomain.SkillAction{
			Intent:               "disable_skill",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            parseCommandFlags(fields[3:]),
			Confidence:           1,
			RiskLevel:            skilldomain.RiskMedium,
			RequiresConfirmation: false,
			Source:               "command",
		}.Normalize(), true, nil
	case "run":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 skill id。示例：`/skill run bid.collect --risk high`")
		}
		arguments := parseCommandFlags(fields[3:])
		risk := resolveRiskLevel(arguments)
		return skilldomain.SkillAction{
			Intent:               "run_skill",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            arguments,
			Confidence:           1,
			RiskLevel:            risk,
			RequiresConfirmation: risk == skilldomain.RiskHigh,
			Source:               "command",
		}.Normalize(), true, nil
	case "replay":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 trace id。示例：`/skill replay trace-xxx`")
		}
		return skilldomain.SkillAction{
			Intent:               "replay_skill_audit",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            map[string]any{},
			Confidence:           1,
			RiskLevel:            skilldomain.RiskLow,
			RequiresConfirmation: false,
			Source:               "command",
		}.Normalize(), true, nil
	case "upgrade":
		if len(fields) < 3 {
			return skilldomain.SkillAction{}, true, fmt.Errorf("缺少 skill id。示例：`/skill upgrade bid.collect --version v2.0.0`")
		}
		return skilldomain.SkillAction{
			Intent:               "upgrade_skill",
			SkillID:              strings.TrimSpace(fields[2]),
			Arguments:            parseCommandFlags(fields[3:]),
			Confidence:           1,
			RiskLevel:            skilldomain.RiskMedium,
			RequiresConfirmation: false,
			Source:               "command",
		}.Normalize(), true, nil
	default:
		return skilldomain.SkillAction{}, true, fmt.Errorf("不支持的 /skill 子命令 %q", actionName)
	}
}

func resolveRiskLevel(arguments map[string]any) skilldomain.RiskLevel {
	if arguments == nil {
		return skilldomain.RiskLow
	}
	raw, ok := arguments["risk"]
	if !ok {
		return skilldomain.RiskLow
	}
	text, _ := raw.(string)
	switch strings.ToLower(strings.TrimSpace(text)) {
	case string(skilldomain.RiskHigh):
		return skilldomain.RiskHigh
	case string(skilldomain.RiskMedium):
		return skilldomain.RiskMedium
	default:
		return skilldomain.RiskLow
	}
}

func parseCommandFlags(tokens []string) map[string]any {
	if len(tokens) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any)
	for i := 0; i < len(tokens); i++ {
		token := strings.TrimSpace(tokens[i])
		if token == "" {
			continue
		}
		if strings.HasPrefix(token, "--") {
			key := strings.TrimPrefix(token, "--")
			value := "true"
			if i+1 < len(tokens) && !strings.HasPrefix(strings.TrimSpace(tokens[i+1]), "--") {
				value = strings.TrimSpace(tokens[i+1])
				i++
			}
			if key != "" {
				out[key] = value
			}
		}
	}
	return out
}
