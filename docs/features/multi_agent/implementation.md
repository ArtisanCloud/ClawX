# 多会话与多 Agent 参考实现方案（V1.5）

## 文档定位
- 本文档是“开发落地方案”，用于指导实现顺序、模块拆分和数据结构。
- 本文档配套的架构说明见：
  - [architecture.md](./architecture.md)
- 本文档不讨论外部参考项目，只定义 ClawX 自身的实现路径。

## 目标
- 在不破坏现有 V1 会话模型的前提下，引入可扩展的多 Agent 能力。
- 优先完成“多窗口对应多会话”。
- 将 `Agent` 作为运行模板引入，而不是作为必须暴露给用户的第一层概念。

## 实现总策略
- 采用“轻执行层 + 可扩展路由层”的混合方案。
- 先实现一条最短执行链路，确保请求可以稳定完成。
- 再在这条链路外增加 `Agent`、`Binding`、默认路由等能力。

### 轻执行层（先落地）
- 请求进入后，直接进入：
  - `Router -> Session Manager -> Backend Adapter`
- `Backend Adapter` 直接完成一次后端调用。
- 本地只保存 Session 元数据和后端会话 ID 映射。

适合当前阶段的原因：
- 开发成本最低。
- 出错面最小。
- 便于先完成多窗口、多会话。

### 可扩展路由层（后补齐）
- 在轻执行层前增加：
  - `Binding -> Agent Registry`
- 用于决定“新 Session 应该继承哪套运行模板”。
- 不改变底层执行方式，只改变参数来源和路由决策。

适合当前阶段的原因：
- 可以在不重写执行链路的前提下扩展多 Agent。
- 保持现有会话模型稳定。

## 交付范围

### 本阶段必须完成
- 多窗口绑定多 Session。
- Session 的新建、恢复、列出、切换。
- Session 与后端会话 ID 的映射。
- 数据模型中预留 `agent_id`。
- 轻量 `Agent Registry`。
- Session 创建时支持指定 `agent_id`。

### 本阶段可选完成
- Window 级默认 Agent。
- 入口到 Agent 的静态 Binding。
- Agent 级默认工作目录与默认后端参数。

### 本阶段不做
- 重型网关服务。
- 复杂插件系统。
- 后台任务调度。
- 多节点执行。
- 高级权限编排。

## 模块拆分

### 分层要求
- `Router / Session Manager / Backend Adapter` 必须先独立。
- `Agent Registry / Binding` 可以后置，但接口需要预留。
- 不把“创建会话”和“执行后端”写死在同一个模块里。

### 1. Window Context
职责：
- 表示一个前端窗口或一个显式交互容器。
- 保存当前绑定的 Session。

最小字段：
- `window_id`
- `current_session_id`
- `preferred_agent_id`（可空）

### 2. Session Manager
职责：
- 创建、查询、恢复、切换 Session。
- 管理会话锁。
- 保存后端会话映射。

最小接口：
- `create_session(window_id, agent_id=None)`
- `get_session(session_id)`
- `list_sessions(window_id=None, agent_id=None)`
- `bind_window(window_id, session_id)`
- `lock_session(session_id)`
- `unlock_session(session_id, lock_token)`

### 3. Agent Registry
职责：
- 读取和管理 Agent 配置。
- 返回某个 Agent 的默认运行参数。
- 根据 Binding 或窗口偏好返回默认 Agent。

最小接口：
- `get_agent(agent_id)`
- `list_agents()`
- `get_default_agent()`
- `resolve_agent_for_context(context)`

### 4. Router
职责：
- 接收来自 UI 或渠道的请求。
- 判断是新建会话、恢复会话还是普通消息。
- 决定本次执行使用哪个 Session、哪个 Agent。

最小接口：
- `handle_new_session(context)`
- `handle_resume_session(context, session_id)`
- `handle_message(context, prompt)`

