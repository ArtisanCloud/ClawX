package skillorchestrator

import "strings"

func HelpText() string {
	return strings.TrimSpace(`
Skill 指令：
- /skill install <skill_id> [--source builtin|clawhub|local|git] [--version vX.Y.Z] [--package <file:///path/to/pkg.tgz>] [--agent <agent_id>]
- /skill disable <skill_id>
- /skill bind <skill_id> [--version vX.Y.Z] [--scope global|project|agent-local] [--project <project_id>] [--agent <agent_id>]
- /skill run <skill_id> [--version vX.Y.Z] [--risk low|medium|high] [--project <project_id>] [--agent <agent_id>] [--confirm <confirmation_id>] [--reject <confirmation_id>]
- /skill replay <trace_id>

说明：
- 自然语言默认会尝试路由到 SkillAction。
- 低置信度场景只会给建议，不会直接执行。
- 技能安装受策略管控（来源白名单、版本策略、禁用规则）。`)
}
