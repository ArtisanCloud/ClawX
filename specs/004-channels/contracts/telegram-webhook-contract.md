# 契约：Telegram polling/webhook 交付行为

## 目的
定义 Telegram 两种接入模式的最小行为、鉴权与错误处理要求。

## 1. 模式契约

- `mode=polling`: 通过 `getUpdates` 拉取消息。
- `mode=webhook`: 通过 HTTP 回调接收更新。

要求：
- 两种模式进入同一归一化与路由流程。
- 切换模式不改变会话与控制命令语义。

## 2. webhook 请求要求

- HTTP Method: `POST`
- Path: 配置的 `webhookPath`
- Header: `X-Telegram-Bot-Api-Secret-Token`（若配置 `webhookSecret` 则必须校验）
- Body: Telegram Update JSON

拒绝条件：
- Method 非 `POST`
- JSON 解析失败
- secret token 不匹配

## 3. setWebhook 行为

- 当 `mode=webhook` 且配置了 `webhookUrl` 时，启动阶段必须尝试 `setWebhook`。
- 注册失败应记录错误并进入退避重试，不得导致主进程退出。

## 4. 错误与重试

- 网络错误、代理错误、超时均视为适配器错误。
- 适配器应指数退避重试，并保持其他渠道运行。

## 5. 审计字段

每次入站至少记录：

```text
channel=telegram
instance_id
mode
event_id(update_id)
intent.kind
duration_ms
```
