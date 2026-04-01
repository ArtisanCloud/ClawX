# 任务清单：Autonomous Execution & Recovery

**输入**: `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/`  
**前置条件**: `spec.md`、`plan.md`

## Phase 1：Failure Classification（P1）

- [x] T001 定义 `FailureClass` 与分类接口于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/classifier.go`
- [x] T002 在执行失败主链路接入分类器于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T003 增加失败分类单元测试于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`
- [x] T003A 在 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go` 增加 routing iron law 守卫：非 `/` 必须走 LLM-first，`/command` 仅走命令规则路径
- [x] T003B 新增契约测试 `routing_iron_law_contract_test.go` 于 `/home/ubuntu/workspace/ClawX/tests/contract/`

## Phase 2：Recovery Registry（P1）

- [x] T004 定义 `RecoveryPolicy` 与注册中心于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/recovery_registry.go`
- [x] T005 定义恢复执行器（重试/退避/熔断）于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/executor.go`
- [x] T006 为 network/tool/permission 三类失败补齐首批策略于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`
- [x] T007 增加恢复执行器单元测试于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`

## Phase 3：Escalation Policy（P1）

- [x] T008 定义升级提问策略（最小打扰）于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/escalation_policy.go`
- [x] T009 统一升级提问模板（已尝试+失败摘要+推荐动作）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T010 增加“非必要不提问”契约测试于 `/home/ubuntu/workspace/ClawX/tests/contract/`
- [x] T010A 在自治执行器接入点强制复用 `control_plan`/`requirement_sync` 的 schema + allowlist gate 于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T010B 新增契约测试 `autonomy_schema_allowlist_gate_contract_test.go` 于 `/home/ubuntu/workspace/ClawX/tests/contract/`

## Phase 4：Audit & Replay（P1）

- [x] T011 新增自治日志记录器 `autonomy.jsonl` 于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/logging/autonomy_log.go`
- [x] T012 在自治流程写入 `classify -> attempt -> escalate -> result` 事件于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T013 新增基础查询/汇总 CLI `clawx trace autonomy-report` 于 `/home/ubuntu/workspace/ClawX/cmd/clawx/`

## Phase 5：UX 与输出约定（P2）

- [x] T014 固化“接收/进度/完成/失败”回执模板于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T015 默认简报模式（长代码/附件按需输出）回归验证于 `/home/ubuntu/workspace/ClawX/tests/integration/`
- [x] T016 更新运维文档（自治日志、报表、排障）于 `/home/ubuntu/workspace/ClawX/docs/11_operations/operations.md`

## Phase 6：收口与回归（P1）

- [x] T017 增加端到端集成测试（成功恢复、升级提问、失败收敛）于 `/home/ubuntu/workspace/ClawX/tests/integration/`
- [x] T018 执行全量回归 `go test ./... -count=1` 并记录结果于 `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/`
- [x] T019 补充交付摘要（SC 对照）于 `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/tasks.md`

## 回归记录（T018）

- 日期：2026-03-24 (UTC)
- 分支：`012-autonomous-execution-recovery`
- 命令：`go test ./... -count=1`
- 结果：PASS（`cmd/clawx`、`internal/...`、`tests/contract`、`tests/integration`、`tests/unit` 全部通过）

## 交付摘要（T019，SC 对照）

- SC-001（recoverable 自动恢复成功率）：
  已补充覆盖“先失败后恢复成功”与“恢复失败后收敛停止”的回归用例，确保恢复策略按失败类型执行且在重试上限后终止。
  相关测试：`TestTryRecoverSessionFlowSuccessAfterRetry`、`TestTryRecoverSessionFlowFailureConverges`。
- SC-002（升级提问完整性）：
  升级提问输出继续强制包含“已尝试 + 推荐操作”，并有回归测试校验。
  相关测试：`TestHandleExecutionFailureEscalationPrompt`。
- SC-003（减少无效打扰）：
  通过恢复策略与升级策略分离（可恢复优先自动处理、不可恢复再提问）保持“非必要不提问”路径，已有契约测试持续保障。
- SC-004（自治日志完整率）：
  自治链路继续记录 `classify -> attempt -> escalate -> result`，并维持日志查询与报表能力（`trace autonomy-report`）回归通过。

## Phase 6 新增测试文件

- `/home/ubuntu/workspace/ClawX/cmd/clawx/autonomy_recovery_flow_test.go`
- `/home/ubuntu/workspace/ClawX/tests/integration/autonomy_recovery_flow_test.go`
