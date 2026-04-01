# 功能规格说明：Lead/Worker Runtime Orchestrator

**功能分支**: `013-lead-worker-runtime-orchestrator`  
**创建时间**: 2026-03-30  
**状态**: Draft  
**输入**: 用户要求“主 Agent 不干活，只做调度；Worker 并行执行；自然语言‘启动’可自动初始化与运行”。

## 用户场景与测试

### 用户故事 1 - 一句话启动运行时（P1）

作为用户，我希望只说“启动”，系统就自动完成项目运行时初始化，而不是让我记命令。

**验收场景**:
1. 用户发送“启动”，系统自动初始化 runtime 状态目录与默认 worker 池。
2. 返回可读回执：当前 worker 数、状态、下一步建议。
3. 不要求用户先输入 `/config` 或 shell 指令。

### 用户故事 2 - Lead 只调度，Worker 执行（P1）

作为用户，我希望主 Agent 不直接执行任务，所有实际工作由 Worker 并行完成。

**验收场景**:
1. Lead 负责任务分配、汇总、恢复，不直接执行业务 `runtime.exec`。
2. 至少支持 `planner/executor/reviewer` 三类 worker 角色。
3. 多任务时可并行派发到空闲 worker。

### 用户故事 3 - 会话中断后可恢复（P1）

作为用户，我希望发生会话中断、压缩或 worker 掉线时，系统自动恢复，不丢任务。

**验收场景**:
1. worker 心跳超时可被检测。
2. 未完成任务可回收并重新分派。
3. 恢复动作有审计日志可追溯。

### 用户故事 4 - Spec Kit 先行门禁（P1）

作为用户，我希望任何实现任务都先补齐 Spec Kit 四件套，避免直接开写代码。

**验收场景**:
1. 规范请求时自动生成 `SPEC.md/PLAN.md/TASKS.md/ANALYZE.md`。
2. 文档缺项时明确提示缺失文件与下一步动作。
3. 在同一会话“继续补齐”不会误触发实现证据门禁。

## 功能需求

- **FR-001**: 系统必须提供 Runtime Orchestrator 组件，负责任务入队、分发、回收、重派。
- **FR-002**: 系统必须提供 Worker Registry，维护 worker 生命周期与心跳状态。
- **FR-003**: 系统必须提供持久化 Task Queue（支持 queued/running/succeeded/failed/canceled）。
- **FR-004**: 自然语言“启动”必须可自动触发 runtime bootstrap。
- **FR-005**: Lead 在编排模式下不得直接执行业务任务命令；业务执行应归属于 worker。
- **FR-006**: 系统必须支持 worker 心跳超时检测与任务恢复重派。
- **FR-007**: 系统必须在日志中记录调度链路：`enqueue -> assign -> execute -> complete/recover`。
- **FR-008**: 规范类请求必须启用 Spec Kit 完整性门禁，缺 `PLAN.md` 或 `ANALYZE.md` 不得进入实现阶段。
- **FR-009**: 非 `/` 自然语言保持 LLM-first；`/` 命令保持规则路径。
- **FR-010**: 用户回执必须包含“结论 + 产物 + 证据 + 下一步”，禁止只输出模板化成功语句。

## 关键实体

- **RuntimeTask**: 任务对象（intent、payload、status、retry、worker 归属）。
- **WorkerState**: worker 生命周期状态（online/busy/progress/idle/error/shutdown）。
- **HeartbeatRecord**: 心跳记录（worker_id、last_seen_at、stale）。
- **DispatchDecision**: 派工决策（task_id、worker_id、reason）。
- **RecoveryEvent**: 恢复动作（超时检测、回收、重派、结果）。

## 成功标准

- **SC-001**: 用户发送“启动”后，60 秒内完成 runtime 初始化并返回就绪回执（P95）。
- **SC-002**: 在编排模式下，Lead 直接执行业务命令比例为 0。
- **SC-003**: worker 异常场景中，90% 未完成任务可在 60 秒内被回收重派（测试样本内）。
- **SC-004**: Spec Kit 缺项提示命中率 100%，且提示包含“缺失文件 + 明确下一步”。
- **SC-005**: 调度审计日志完整率 100%（每个 task 都有关键阶段事件）。

## 与既有规格关系

- 依赖：
  - `012-autonomous-execution-recovery`（失败分类与恢复策略）
  - `011-prompt-caching-staged-routing`（LLM 分阶段编排）
- 不重复定义：
  - `action_plan` 协议与 `runtime.exec` 原生执行器
  - 既有 requirement/task 文档持久化机制
