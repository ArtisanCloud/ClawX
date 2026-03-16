# 契约：IRC 网关接入

## 目的
定义 IRC 网关接入 在 ClawX 的接入协议、安全校验、归一化与错误语义，保证跨渠道一致性。

## 1. 接入模式
- 推荐模式：`socket 长连接`。
- 渠道实例必须支持 `enabled/defaultAgent/instances` 最小配置集合。
- 入站事件必须进入统一 Router，不得分叉控制命令语义。

## 2. 安全契约
- 核心安全要求：连接保活 + nick/channel 权限控制。
- 未通过鉴权/验签/权限检查时返回 `403`，并记录结构化日志。
- 重放请求必须按幂等策略拒绝并标记 `duplicate=true`。

## 3. 消息归一化契约
最小映射字段：
- `channel`：渠道标识
- `instance`：实例 ID
- `conversation_id`：稳定会话标识
- `user_id`：发送者标识
- `window_id`：窗口标识（兼容模式可回落）
- `text`：文本正文（无正文可为空）

## 4. 控制语义契约
- 必须支持统一控制命令：`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`。
- 控制命令优先级必须高于普通任务与 skill 路由。
- 渠道特有 metadata 不得改变命令语义。

## 5. 错误语义
- Method 非法：`405`
- 鉴权/签名失败：`403`
- 请求体格式错误：`400`
- 入站接收成功但下游执行失败：返回接收成功语义，错误落日志。

## 6. 可观测性与去重
- 幂等键：`channel|instance|event_id`。
- 日志字段至少包含：`channel`、`instance`、`event_id`、`intent.kind`、`duration_ms`。
- 适配器失败应采用指数退避重试（初始 2s，最大 60s），不得导致主进程退出。
