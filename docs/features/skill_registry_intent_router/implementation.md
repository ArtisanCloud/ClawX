# 实现状态（已落地）

## 模块落地
- `internal/domain/skill`:
  - Skill 定义、状态、快照、意图决策模型。
- `internal/application/skillregistry`:
  - 发现/解析/冲突处理/刷新编排。
  - `list/reload/enable/disable` 所需服务接口。
- `internal/application/intent`:
  - 固定优先级路由流水线。
  - 显式 `/skill`、规则匹配、LLM 兜底、多候选自动单选。
  - 权限评估与错误分类。
- `internal/infrastructure/skills`:
  - `SKILL.md` frontmatter 解析。
  - 多来源扫描。
  - 索引原子写入与 DM pairing 文件存储。
- `cmd/clawx`:
  - 新增 `skill` CLI 子命令。
  - 启动时注入 Skill Registry + Intent Router。
  - Discord/Telegram 入站分支支持 `kind=skill`。

## 配置结构（config.json）
```json
{
  "skills": {
    "enabled": true,
    "sources": {
      "userDir": "~/.clawx/skills",
      "workspaceDir": ".clawx/skills",
      "builtinEnabled": true,
      "builtinDir": "internal/skills/builtin"
    },
    "disabledNames": [],
    "allowlist": {
      "users": [],
      "channels": []
    },
    "defaultMode": "channel_allowlist_dm_pairing",
    "pairingTTLSeconds": 604800
  },
  "intentRouter": {
    "mode": "rule_first_llm_fallback",
    "llmFallback": {
      "enabled": true,
      "confidenceThreshold": 0.72
    }
  }
}
```

## CLI 命令
- `clawx skill list`
- `clawx skill reload`
- `clawx skill enable <name>`
- `clawx skill disable <name>`

## 日志字段
- `conversation_id`
- `intent.kind`
- `intent.reason`
- `intent.skill`
- `intent.confidence`
- `session_id`
- `duration_ms`

## Staged Routing（011）实现细则
- 入口：自然语言请求会在执行输入中注入 `[Staged Routing Snapshot]`，包含 `intent.*`、`route.*`、`fallback.*`、`execution.can_execute`。
- 阶段模型：
  - `IntentPlan`: `type/intent/route/risk/complexity/target_agent_id`
  - `RoutePlan`: `type/route/use_route_planner/context_budget_tier/tool_hints`
- 短路策略：
  - 低复杂度请求允许 `SkipRoutePlanner=true`，减少二阶段成本。
  - `skill` 路由默认保留 route planner，避免误选技能。
- 安全回退：
  - 任一阶段校验失败、空输入或路由不一致时，标记 `fallback.enabled=true`。
  - 回退模式仅 `clarify/suggest`，并设置 `execution.can_execute=false`，禁止副作用执行。
- 执行边界：
  - 结构化执行仍复用 `control_plan`/`requirement_sync` 既有解析与审计链路。
  - 非 `/` 自然语言不会触发文本 `/command` 自动执行。

## Prompt Cache Key 设计细则（011）
- 设计目标：稳定高复用前缀 + 低变维度，提升 `cached_tokens` 命中率。
- 当前模板（trace / execution）：
  - `clawx:nl:execute:<agent_id>:<project_id>:<route_key>`
- 字段约束：
  - `agent_id`、`project_id` 缺省回落到稳定默认值（如 `default` / `main`）。
  - `route_key` 做分隔符清洗（`:/\ 空格 -> _`），避免高噪声 key。
  - 禁止把时间戳、用户输入全文、随机 ID 放入 key。
- retention 策略：
  - 默认 `in_memory`（稳定、风险低）。
  - 仅在模型/平台确认支持时再切换更长 retention。
- 可观测字段：
  - 请求：`prompt_cache_key`、`prompt_cache_retention`
  - 响应：`prompt_cached_tokens`、`prompt_tokens`
  - 事件：`llm_io`（`phase=request/response`）
