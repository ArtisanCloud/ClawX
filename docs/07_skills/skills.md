# Skills 规范（Claude Compatibility First）

## 定位
SynapseX 的 Skill 能力采用 Claude Code 生态规范，目标是直接复用已有 Skill 资产，而不是定义一套新格式。

## 兼容原则
- Skill 根目录必须包含 `SKILL.md`。
- `SKILL.md` frontmatter 至少包含 `name` 与 `description`。
- Skill 的 Markdown body 作为执行上下文指令，可按需读取 `references/`、`scripts/`、`assets/`。
- SynapseX 不改写第三方 Skill 本体，只做加载、路由、权限控制与执行注入。

## 路由原则
- 采用“规则优先 + LLM 兜底”：
1. 控制命令优先。
2. 显式 skill 命令（如 `/skill <name>`）次之。
3. 文本规则匹配再次之。
4. 最后才走 LLM 意图兜底。

## 权限原则
- 支持 skill 总开关。
- 支持按 skill 名禁用。
- 支持按 user/channel 白名单授权。

## 相关专题
- `docs/features/skill_registry_intent_router/overview.md`
- `docs/features/skill_registry_intent_router/architecture.md`
- `docs/features/skill_registry_intent_router/implementation.md`
- `docs/features/skill_registry_intent_router/compatibility_claude_skills.md`
- `docs/features/skill_registry_intent_router/testing.md`
