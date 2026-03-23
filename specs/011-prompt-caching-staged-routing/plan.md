# Implementation Plan: 011-prompt-caching-staged-routing

**Branch**: `011-prompt-caching-staged-routing` | **Date**: 2026-03-22 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/011-prompt-caching-staged-routing/spec.md](/home/ubuntu/workspace/ClawX/specs/011-prompt-caching-staged-routing/spec.md)
**Input**: Feature specification from `/specs/011-prompt-caching-staged-routing/spec.md`

## Summary

在不改变 ClawX 执行安全边界的前提下，为自然语言路径引入 OpenAI Prompt Caching 与分阶段 LLM 编排能力，形成“可命中、可观测、可回退”的稳定调用链路。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: 现有 `cmd/clawx/main.go` 执行链路、`internal/application/skillorchestrator`、trace logger  
**Storage**: 结构化 trace 日志（`~/.clawx/logs/trace.jsonl`）+ 现有审计存储  
**Testing**: `go test ./...` + `tests/contract` + `tests/integration`  
**Constraints**:
- 非 `/` 自然语言路径必须保持 LLM-first。
- 执行端仅消费结构化计划，不消费自由文本命令。
- Prompt Caching 仅用于性能优化，不能承担状态语义。

## Constitution Check

| Gate | 检查项 | 结果 |
|------|--------|------|
| Session-First | 会话路由与会话切换语义保持不变 | PASS |
| Agent Template | Agent 仍为运行时模板，新增仅为路由编排能力 | PASS |
| Direct Execution | 不引入外部重编排服务 | PASS |
| Boundary Stable | Planner/Router/Executor 边界可替换 | PASS |
| Docs-driven | 范围由 Phase 8 与 011 spec 驱动 | PASS |

## Project Structure

```text
specs/011-prompt-caching-staged-routing/
├── spec.md
├── plan.md
└── tasks.md
```

涉及代码目录：

```text
cmd/clawx/
  main.go
  requirement_docs.go
internal/application/skillorchestrator/
  (planner/parser/executor)
internal/infrastructure/backend/
  profile_runner.go
internal/infrastructure/logging/
  rotating_writer.go
tests/
  contract/
  integration/
  unit/
```

## Phase Design

1. **Phase A - Cache Plumbing**
- 在 OpenAI 请求构建处加入 `prompt_cache_key`、`prompt_cache_retention`。
- 在响应采集处写入 `cached_tokens` 指标与 stage 字段。

2. **Phase B - Staged Routing**
- 抽象第一阶段 `IntentPlan` 输出。
- 抽象第二阶段 `RoutePlan`（按技能/工具线路缩小上下文）。
- 第三阶段输出 `ExecutionPlan` 并交给既有执行器。

3. **Phase C - Safety + Fallback**
- 任一阶段解析失败统一回退澄清/建议路径。
- 保持 `control_plan` / `requirement_sync` 的既有校验与审计门禁。

## Risks

- 风险：阶段拆分增加一次网络调用导致延迟上升。  
  缓解：低复杂请求短路二阶段。
- 风险：缓存键设计不当导致低命中。  
  缓解：固定系统前缀 + 阶段标识 + 低变字段。

## Out-Of-Scope Clarification

- 011 不负责“执行器自治恢复”（如依赖安装失败后的多源重试、权限升级决策、网络故障自愈）。
- 011 不定义“异步任务回执协议”（收到/进度/失败模板）。
- 011 只提供自治所需的结构化基础：
  - 分阶段结构化计划；
  - 缓存与 token 可观测；
  - 回退时不执行副作用的安全边界。

## Handoff To Autonomy Layer

- 后续自治能力应在独立规格中实现（建议新 spec），并复用：
  - `internal/application/skillorchestrator/*plan*.go` 的结构化计划；
  - `cmd/clawx/main.go` 中既有 trace/audit；
  - `tests/contract` 的“禁止自由文本自动执行”门禁。
