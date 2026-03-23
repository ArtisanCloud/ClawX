# Skills 规范（Claude Compatibility First）

## 定位
ClawX 的 Skill 能力采用 Claude Code 生态规范，目标是直接复用已有 Skill 资产，而不是定义一套新格式。

## 兼容原则
- Skill 根目录必须包含 `SKILL.md`。
- `SKILL.md` frontmatter 至少包含 `name` 与 `description`。
- Skill 的 Markdown body 作为执行上下文指令，可按需读取 `references/`、`scripts/`、`assets/`。
- ClawX 不改写第三方 Skill 本体，只做加载、路由、权限控制与执行注入。

## 路由原则
- 采用“命令直达 + 自然语言 LLM 规划 + 结构化执行”：
1. `/...` 显式命令走命令处理器（直接执行）。
2. 非 `/` 自然语言走 LLM 路由与规划。
3. 自动执行仅接受结构化 action/control plan。
4. 禁止从普通文本里正则提取 `/command` 自动执行。

## Prompt Caching（OpenAI）接入规范
- 目标：降低自然语言路由与规划阶段的重复 token 成本，并提升首段规划响应速度。
- API 要求：接入方必须遵循 OpenAI 官方文档中的 `Requirements` 与字段约束。
- 请求字段：
1. `prompt_cache_key`：用于稳定同前缀请求的缓存路由；建议按“固定系统前缀 + 路由阶段”生成稳定 key。
2. `prompt_cache_retention`：默认 `in_memory`；按场景可选 `24h`（需确认模型支持）。
- 响应观测：
1. 必须采集 `usage.prompt_tokens_details.cached_tokens`。
2. 以 `cached_tokens > 0` 判定命中。
3. 低于最小前缀长度（通常 `< 1024`）命中为 0 属于正常现象。
- 语义边界：Prompt Caching 是性能优化，不是业务语义状态存储，不能替代会话状态与审计日志。

## 工程参数示例（011）
- 推荐 key 模板：
1. Planner：`clawx:nl:planner:<agent_id>:<project_id>`
2. Route Planner：`clawx:nl:route:<agent_id>:<project_id>:<route>`
3. Executor：`clawx:nl:execute:<agent_id>:<project_id>:<route_key>`
- 示例（Executor）：
1. `clawx:nl:execute:main:main:route`
2. `clawx:nl:execute:bid-all:bid-all:requirement_update`
- 生成约束：
1. 保留稳定前缀 `clawx:nl:<stage>`。
2. 仅使用低变字段（agent/project/route）。
3. 不要把时间戳、UUID、完整用户输入塞进 key。

- retention 选择：
1. 默认：`in_memory`（当前默认值，建议保留）。
2. 长 retention（如 `24h`）：仅在模型能力和平台策略确认支持后启用。

- 环境变量示例：
```bash
# 默认可不设置（回落 in_memory）
export CLAWX_PROMPT_CACHE_RETENTION=in_memory
```

- 响应观测示例（trace.jsonl）：
```json
{
  "event": "llm_io",
  "phase": "response",
  "channel": "discord",
  "agent_id": "main",
  "intent_kind": "execute",
  "prompt_cached_tokens": 512,
  "prompt_tokens": 1024
}
```

## 分阶段请求（推荐）
- 默认采用“两段或三段”：
1. `Planner`：轻量识别 `intent/target_agent/risk/route`。
2. `Router`：按 route 仅加载所需 skill 元数据与最小上下文。
3. `Executor`：输出结构化计划（如 `control_plan` / `requirement_sync`）并由 ClawX 执行器落地。
- 原则：静态前缀尽量稳定、动态内容尽量后置，以提升 Prompt Caching 命中率。
- 禁止一次性注入全量技能正文与全量历史上下文。

## 官方参考
- API 指南（Prompt Caching）：https://platform.openai.com/docs/guides/prompt-caching
- 发布说明：https://openai.com/index/api-prompt-caching/

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
