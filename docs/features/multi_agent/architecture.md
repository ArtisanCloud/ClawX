# 多会话与多 Agent 架构（V1.5）

## 目标
- 在现有会话模型上，明确 `Session` 与 `Agent` 的职责边界。
- 支持“多窗口对应多会话”的主流程。
- 为未来需要的多工作目录、多权限、多角色路由预留 `Agent` 能力。
- 保持第一阶段实现简单，不引入额外的常驻中台或复杂插件系统。

## 核心结论
- **V1 默认主模型是 `Window -> Session`**，不是 `Window -> Agent`。
- **`Session` 负责上下文隔离**：新建会话、恢复会话、会话历史、会话锁。
- **`Agent` 负责运行环境隔离**：工作目录、后端配置、工具权限、系统提示、入口绑定。
- 仅当需要“不同运行环境”时才启用多 Agent；仅仅需要“新开对话”时，新建 Session 即可。

## 适用边界

### 仅用多 Session 即可的场景
- 同一用户需要多个独立对话窗口。
- 多个窗口共享同一工作目录。
- 多个窗口共享同一后端与模型配置。
- 只需要避免上下文串线，不需要切换权限或角色。

### 需要引入多 Agent 的场景
- 不同窗口对应不同工作目录或代码仓库。
- 不同入口需要不同系统提示或角色设定。
- 不同入口需要不同工具权限或执行策略。
- 某些渠道、频道或线程需要固定路由到特定运行模板。
- 未来需要后台任务、定时任务或常驻任务使用独立运行环境。

## 核心概念
- **Window**：一个前端窗口或一个显式的交互容器，绑定一个当前 Session。
- **Session**：一次独立对话实例，保存对话历史、运行状态、锁和后端会话标识。
- **Backend Session Id**：后端侧的原生会话 ID，用于新建、恢复或列出会话。
- **Agent**：运行模板，描述默认工作目录、后端类型、模型、系统提示、工具权限等。
- **Profile**：后端运行配置，如 CLI 路径、超时、默认参数、鉴权来源。
- **Binding**：把某个入口（窗口、频道、线程、私聊）映射到默认 Agent 的规则。
- **Router**：收到请求后，决定使用哪个 Session、是否新建 Session、是否使用某个 Agent。

## 推荐架构

### V1 默认结构
- `Channel / UI Adapter`
- `Router`
- `Session Manager`
- `Backend Adapter`

### V1.5 演进结构
- `Channel / UI Adapter`
- `Router`
- `Session Manager`
- `Agent Registry`
- `Backend Adapter`

说明：
- `Agent Registry` 在 V1.5 中只需要是一个轻量配置层，不要求是独立服务。
- `Backend Adapter` 继续保持统一接口，按后端类型执行新建、恢复和单次调用。

## 混合实现策略

### 设计原则
- 采用“两层融合”方案，而不是一步到位引入重型中台。
- 第一层优先保证“能直接执行、能快速落地”。
- 第二层提供“多 Agent、入口绑定、后续扩展”的结构预留。

### 第一层：轻执行层
- 每次请求由 `Backend Adapter` 直接完成一次后端调用。
- `Session Manager` 负责保存本地 Session 与后端会话 ID 的映射。
- `Router` 只做本地路由，不依赖常驻网关。

适用目标：
- 快速跑通多窗口、多会话。
- 先把新建、恢复、继续执行打通。
- 保持链路短，方便调试和排障。

### 第二层：可扩展路由层
- 在轻执行层之上增加 `Agent Registry` 与 `Binding`。
- 入口先命中默认 Agent，再创建或恢复 Session。
- 运行参数通过 `Agent -> Session` 逐层下发，最终交给 `Backend Adapter`。

适用目标：
- 支持不同工作目录、不同运行模板、不同入口默认路由。
- 为未来统一投递、后台任务、更多入口类型预留稳定边界。

### 为什么采用融合方案
- 仅用轻执行层，足以完成多 Session，但无法优雅支持不同运行环境。
- 直接引入重型中台，会显著提高复杂度，不利于当前阶段迭代。
- 融合方案允许先做简单链路，再逐步扩展为更强的路由模型。

### 当前建议
- V1 保持“请求直达 `Backend Adapter`”。
- V1.5 增加 `Agent Registry` 与 `Binding`。
- 只有在多入口、多投递、多后台任务明确出现后，才考虑把 `Router` 演进为更重的统一调度层。

## 数据模型

### Session
- `id`
- `window_id`
- `agent_id`（可空，V1 可先不强制）
- `backend`
- `backend_session_id`
- `status` (`idle` | `running` | `error`)
- `cwd`
- `last_used_at`
- `lock_token`

