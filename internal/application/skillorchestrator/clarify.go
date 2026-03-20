package skillorchestrator

import "strings"

func BuildLowConfidenceSuggestion(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		text = "（空输入）"
	}
	return "我理解你可能在操作技能，但当前置信度不足，先不执行。\n" +
		"可改用更明确表达，例如：\n" +
		"- /skill install <skill_id>\n" +
		"- /skill bind <skill_id> --agent <agent_id>\n" +
		"- 直接说“安装技能 <skill_id> 到 <agent_id>”\n" +
		"原始输入：" + text
}
