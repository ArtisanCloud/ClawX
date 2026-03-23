# 任务清单：Autonomous Execution & Recovery

**输入**: `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/`  
**前置条件**: `spec.md`、`plan.md`

## Phase 1：Failure Classification（P1）

- [ ] T001 定义 `FailureClass` 与分类接口于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/classifier.go`
- [ ] T002 在执行失败主链路接入分类器于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [ ] T003 增加失败分类单元测试于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`

## Phase 2：Recovery Registry（P1）

- [ ] T004 定义 `RecoveryPolicy` 与注册中心于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/recovery_registry.go`
- [ ] T005 定义恢复执行器（重试/退避/熔断）于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/executor.go`
- [ ] T006 为 network/tool/permission 三类失败补齐首批策略于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`
- [ ] T007 增加恢复执行器单元测试于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/`

## Phase 3：Escalation Policy（P1）

- [ ] T008 定义升级提问策略（最小打扰）于 `/home/ubuntu/workspace/ClawX/internal/application/autonomy/escalation_policy.go`
- [ ] T009 统一升级提问模板（已尝试+失败摘要+推荐动作）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [ ] T010 增加“非必要不提问”契约测试于 `/home/ubuntu/workspace/ClawX/tests/contract/`

## Phase 4：Audit & Replay（P1）

- [ ] T011 新增自治日志记录器 `autonomy.jsonl` 于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/logging/autonomy_log.go`
- [ ] T012 在自治流程写入 `classify -> attempt -> escalate -> result` 事件于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [ ] T013 新增基础查询/汇总 CLI（可选 `clawx trace autonomy-report`）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/`

## Phase 5：UX 与输出约定（P2）

- [ ] T014 固化“接收/进度/完成/失败”回执模板于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [ ] T015 默认简报模式（长代码/附件按需输出）回归验证于 `/home/ubuntu/workspace/ClawX/tests/integration/`
- [ ] T016 更新运维文档（自治日志、报表、排障）于 `/home/ubuntu/workspace/ClawX/docs/11_operations/operations.md`

## Phase 6：收口与回归（P1）

- [ ] T017 增加端到端集成测试（成功恢复、升级提问、失败收敛）于 `/home/ubuntu/workspace/ClawX/tests/integration/`
- [ ] T018 执行全量回归 `go test ./... -count=1` 并记录结果于 `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/`
- [ ] T019 补充交付摘要（SC 对照）于 `/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/tasks.md`