说明：
- `backend_session_id` 用于映射后端原生会话。
- `cwd` 默认继承自 Agent；若用户手动切换，可写回 Session 级覆盖。

### Agent
- `id`
- `name`
- `workspace`
- `backend`
- `profile_id`
- `model`
- `system_prompt`
- `tools_policy`
- `enabled`

说明：
- `Agent` 是“运行模板”，不是“对话实例”。
- 一个 Agent 可以对应多个 Session。

### Binding
- `id`
- `scope_type` (`window` | `channel` | `thread` | `user`)
- `scope_key`
- `agent_id`
- `priority`
- `enabled`

说明：
- `Binding` 用于入口默认路由。
- 若没有命中 Binding，则回退到默认 Agent 或当前窗口已选 Agent。

## 路由规则

### 新建会话
1. UI 或渠道触发“新建会话”。
2. Router 解析当前入口上下文。
3. 根据 Binding 或当前窗口配置确定 `agent_id`。
4. Session Manager 创建新 Session。
5. Backend Adapter 创建后端会话，并回填 `backend_session_id`。

### 恢复会话
1. 用户选择已有 Session。
2. Router 校验 Session 是否存在且可访问。
3. Session Manager 将该 Session 绑定到当前 Window。
4. 后续请求继续使用该 Session 的 `backend_session_id`。

### 普通消息
1. Router 先定位当前 Window 绑定的 Session。
2. 若不存在，则按“新建会话”流程创建。
3. Session Manager 申请会话锁。
4. Backend Adapter 使用 `backend_session_id` 执行。
5. 输出回传后释放锁，更新状态与时间戳。

## 与现阶段能力的对应关系
- 轻执行层对应当前最小可用链路：
  - `Window / Channel -> Router -> Session Manager -> Backend Adapter`
- 可扩展路由层对应未来多 Agent 能力：
  - `Window / Channel -> Binding -> Agent Registry -> Router -> Session Manager -> Backend Adapter`

说明：
- 前者先解决“把请求执行起来”。
- 后者在不破坏前者的前提下，补充“该请求应该用什么运行模板”。

## 执行原则
- 同一 Session 串行执行，避免并发写入同一上下文。
- 不同 Session 可以并发执行。
- 新建 Session 优先解决上下文隔离，不隐式复制旧会话状态。
- Agent 只提供默认值；Session 运行后可拥有自己的状态与覆盖项。

## 接口建议

### Router
- `resolve_agent(context) -> agent_id`
- `resolve_session(context) -> session_id`
- `create_session(context, agent_id) -> session`

### Session Manager
- `create(window_id, agent_id) -> session`
- `bind_window(window_id, session_id) -> None`
- `lock(session_id) -> lock_token`
- `unlock(session_id, lock_token) -> None`

### Agent Registry
- `get(agent_id) -> agent`
- `list_enabled() -> agent[]`
- `match_binding(context) -> agent_id | null`

### Backend Adapter
- `create_session(session, agent, profile) -> backend_session_id`
- `run_turn(session, prompt, options) -> result`
- `resume_session(session) -> result`

## 实施顺序

### M0：只做多 Session
- 一个窗口绑定一个 Session。
- 支持新建、恢复、列出 Session。
- 数据模型中预留 `agent_id`，但先不暴露 Agent 管理。

### M1：引入轻量 Agent
- 增加 `Agent Registry` 配置。
- Session 创建时允许带 `agent_id`。
- 支持按窗口默认 Agent 创建新会话。

### M2：增加 Binding
- 支持入口到 Agent 的自动路由。
- 支持频道、线程、用户级默认 Agent。

### M3：再考虑高级能力
- 更细粒度权限。
- 后台任务。
- 统一投递层。
- 更多入口类型。

## 不建议在当前阶段做的内容
- 常驻网关服务作为前置必需条件。
- 重型插件系统。
- 复杂的多节点执行。
- 会话与 Agent 强绑定到无法切换。
- 为了多 Agent 而重写现有会话模型。

## 开发建议
- 先把 `Session` 作为一等公民实现完整。
- `Agent` 先做成“配置模板”，不要做成复杂生命周期对象。
- UI 层优先暴露“新建会话 / 切换会话”，而不是先暴露“管理 Agent”。
- 数据层提前预留 `agent_id`、`profile_id`、`backend_session_id`，避免后续迁移成本。

## 验收标准
- 用户可在多个窗口中同时维护多个独立 Session。
- 每个 Session 可独立新建、恢复和继续执行。
- 引入 Agent 后，不影响现有 Session 行为。
- 同一 Agent 下可同时创建多个 Session。
- 不同 Agent 可拥有不同默认工作目录与运行配置。
