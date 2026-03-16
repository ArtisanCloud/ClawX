# Telegram Bot 创建与接入

## 目标
- 用一份文档完成：创建 Bot、引导式增量配置、启动服务、最小验收。
- 覆盖 Telegram 双模式：`polling` 与 `webhook`。

## 1. 创建 Telegram Bot
1. 在 Telegram 找到 `@BotFather`。
2. 发送 `/newbot`。
3. 记录两项信息：
   - Bot Token
   - Bot Username

## 2. 配置（推荐：引导式增量）

直接运行：

```bash
go run ./cmd/clawx config channel telegram
```

说明：
- 这是增量配置，不会重建整份配置文件。
- 回车可保持原值。
- 你只需要跟着提示填，不需要一个个手敲 `config set`。

可选检查（仅在你不确定配置文件位置时）：

```bash
go run ./cmd/clawx config path
```

当提示 `Telegram mode` 时：
- 选择 `Polling`：只需填写 `pollingSeconds`，不依赖公网回调。
- 选择 `Webhook`：填写 `webhookUrl`、`webhookPath`、`webhookSecret`。

Webhook 关键约束：
- `webhookUrl` 必须是 Telegram 可访问的公网 HTTPS 地址。
- `webhookUrl` 里的 path 必须和 `webhookPath` 一致。
- `webhookPath` 必须以 `/` 开头（例如 `/webhooks/telegram`）。

### webhook 到底在哪配？

- 不是在 Telegram 客户端里配。
- 是在 ClawX 配置里填写 `webhookUrl/webhookPath/webhookSecret`。
- ClawX 启动后会调用 Telegram Bot API `setWebhook` 自动注册。
- 注册成功后，Telegram 才会把消息回调到你的 `webhookUrl`。

可选核对：

```bash
go run ./cmd/clawx config get channels.telegram
```

## 3. 启动服务

```bash
go run ./cmd/clawx serve
```

Polling 模式下，预期日志包含：
- `telegram adapter started: instance=... mode=polling`

Webhook 模式下，预期日志包含：
- `telegram webhook route registered`
- `telegram webhook configured`
- `telegram adapter started: instance=... mode=webhook`

可再用官方接口确认 Telegram 侧是否已注册：

```bash
export TG_BOT_TOKEN=<YOUR_BOT_TOKEN>
curl -s "https://api.telegram.org/bot$TG_BOT_TOKEN/getWebhookInfo"
```

## 4. 模式验证

### 4.1 Polling 最小验证

1. 配置模式为 `polling` 后启动服务。
2. 在 Telegram 私聊发送 `/new`、`hello`、`/list`。

预期：
- 消息可直接处理，不依赖 webhook 回调。
- 服务日志持续出现 `telegram adapter started ... mode=polling`。

### 4.2 Webhook 配通验证（必须做）

按下面 3 组检查，全部通过才算 webhook 配通。

### 4.2.1 入口连通检查（Nginx + HTTPS）

```bash
curl --noproxy '*' -i https://<your-domain>/webhooks/telegram
```

预期：
- 返回 `405 Method Not Allowed`（说明路由已挂载，只允许 POST）。

可选再测一次 POST：

```bash
curl --noproxy '*' -i -X POST https://<your-domain>/webhooks/telegram \
  -H 'Content-Type: application/json' \
  -d '{}'
```

预期：
- 返回 `200 OK`
- body 为 `{"ok":true}`

### 4.2.2 Telegram 侧注册检查（setWebhook 是否生效）

```bash
export BOT_TOKEN=<YOUR_BOT_TOKEN>
curl -s "https://api.telegram.org/bot$BOT_TOKEN/getWebhookInfo"
```

预期返回里至少满足：
- `ok=true`
- `result.url` 等于 `https://<your-domain>/webhooks/telegram`

### 4.2.3 端到端消息检查

在 Telegram 私聊 bot 发送：
1. `/new`
2. `hello`
3. `/list`

预期：
- `/new` 返回新会话 `sess-...`
- `hello` 有正常回复
- `/list` 返回当前会话列表

## 5. 在 Telegram 里发给谁
1. 在 Telegram 搜索 `@<你的bot用户名>`。
2. 打开 bot 私聊窗口，点击 `Start`（或发送 `/start`）。
3. 在这个私聊窗口里发送 `/new`、`hello`、`/list`。

说明：
- 你是发给自己创建的 bot，不是发给 `@BotFather`。
- 这个私聊窗口就是单窗口测试入口。

## 6. 常见问题（Webhook）
- `404`：通常是 ClawX 未以 webhook 模式启动，或 `webhookPath` 不一致。
- `401`：回调被 Basic Auth 挡住，需确保 webhook 路径关闭认证。
- `getWebhookInfo.url` 为空：`setWebhook` 未成功，检查服务日志和公网 HTTPS 可达性。
- 证书不匹配：证书 SAN 必须包含你的 webhook 域名。

## 7. 多窗口怎么理解（Telegram）
- 同一个私聊窗口 = 一个窗口上下文。
- 要模拟窗口 A/B，可用不同聊天上下文（例如 Telegram 私聊 + Discord 私聊）。

## 附录：逐项命令（仅自动化脚本场景）
只有在你做脚本化部署时，才建议用 `config set`：

```bash
go run ./cmd/clawx config set channels.telegram.mode webhook
go run ./cmd/clawx config set channels.telegram.webhookUrl https://<your-domain>/webhooks/telegram
go run ./cmd/clawx config set channels.telegram.webhookPath /webhooks/telegram
go run ./cmd/clawx config set channels.telegram.webhookSecret <RANDOM_SECRET>
```
