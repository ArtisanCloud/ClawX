# Implementation Plan: 013-lead-worker-runtime-orchestrator

**Branch**: `013-lead-worker-runtime-orchestrator` | **Date**: 2026-03-30 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/013-lead-worker-runtime-orchestrator/spec.md](/home/ubuntu/workspace/ClawX/specs/013-lead-worker-runtime-orchestrator/spec.md)  
**Input**: Feature specification from `/specs/013-lead-worker-runtime-orchestrator/spec.md`

## Summary

在 ClawX 内新增 Lead/Worker 运行时编排层：Lead 只调度，Worker 并行执行；自然语言“启动”自动完成运行时初始化；中断后可基于心跳与任务队列恢复；规范请求受 Spec Kit 四件套完整性门禁约束。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: `cmd/clawx/main.go`、`cmd/clawx/action_plan.go`、`cmd/clawx/runtime_exec_plan.go`  
**New Modules**: `internal/application/runtimeorchestrator`（新增）  
**Storage**:
- `~/.clawx/workspaces/<agent>/.clawx/runtime/tasks.jsonl`
- `~/.clawx/workspaces/<agent>/.clawx/runtime/worker_states.json`
- `~/.clawx/workspaces/<agent>/.clawx/runtime/heartbeats.json`
- `~/.clawx/logs/runtime_orchestrator.jsonl`
**Testing**: `go test ./... -count=1` + contract/integration 新增用例  
**Constraints**:
- 保持 `/` 与非 `/` 路由铁律
- 不破坏现有 `action_plan` 与 `runtime.exec` 执行协议
- 回执必须可读，不输出模板噪音

## Constitution Check

| Gate | 检查项 | 结果 |
|------|--------|------|
| LLM-first | 非命令请求走 LLM 编排 | PASS |
| Safety | Lead/Worker 边界与恢复策略可审计 | PASS |
| Auditability | 调度链路日志完整 | PASS |
| Scope Isolation | 按 agent workspace 隔离 runtime 状态 | PASS |

## Project Structure

```text
specs/013-lead-worker-runtime-orchestrator/
├── spec.md
├── plan.md
├── tasks.md
└── analyze.md
```

涉及代码目录（规划）：

```text
cmd/clawx/
  main.go
  action_plan.go
  runtime_exec_plan.go
  speckit_flow_state.go
internal/application/runtimeorchestrator/
  service.go
  queue.go
  registry.go
  dispatcher.go
  recovery.go
internal/infrastructure/logging/
  runtime_orchestrator_log.go
tests/contract/
tests/integration/
```

## Phase Design

1. **Phase A - Runtime Bootstrap（P1）**
- 支持自然语言“启动”触发 runtime 初始化。
- 创建 runtime 状态目录与默认 worker 池。

2. **Phase B - Queue + Registry（P1）**
- 引入 task queue 与 worker registry 持久化。
- 实现 enqueue/assign/update 完整状态流转。

3. **Phase C - Dispatcher（P1）**
- Lead 调度策略：空闲优先、同资源粘性分配。
- 禁止 Lead 直接执行 task 业务命令。

4. **Phase D - Recovery（P1）**
- 心跳超时检测、任务回收、重派。
- 输出恢复动作审计日志。

5. **Phase E - UX + Spec Gate（P1）**
- 缺文档时强制提示缺项与明确下一步。
- 会话级 Spec Kit 补齐上下文继承（“继续补齐”不误判）。

## Risks

- 风险：调度状态竞争导致重复派工。  
  缓解：任务分配原子更新 + worker 锁。
- 风险：恢复机制误判 busy worker 为 dead。  
  缓解：超时阈值 + busy 宽限窗口 + 二次探活。
- 风险：回执信息过多影响可读性。  
  缓解：固定“结论/产物/证据/下一步”四段式。
