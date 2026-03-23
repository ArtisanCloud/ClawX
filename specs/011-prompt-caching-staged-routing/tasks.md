# 任务清单：Prompt Caching + Staged LLM Routing

**输入**: `/home/ubuntu/workspace/ClawX/specs/011-prompt-caching-staged-routing/`  
**前置条件**: `spec.md`、`plan.md`

## Phase 1：基础接入（Prompt Caching Plumbing）

- [x] T001 在 OpenAI 请求构建路径增加 `prompt_cache_key` 参数于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/backend/profile_runner.go`
- [x] T002 在 OpenAI 请求构建路径增加 `prompt_cache_retention` 参数（默认 `in_memory`）于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/backend/profile_runner.go`
- [x] T003 在响应解析中采集 `usage.prompt_tokens_details.cached_tokens` 于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/backend/codex_trace_log.go`
- [x] T004 在 trace 事件 `llm_io` 增加缓存命中字段于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T005 新增 Prompt Caching 参数/响应单元测试于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/backend/`

## Phase 2：分阶段编排（Planner -> Route Planner -> Executor）

- [x] T006 定义 `IntentPlan` 结构与校验器于 `/home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/intent_plan.go`
- [x] T007 定义 `RoutePlan` 结构与校验器于 `/home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/route_plan.go`
- [x] T008 定义阶段编排器（含短路策略）于 `/home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/staged_router.go`
- [x] T009 将自然语言主路径接入阶段编排器于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T010 在阶段 3 继续复用既有 `control_plan`/`requirement_sync` 执行器于 `/home/ubuntu/workspace/ClawX/cmd/clawx/agent_chat.go` 与 `/home/ubuntu/workspace/ClawX/cmd/clawx/requirement_docs.go`

## Phase 3：安全回退与一致性

- [x] T011 实现阶段失败统一回退策略（澄清/建议，不执行）于 `/home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/staged_router.go`
- [x] T012 增加“禁止文本命令自动执行”回归断言于 `/home/ubuntu/workspace/ClawX/tests/contract/control_plan_no_regex_autorun_contract_test.go`
- [x] T013 增加 staged routing 集成测试（命中/短路/失败回退）于 `/home/ubuntu/workspace/ClawX/tests/integration/staged_routing_flow_test.go`
- [x] T014 增加 requirement_sync 指定 agent_id 的跨 agent 会话切换集成测试于 `/home/ubuntu/workspace/ClawX/tests/integration/requirement_sync_agent_switch_flow_test.go`

## Phase 4：可观测与门禁

- [x] T015 增加缓存命中率统计脚本或聚合器于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/logging/`
- [x] T016 在运维文档补充 `cached_tokens` 观测方法与阈值建议于 `/home/ubuntu/workspace/ClawX/docs/11_operations/`
- [x] T017 增加性能基线测试（分阶段前后 token/延迟对比）于 `/home/ubuntu/workspace/ClawX/tests/integration/staged_routing_performance_test.go`
- [x] T018 执行全量回归 `go test ./... -count=1` 并记录结果于 `/home/ubuntu/workspace/ClawX/specs/011-prompt-caching-staged-routing/`

## Phase 5：文档收口

- [x] T019 更新 `docs/features/skill_registry_intent_router/implementation.md` 补充 staged routing 与 cache key 设计细则
- [x] T020 更新 `docs/07_skills/skills.md` 补充工程参数示例（key 模板、retention 选择）
- [x] T021 产出 011 交付摘要（SC 对照）于 `/home/ubuntu/workspace/ClawX/specs/011-prompt-caching-staged-routing/tasks.md`

## 011 交付摘要（SC 对照）

- SC-001 `cached_tokens` 可观测完整率：
  - 已在 `llm_io` response 事件记录 `prompt_cached_tokens` 与 `prompt_tokens`，并提供聚合器统计。
  - 对应实现：`cmd/clawx/main.go`、`internal/infrastructure/logging/prompt_cache_metrics.go`。
- SC-002 token 成本下降：
  - 已引入 staged routing 短路策略（低复杂度跳过 route planner），并提供性能基线测试验证 token/latency 下降方向。
  - 对应测试：`tests/integration/staged_routing_performance_test.go`。
- SC-003 结构化执行误触发控制：
  - 执行阶段继续复用结构化 `control_plan`/`requirement_sync` 解析与校验；普通文本命令不自动执行。
  - 对应测试：`tests/contract/control_plan_no_regex_autorun_contract_test.go`。
- SC-004 Planner 阶段性能门限：
  - 已建立 staged router 平均延迟门禁（integration 性能测试内断言）。
  - 对应测试：`tests/integration/staged_routing_performance_test.go`。

## Phase 6：运维工具化（Post-Doc Closure）

- [x] T022 新增 `clawx trace cache-report` CLI（读取 trace 并输出 prompt cache 命中率）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/trace_cli.go`
- [x] T023 在运维手册补充 CLI 使用方式（text/json）于 `/home/ubuntu/workspace/ClawX/docs/11_operations/operations.md`
- [x] T024 增加 trace CLI 单元测试并通过回归于 `/home/ubuntu/workspace/ClawX/cmd/clawx/trace_cli_test.go`

## Phase 7：文档边界对齐（Scope Alignment）

- [x] T025 在 011 `spec.md` 明确范围边界：仅缓存与分阶段编排，不含通用自治恢复
- [x] T026 在 011 `plan.md` 增加 Out-Of-Scope 与后续自治层衔接说明
- [x] T027 在任务清单中记录 011 与后续自治能力的职责分界，避免需求混入 011