分阶段要求：
- V1 可以只依赖默认 Agent 或窗口当前 Agent。
- V1.5 再接入 `Binding` 规则。

### 5. Backend Adapter
职责：
- 屏蔽后端差异。
- 执行“新建会话”“恢复会话”“单次对话”。
- 回写后端原生会话 ID。

最小接口：
- `create_backend_session(session, agent, profile)`
- `run_turn(session, prompt, options)`
- `list_backend_sessions(profile=None)`

实现要求：
- `Backend Adapter` 必须保持“直接执行”的简单模式。
- 当前阶段不要求引入额外常驻调度层。
- 后续若增加统一调度，也应复用该接口，而不是替换 Session 主流程。

## 数据结构

### Session
```json
{
  "id": "sess_xxx",
  "window_id": "win_xxx",
  "agent_id": "default",
  "backend": "codex",
  "backend_session_id": "uuid-or-native-id",
  "cwd": "/workspace/project",
  "status": "idle",
  "lock_token": null,
  "last_used_at": "2026-03-03T00:00:00Z"
}
```

实现要求：
- `agent_id` 允许为空，但建议从创建开始就写入。
- `backend_session_id` 创建后必须持久化到 Session 元数据。

### Agent
```json
{
  "id": "default",
  "name": "Default Agent",
  "workspace": "/workspace/project",
  "backend": "codex",
  "profile_id": "default",
  "model": "",
  "system_prompt": "",
  "tools_policy": "standard",
  "enabled": true
}
```

实现要求：
- `workspace` 是 Session 默认工作目录来源。
- `backend` 决定使用哪个 Backend Adapter。
- `profile_id` 用于读取后端专用配置。

### Binding
```json
{
  "id": "bind_xxx",
  "scope_type": "window",
  "scope_key": "win_xxx",
  "agent_id": "default",
  "priority": 100,
  "enabled": true
}
```

实现要求：
- V1.5 先支持 `window`。
- 后续扩展 `channel`、`thread`、`user`。

## 创建流程

### 新建窗口
1. 前端生成 `window_id`。
2. 创建 `Window Context`。
3. 若无显式选择，使用默认 Agent。
4. 不强制立即创建 Session，可延迟到首条消息。

说明：
- 这是轻执行层的关键设计：窗口先存在，Session 可以懒创建。
- 这样可以避免一打开窗口就产生无效会话。

### 首条消息到达
1. Router 检查 `window_id` 是否已有 `current_session_id`。
2. 若无，则调用 `create_session(window_id, resolved_agent_id)`。
3. Session Manager 创建本地 Session。
4. Backend Adapter 创建后端会话。
5. 回写 `backend_session_id`。
6. 将 Session 绑定到当前窗口。

说明：
- 这里体现“轻执行层 + 可扩展路由层”的结合：
  - 先由 `Agent` 提供默认参数；
  - 再由 `Backend Adapter` 直接落地执行。

### 切换会话
1. 用户选择已有 Session。
2. Router 校验该 Session 可用。
3. Session Manager 更新当前窗口绑定。
4. 后续消息全部路由到该 Session。

## 执行流程

### 普通消息
1. Router 根据 `window_id` 获取当前 Session。
2. Session Manager 加锁。
3. 读取 Session 对应 Agent。
4. 合并运行参数：
   - Session 级覆盖
   - Agent 默认值
   - Profile 默认值
5. Backend Adapter 执行本轮对话。
6. 保存状态与时间戳。
7. 解锁。

实现要点：
- 本阶段仍然是“请求到达即直接执行”。
- 不增加额外排队中台，除非后续并发和投递复杂度明显提升。

### 新建会话指令
1. Router 解析为控制指令。
2. 关闭当前“继续使用当前会话”的绑定关系。
3. 创建新的 Session。
4. 将新 Session 绑定到当前窗口。

