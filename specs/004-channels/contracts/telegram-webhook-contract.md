# 契约：Telegram polling/webhook 交付行为

## 目的
定义 Telegram 双模式在 ClawX 的接入协议、安全约束、错误语义与一致性要求。

## 1. 模式契约
- `mode=polling`：适配器通过 `getUpdates` 拉取消息。
- `mode=webhook`：HTTP 回调接收更新，服务启动时执行 `setWebhook`。
- 两种模式必须进入同一控制命令与会话执行链路。

## 2. webhook 请求契约
- Method：`POST`
- Path：实例级路由（默认 `/webhooks/telegram/<instance-id>`）
- Header：`X-Telegram-Bot-Api-Secret-Token`（配置了 `webhookSecret` 时必须匹配）
- Body：Telegram Update JSON

### 响应
- 合法请求：`200`，`{"ok":true}`
- 非法 Method：`405`
- secret 不匹配：`403`
- JSON 非法：`400`

## 3. setWebhook 契约
- 当 `mode=webhook` 且实例启用时，服务启动后必须尝试注册 webhook。
- 注册失败不得导致主进程退出，必须进入渠道级重试。

## 4. polling 契约
- 轮询必须保留 `update_id` 游标，避免重复消费。
- 拉取失败必须进入渠道级重试，不影响其他渠道。

## 5. 消息归一化契约
- 文本消息映射到统一消息模型后进入 Router。
- 控制命令集合：`/new`、`/resume`、`/switch`、`/list`、`/current`、`/cancel`。
- Telegram 特有字段（chat/thread/reply）只作为 metadata，不得改变控制语义。

## 6. 重试与隔离
- 适配器失败必须记录结构化日志：`channel`、`instance`、`retry_count`、`last_error`。
- 退避策略：指数退避，初始 2s，最大 60s。
- 任何 Telegram 故障不得触发主进程退出。

## 7. 审计字段
入站路由与执行日志至少包含：
- `channel=telegram`
- `instance`
- `conversation_id`
- `intent.kind`
- `duration_ms`
