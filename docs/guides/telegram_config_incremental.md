# Telegram 增量配置（简版）

## 推荐方式

```bash
go run ./cmd/clawx config channel telegram
```

特点：
- 只修改 Telegram 相关字段。
- 其他渠道（Discord/Feishu/WeCom）配置不会被覆盖。

## 常用字段
- `enabled`
- `mode`：`polling` 或 `webhook`
- `token`
- `botUsername`
- `requireCommandOrMention`
- `pollingSeconds`（polling 模式）
- `webhookUrl` / `webhookPath` / `webhookSecret`（webhook 模式）

## 非交互方式（可选）

```bash
go run ./cmd/clawx config set channels.telegram.enabled true
go run ./cmd/clawx config set channels.telegram.mode webhook
go run ./cmd/clawx config set channels.telegram.webhookUrl https://<domain>/webhooks/telegram
go run ./cmd/clawx config set channels.telegram.webhookPath /webhooks/telegram
```

## 验证配置是否生效

```bash
go run ./cmd/clawx config get channels.telegram
```

## 建议
- Webhook 模式务必使用 HTTPS。
- 若只想改 Telegram，不要执行全量 `clawx config` 覆盖流程。