### 恢复会话指令
1. Router 校验目标 Session。
2. 切换窗口绑定。
3. 不重建后端会话，直接继续使用已保存的 `backend_session_id`。

## 配置建议

### Agent 配置文件
建议新增独立配置段：

```yaml
agents:
  default_agent: default
  list:
    - id: default
      name: Default Agent
      workspace: /workspace/project
      backend: codex
      profile_id: default
      tools_policy: standard
```

### Binding 配置文件
可选配置段：

```yaml
bindings:
  - scope_type: window
    scope_key: win_default
    agent_id: default
    priority: 100
```

## 推荐文件落位

若按当前仓库结构落地，建议拆分为以下模块：
- `src/core/window_context.*`
- `src/core/session_manager.*`
- `src/core/agent_registry.*`
- `src/core/router.*`
- `src/core/bindings.*`
- `src/adapters/backend/*`
- `src/core/bindings.*`（可延后）

若当前还没有 `src/`，可先在现有核心目录中按同样职责拆文件，重点是职责分离，不强制目录名。

## 开发顺序

### Step 1：补数据模型
- 给 Session 增加 `agent_id`
- 给 Session 增加 `backend_session_id`
- 增加 `window_id`
- 增加 `cwd` 的 Session 级字段

验收：
- 现有单会话流程不受影响。

### Step 2：实现 Window 绑定
- 增加 `Window Context`
- 支持一个窗口绑定一个当前 Session
- 支持切换当前 Session

验收：
- 同一用户可在多个窗口中持有不同 Session。

### Step 3：保持轻执行链路稳定
- 确保 `Router -> Session Manager -> Backend Adapter` 链路独立可运行
- 确保没有 `Agent` 时也能用默认配置执行
- 确保 Session 可独立新建、恢复、继续执行

验收：
- 即使未开启多 Agent，也能完整跑通主流程。

### Step 4：实现轻量 Agent Registry
- 支持读取默认 Agent
- 支持按 `agent_id` 读取配置
- Session 创建时写入 `agent_id`

验收：
- 新 Session 能继承 Agent 默认工作目录与后端。

### Step 5：改造 Router
- 新建会话时先决策 Agent，再创建 Session
- 普通消息时基于 Session 反查 Agent
- 支持控制指令与普通消息分流

验收：
- “新建会话 / 恢复会话 / 发送消息”三条主链路打通。

### Step 6：加入 Binding（可选）
- 先支持 `window -> agent`
- 后续再扩展到 `channel` / `thread`

验收：
- 新窗口可自动命中默认 Agent。

## 测试清单

### 单元测试
- Session 创建时正确写入 `agent_id`
- Session 创建时正确保存 `backend_session_id`
- 同一 Session 锁互斥
- 不同 Session 可并发
- Router 能正确解析“新建/恢复/普通消息”

### 集成测试
- 窗口 A、窗口 B 各自创建独立 Session
- 窗口 A 切换到旧 Session 后继续执行
- 同一 Agent 下创建多个 Session 互不串线
- 不同 Agent 使用不同默认工作目录

### 回归测试
- 没有启用 Agent 时，现有单会话流程仍可用
- 不配置 Binding 时，系统仍能回退到默认 Agent

## 验收标准
- 用户可在多个窗口中独立创建和切换 Session。
- Session 具备稳定的后端会话映射能力。
- 引入 Agent 后，不需要重写现有会话主流程。
- Agent 可以作为模板被多个 Session 复用。
- 路由规则可先从简单默认值开始，再逐步扩展。

## 当前建议
- 先实现到 Step 5。
- Step 6 可以作为下一里程碑。
- 先不要把 Agent 管理做成重 UI；优先把底层数据和路由模型打稳。

## 文档口径说明
- 若在其他文档中描述本方案，应统一表述为：
  - “当前采用轻执行层直连后端”
  - “在其上预留可扩展路由层与 Agent 模板能力”
- 避免把多 Agent 描述成当前阶段的前置条件。
