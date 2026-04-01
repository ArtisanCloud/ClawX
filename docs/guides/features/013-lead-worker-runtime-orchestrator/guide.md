# Lead/Worker Runtime Orchestrator 使用指导（版本：v1.0）

## 1. 功能目标
- 让用户只说“启动”就能初始化运行时调度层。
- Lead 只做编排，不直接执行业务 shell。
- Worker 承担执行，支持空闲优先派工、同资源粘性派工。
- Worker 掉线可自动回收 running 任务并重派。

## 2. 运行时目录与关键文件
- 目录：`<workspace>/.clawx/runtime/`
- `tasks.jsonl`：任务队列（queued/running/succeeded/failed/canceled）
- `worker_states.json`：worker 状态（idle/online/busy/offline）
- `heartbeats.json`：心跳时间
- `runtime_meta.json`：运行时元信息（`lead_mode=true`）
- `dispatch_state.json`：粘性路由状态

## 3. 标准流程（SOP）
1. 在目标 agent 下发送自然语言：`启动`
2. 系统执行 `runtime.bootstrap`，初始化 runtime 文件与默认 worker 池
3. 后续业务执行请求进入 `action_plan.runtime.exec`
4. 若 `lead_mode=true`，请求不会直接执行 shell，而是入队+派工
5. 若检测到 stale worker，会触发 recovery：回收任务并重派

## 4. 回执规范（用户可见）
- 统一四段：`结论`、`产物/问题`、`证据`、`下一步`
- Spec Kit 缺文档时必须明确：
  - 缺失文件
  - 当前不进入实现阶段
  - 可直接回复的下一句（例如“继续补齐 PLAN 和 ANALYZE”）

## 5. 代码映射
- 启动与执行入口：`cmd/clawx/action_plan.go`
- Lead 执行约束与回执：`cmd/clawx/runtime_exec_plan.go`
- runtime 初始化：`internal/application/runtimeorchestrator/service.go`
- 队列：`internal/application/runtimeorchestrator/queue.go`
- 注册表：`internal/application/runtimeorchestrator/registry.go`
- 派工器：`internal/application/runtimeorchestrator/dispatcher.go`
- 恢复引擎：`internal/application/runtimeorchestrator/recovery.go`
- 调度审计日志：`internal/infrastructure/logging/runtime_orchestrator_log.go`

## 6. 验收建议
- 启动后检查 runtime 文件是否齐全。
- 连续提交同资源任务，确认 worker 粘性。
- 人工制造 stale worker，确认任务被回收重派。
- 触发 Spec Kit 不完整场景，确认提示缺失文件与下一步。

## 7. 变更记录
| 日期 | 修改人 | 变更内容 |
|---|---|---|
| 2026-03-31 | Codex | 新增 013 Lead/Worker Runtime Orchestrator 使用指导 |
