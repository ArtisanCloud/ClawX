# Phase 3 - Multi Agent

## 目标
- 在多 Session 稳定后，引入可落地的多 Agent 能力。
- 先完成“多 Agent 运行模板 + 路由绑定”，再逐步补“多 Agent 协作编排”。
- 保持现有 `Router -> Session Manager -> Backend Adapter` 主链路不重构。

## 当前状态（基线）
- 已有多会话与窗口隔离（Phase 2）。
- 已有 Skill Registry 与意图路由（Phase 3 早期能力）。
- 已支持单 Agent 主流程与默认 Agent 回退。

## 与 OpenClaw 的差距（本阶段要覆盖）
- 已对齐部分：
  - 多 Agent 基础概念（Agent 作为运行模板）。
  - 会话与 agent 维度隔离（`agent_id` 参与上下文隔离）。
- 未对齐部分（重点）：
  - 缺少“子智能体协作编排”能力（类似 `sessions_spawn` 并行子任务）。
  - 缺少“父会话聚合子会话结果”的标准协议与状态机。
  - 缺少面向用户的 Agent 协作控制命令与观测视图（任务树、进度、失败重试）。

## 范围
### P3（必须）
- `Agent Registry`
- Agent 作为运行模板
- Session 记录 `agent_id`
- 默认 Agent
- 可选 `window -> agent` Binding

### P3.5（可选增强，建议本阶段一并规划）
- 多 Agent 协作任务模型（父任务 + 子任务 + 聚合结果）。
- 子任务派发接口（异步派发、状态查询、取消、超时）。
- 协作策略最小集：
  - 顺序分工（A -> B -> C）
  - 并行分工（A/B/C 并行，父任务聚合）
- 用户可见的协作控制命令（示例）：
  - `/agents`
  - `/agent use <agent_id>`
  - `/delegate <agent_id> <task>`
  - `/delegates`（查看协作子任务状态）

## 不做
- 重型中台
- 高级插件系统
- 多节点调度
- 跨实例分布式调度与队列系统
- 自动长期记忆同步（跨 agent 全局知识库）

## 里程碑
### M1: Agent Registry 与绑定（P3 MVP）
- 可配置多个 Agent（workspace/profile/tools policy）。
- 新建 Session 可明确绑定 `agent_id`。
- 支持 `window -> agent` 默认绑定和回退。

### M2: 协作编排最小闭环（P3.5）
- 父会话可派发 >=2 个子任务到不同 Agent。
- 子任务状态可追踪：`pending/running/success/failed/canceled`。
- 父会话可聚合子任务结果并返回统一回复。

### M3: 稳定性与治理
- 协作超时、取消、重试策略可配置。
- 审计日志包含 `agent_id/parent_session_id/subtask_id/duration/status`。
- 指标可观测（成功率、耗时、失败原因分布）。

## 验收标准（阶段门禁）
- 同一 Agent 下可创建多个 Session
- 不同 Agent 可拥有不同默认工作目录与运行配置
- 未配置 Agent 时仍可回退到默认流程
- 支持至少 1 个复杂任务被拆分为 2~3 个子任务并由不同 Agent 分工执行
- 子任务失败不会导致主进程崩溃，且父任务可得到明确失败摘要
- 多 Agent 协作链路可通过集成测试与人工验收脚本

## 测试与验收场景
- 场景 1：同一窗口切换 Agent，新建会话后执行路径命中新 Agent 配置。
- 场景 2：父任务派发两个子任务（代码分析 + 文档生成），最终聚合结果返回。
- 场景 3：其中一个子任务失败，父任务返回“部分成功 + 失败原因”。
- 场景 4：取消父任务时，所有运行中子任务被联动取消。

## 风险与缓解
- 风险：协作编排引入并发复杂度，容易出现状态不一致。
  - 缓解：先单机内存/文件状态机，保证原子状态迁移与幂等更新。
- 风险：子任务过多导致资源争抢。
  - 缓解：设置每父任务最大并发子任务数与全局并发上限。
- 风险：用户理解成本上升。
  - 缓解：先提供最小命令集与清晰状态反馈，不引入复杂 DSL。

## 交付物
- 计划文档：`docs/plans/phase_3_multi_agent.md`（本文件）
- 规格与任务（下一步）：
  - `specs/005-multi-agent-orchestration/spec.md`
  - `specs/005-multi-agent-orchestration/plan.md`
  - `specs/005-multi-agent-orchestration/tasks.md`
