# 任务清单：Lead/Worker Runtime Orchestrator

**输入**: `/home/ubuntu/workspace/ClawX/specs/013-lead-worker-runtime-orchestrator/`  
**前置条件**: `spec.md`、`plan.md`

## Phase 1：Runtime Bootstrap（P1）

- [x] T001 新增 runtime orchestrator 基础骨架于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/service.go`
- [x] T002 新增自然语言“启动”意图接入与 bootstrap 入口于 `/home/ubuntu/workspace/ClawX/cmd/clawx/main.go`
- [x] T003 初始化 runtime 目录与状态文件于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/`
- [x] T004 新增 bootstrap 集成测试（启动后状态文件存在）于 `/home/ubuntu/workspace/ClawX/tests/integration/`

## Phase 2：Queue + Registry（P1）

- [x] T005 新增任务队列模型与持久化读写于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/queue.go`
- [x] T006 新增 worker registry 与生命周期状态模型于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/registry.go`
- [x] T007 新增 heartbeat 上报与超时判定逻辑于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/registry.go`
- [x] T008 新增 queue/registry 单元测试于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/`

## Phase 3：Dispatcher（P1）

- [x] T009 新增派工策略（空闲优先 + 同资源粘性）于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/dispatcher.go`
- [x] T010 在执行路径加入“Lead 不直接执行业务 task”约束于 `/home/ubuntu/workspace/ClawX/cmd/clawx/runtime_exec_plan.go`
- [x] T011 新增 dispatcher 契约测试（并行任务分配）于 `/home/ubuntu/workspace/ClawX/tests/contract/`

## Phase 4：Recovery（P1）

- [x] T012 新增恢复引擎（回收超时任务并重派）于 `/home/ubuntu/workspace/ClawX/internal/application/runtimeorchestrator/recovery.go`
- [x] T013 新增 runtime orchestrator 审计日志于 `/home/ubuntu/workspace/ClawX/internal/infrastructure/logging/runtime_orchestrator_log.go`
- [x] T014 新增恢复链路集成测试（worker 掉线后任务恢复）于 `/home/ubuntu/workspace/ClawX/tests/integration/`

## Phase 5：Spec Kit 门禁与交互体验（P1）

- [x] T015 强化 Spec Kit 完整性门禁（缺 `PLAN.md/ANALYZE.md` 禁止进入实现）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/action_plan.go`
- [x] T016 会话级 Spec Kit 补齐上下文继承优化（“继续补齐”可续流）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/speckit_flow_state.go`
- [x] T017 统一可读回执（结论/产物/证据/下一步）于 `/home/ubuntu/workspace/ClawX/cmd/clawx/runtime_exec_plan.go`
- [x] T018 新增门禁与回执契约测试于 `/home/ubuntu/workspace/ClawX/tests/contract/`

## Phase 6：收口与回归（P1）

- [x] T019 执行全量回归 `go test ./... -count=1` 并记录结果
- [x] T020 更新指南文档（SOP + runtime 调度说明）于 `/home/ubuntu/workspace/ClawX/docs/guides/features/`
- [x] T021 输出交付摘要（SC 对照 + 风险遗留）到本文件

## 交付摘要（T021）

### 1) 全量回归结果（T019）
- 执行时间：2026-03-31 (UTC)
- 命令：`go test ./... -count=1`
- 结果：通过（0 failed）
- 关键包：
  - `clawx/tests/contract` 通过（65.014s）
  - `clawx/tests/integration` 通过（61.887s）
  - `clawx/cmd/clawx`、`internal/application/runtimeorchestrator`、`internal/infrastructure/logging` 均通过

### 2) 指南更新结果（T020）
- 新增：
  - `/home/ubuntu/workspace/ClawX/docs/guides/features/013-lead-worker-runtime-orchestrator/guide.md`
- 索引更新：
  - `/home/ubuntu/workspace/ClawX/docs/guides/README.md`

### 3) 成功标准（SC）对照
- SC-001（启动可用）：已达成。`runtime.bootstrap` 已落地并有集成测试验证。
- SC-002（Lead 不直接执行业务）：已达成。`lead_mode=true` 下 `runtime.exec` 走入队与派工。
- SC-003（掉线恢复重派）：已达成。Recovery 引擎支持 stale worker 检测、requeue、reassign，且有测试。
- SC-004（Spec Kit 缺项提示）：已达成。缺项提示包含缺失文件与明确下一步，支持“继续补齐”续流。
- SC-005（调度审计链路）：部分达成。本期已落地 recovery 审计（stale/requeue/reassign）；完整覆盖 `enqueue -> assign -> execute -> complete` 仍需扩展。

### 4) 风险与遗留
- 尚未接入统一“执行完成/失败”事件审计（当前 recovery 事件已记录，execute/complete 仍有缺口）。
- 当前 worker 执行器仍处于编排优先形态，尚未引入独立 worker 进程池与并发执行上限策略。
