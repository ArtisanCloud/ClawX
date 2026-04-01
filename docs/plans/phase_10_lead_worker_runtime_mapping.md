# Phase 10：Lead/Worker Runtime 对齐落地方案（Agent Kit -> ClawX）

## 1. 目标
- 在 ClawX 内落地「主 Agent 只调度，Worker 并行执行」模型。
- 用户只说自然语言（如“启动”），系统自动完成初始化、派工、监控、恢复。
- 保持现有铁律：`/` 命令走规则，非 `/` 自然语言走 LLM 规划。

## 2. 对齐原则
- Lead 不直接执行业务命令，不直接改业务文件。
- Worker 才执行任务（shell、代码、测试、抓取、通知）。
- 所有调度决策基于状态账本（heartbeat + lifecycle），不是基于猜测。
- 会话中断/压缩后必须可恢复，不丢任务上下文。

## 3. ClawX 模块设计

### 3.1 Runtime Orchestrator（新增）
- 职责：
  - 维护 Lead/Worker 拓扑
  - 任务入队、分发、重试、取消
  - 读取状态账本做恢复决策
- 建议位置：
  - `internal/application/runtimeorchestrator/`

### 3.2 Worker Registry（新增）
- 职责：
  - Worker 注册、心跳、状态上报、注销
  - 维护 worker lifecycle：`online -> busy -> progress -> idle -> error -> shutdown`
- 持久化：
  - `~/.clawx/workspaces/<agent>/.clawx/runtime/worker_states.json`
  - `~/.clawx/workspaces/<agent>/.clawx/runtime/heartbeats.json`

### 3.3 Task Queue（新增）
- 职责：
  - 存储待执行任务、进行中任务、完成任务
  - 支持优先级、重试次数、超时、取消
- 持久化：
  - `~/.clawx/workspaces/<agent>/.clawx/runtime/tasks.jsonl`

### 3.4 Dispatcher（新增）
- 职责：
  - Lead 按“空闲 Worker + 能力标签”分发任务
  - 同一实体/同一资源优先绑定同一 Worker（避免并发冲突）
- 规则：
  - 全忙时可排队；可选“临时 Worker”策略（后续）

### 3.5 Recovery Engine（增强）
- 职责：
  - 发现超时心跳 Worker
  - 判断是否可恢复或替换
  - 对未完成任务回收并重派
- 数据依据：
  - heartbeat 时间戳 + worker status + task state

### 3.6 Observability（增强）
- 统一追踪：
  - `~/.clawx/logs/trace.jsonl`
  - `~/.clawx/logs/runtime_exec.jsonl`
  - `~/.clawx/logs/runtime_orchestrator.jsonl`（新增）
- 对用户回执：
  - 仅给“结论/产物/证据/下一步”
  - 技术细节写日志，不刷屏

## 4. 启动流（自然语言“启动”）
1. 识别为 runtime bootstrap 意图（非 `/`，LLM 输出 `action_plan`）。
2. 校验当前 agent workspace 完整性（文档、目录、权限）。
3. 初始化 runtime 状态文件（tasks/worker_states/heartbeats）。
4. 创建默认 worker 池（如 `planner/executor/reviewer`）。
5. 启动调度循环（queue poll + heartbeat check）。
6. 回执“已就绪 + 当前 worker 数 + 下一步建议”。

## 5. 数据结构（建议）

### 5.1 WorkerState
```json
{
  "worker_id": "w-executor-1",
  "role": "executor",
  "status": "idle",
  "last_heartbeat_at": "2026-03-30T12:00:00Z",
  "current_task_id": "",
  "meta": {}
}
```

### 5.2 RuntimeTask
```json
{
  "task_id": "t-20260330-001",
  "source": "nl",
  "intent": "spec.plan",
  "payload": {"feature":"source-keyword-monitoring"},
  "status": "queued",
  "assigned_worker_id": "",
  "retry": 0,
  "max_retry": 3,
  "created_at": "2026-03-30T12:00:00Z"
}
```

## 6. 与现有 ClawX 能力对齐
- 继续使用 `action_plan` 作为唯一结构化执行协议。
- `runtime.exec` 仍由 ClawX 原生执行器执行（非 Codex 沙盒代执行）。
- 需求文档同步继续复用现有：
  - `TASK_PLAN.md`
  - `TASK_EXECUTION.md`
  - `.clawx/workspace-state.json`
- Spec Kit 文档完整性门禁继续保留（缺 `PLAN/ANALYZE` 不进实现）。

## 7. 分阶段实施

### Phase A（P1，先可用）
- 新增 Orchestrator + Worker Registry + Task Queue 基础实现
- 实现“启动”自动初始化 worker 池
- 实现单任务分发与回执闭环

### Phase B（P1，稳定化）
- 心跳超时检测 + 任务回收重派
- worker 生命周期持久化与恢复
- 增加 runtime_orchestrator 审计日志

### Phase C（P2，增强）
- 能力标签路由（按 skill/tool capability 分派）
- 临时 Worker 弹性扩缩
- 可视化 runtime 状态面板（可选）

## 8. 验收标准
- 用户只发“启动”，可完成 runtime 初始化，不需手动命令。
- Lead 不执行业务命令，所有业务动作都有 worker 归属。
- 任一 worker 异常退出后，任务可在 60s 内恢复到可执行状态。
- 回执必须可读：告诉用户“当前状态 + 下一步”，不要求用户猜命令。

## 9. 下一步（建议立即开工）
- 先开 spec：`specs/013-lead-worker-runtime-orchestrator/`
- 按 Spec Kit 流程生成：`spec/plan/tasks/analyze`
- 再分两批实现：
  - 批次 1：bootstrap + registry + queue
  - 批次 2：dispatcher + recovery + observability
