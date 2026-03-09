# 数据模型：第二阶段多窗口多会话

## 1. 会话（Session）

### 作用
表示一个可持续执行上下文，承载后端会话映射、状态与锁语义。

### 字段
- `id`: 内部会话标识。
- `window_id`: 最近一次绑定的窗口标识（Phase 2 开始真实使用）。
- `agent_id`: 预留字段，当前阶段允许为空。
- `backend`: 后端类型。
- `backend_session_id`: 后端原生会话标识。
- `conversation_id`: 渠道统一对话标识。
- `cwd`: 执行工作目录。
- `status`: `idle | running | error`。
- `lock_token`: 当前执行锁标识，无锁为空。
- `last_used_at`: 最近使用时间。

### 约束
- 同一会话同一时刻仅允许一个有效执行锁。
- `backend_session_id` 一旦获得必须持久化。
- `window_id` 在 `new/resume/switch/continue` 后应保持与当前窗口语义一致。

## 2. 窗口绑定（Window Binding）

### 作用
表示“窗口当前会话指针”的唯一真相，驱动窗口优先路由。

### 字段
- `window_id`: 窗口标识。
- `conversation_id`: 来源对话标识（兼容与排障使用）。
- `current_session_id`: 当前窗口绑定会话。
- `last_used_at`: 窗口级最近使用时间。
- `updated_at`: 最后更新时间。

### 约束
- 一个 `window_id` 在任意时刻只允许一个 `current_session_id`。
- `current_session_id` 必须引用有效且可访问的 `Session`。
- 当入口无显式 `window_id` 时，必须先归一化为兼容值再读写绑定。

## 3. 归一化消息（Normalized Message）

### 作用
作为渠道层与路由层之间的统一输入。

### 字段
- `conversation_id`
- `window_id`（新增）
- `user_id`
- `text`
- `reply_to`（可选）
- `attachments`（可选）
- `channel`
- `context_flags`

### 约束
- `window_id` 可由外部显式传入。
- 若外部缺省，系统必须生成兼容值 `compat:<conversation_id>`。

## 4. 控制命令（Control Command）

### 作用
表达会话控制意图，驱动窗口级会话管理。

### 命令集合
- `/new`
- `/resume <session_id>`
- `/switch <session_id>`
- `/list`
- `/current`
- `/cancel`

### 约束
- 内建命令由 Core Router 优先解析。
- `/switch` 仅更新窗口绑定，不触发执行。
- `/resume` 更新窗口绑定；后续输入继续对应 `backend_session_id`。

## 5. 状态更新规则

### 会话状态流转
- `idle -> running`: 开始执行。
- `running -> idle`: 成功完成或取消。
- `running -> error`: 执行失败。

### 窗口绑定更新触发
- 新建会话成功：窗口绑定到新会话。
- 恢复会话成功：窗口绑定到目标会话。
- 切换会话成功：窗口绑定到目标会话。
- 普通消息继续成功：刷新窗口和会话 `last_used_at`。
