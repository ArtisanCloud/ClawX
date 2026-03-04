# 数据模型：第一阶段基座能力

## 1. 会话（Session）

### 作用
表示一个受控执行上下文，承载用户在某个渠道会话中的连续工作状态。

### 字段
- `id`: 系统内部会话标识。
- `backend`: 当前会话绑定的执行后端类型。
- `backend_session_id`: 后端原生会话标识，用于继续执行或恢复会话。
- `window_id`: 预留字段，当前阶段可为空，用于后续多窗口扩展。
- `agent_id`: 预留字段，当前阶段可为空，用于后续轻量多 Agent 扩展。
- `conversation_id`: 渠道侧唯一对话标识。
- `cwd`: 当前会话执行工作目录。
- `status`: 当前状态，取值为 `idle`、`running`、`error`。
- `lock_token`: 当前执行锁标识，无锁时为空。
- `last_used_at`: 最近一次使用时间。

### 约束
- 同一时间只能有一个活动执行持有 `lock_token`。
- `backend_session_id` 一旦从后端获得，必须持久保存在会话元数据中。
- `cwd` 必须位于允许工作目录边界内。

### 状态流转
- `idle -> running`: 开始执行。
- `running -> idle`: 执行成功完成或取消后安全退出。
- `running -> error`: 执行异常结束。
- `error -> idle`: 明确恢复后可回到可用状态。

## 2. 执行记录（Execution Run）

### 作用
表示某个会话中的一次实际执行，用于排障、审计摘要和运维观察。

### 字段
- `id`: 执行记录标识。
- `session_id`: 关联会话。
- `request_summary`: 本次请求摘要。
- `start_at`: 开始时间。
- `end_at`: 结束时间。
- `duration_ms`: 执行时长。
- `result_state`: 结果状态，取值为 `success`、`failed`、`timeout`、`cancelled`、`partial_delivery`。
- `delivery_attempts`: 输出送达尝试次数。
- `failure_reason`: 失败原因摘要，可为空。

### 约束
- 一个执行记录只能归属于一个会话。
- 同一会话在 `running` 状态时不得创建第二个活动执行记录。

## 3. 渠道上下文（Channel Context）

### 作用
表示请求来自哪个渠道、哪个用户、哪个私聊或线程上下文，用于会话映射和权限判断。

### 字段
- `channel`: 渠道类型，例如 Discord 或 Telegram。
- `user_id`: 发起用户标识。
- `guild_id`: 群组或服务器标识，可为空。
- `thread_id`: 线程或 Topic 标识，可为空。
- `conversation_id`: 统一生成后的内部对话标识。
- `is_direct_message`: 是否为私聊。
- `is_allowed`: 当前上下文是否被允许触发执行。

### 约束
- `conversation_id` 必须能唯一表示私聊和线程隔离。
- 不允许的上下文不得进入执行阶段。

## 4. 输出片段（Output Segment）

### 作用
表示一段可发送到渠道的执行输出子片段。

### 字段
- `session_id`: 归属会话。
- `sequence`: 顺序编号，从 1 开始递增。
- `content`: 用户可见内容。
- `is_final`: 是否为最后一段。
- `delivery_state`: 送达状态，取值为 `pending`、`sent`、`failed`。

### 约束
- 同一次执行中的输出片段必须按 `sequence` 递增送达。
- 单个输出片段长度必须符合渠道上限。

## 5. 配置快照（Config Snapshot）

### 作用
表示当前进程启动后生效的关键配置集合，用于运行时访问和错误诊断。

### 字段
- `allowed_roots`: 允许执行的工作目录列表。
- `default_cwd`: 默认工作目录。
- `timeout_seconds`: 单次执行超时配置。
- `discord_enabled`: Discord 是否启用。
- `telegram_enabled`: Telegram 是否启用。
- `health_probe_enabled`: 健康探针是否启用。

### 约束
- 会话执行不得突破 `allowed_roots`。
- 渠道未启用时不得接受该渠道请求。
