# 快速启动：第四阶段渠道扩展

## 目标
在保持现有多会话与路由链路稳定的前提下，完成 Telegram webhook、Feishu、WeCom 渠道扩展与增量配置闭环。
并将 OpenClaw 未实现渠道同步到统一技术规范与任务波次（Wave 2~4）。

## 开发前准备

1. 确认当前分支为 `004-channels`。
2. 阅读以下文档：
   - `/home/ubuntu/workspace/SynapseX/docs/plans/phase_4_channels.md`
   - `/home/ubuntu/workspace/SynapseX/specs/004-channels/spec.md`
   - `/home/ubuntu/workspace/SynapseX/specs/004-channels/plan.md`
   - `/home/ubuntu/workspace/SynapseX/specs/004-channels/openclaw-channel-parity.md`
3. 确保本地可运行命令：
   - `go run ./cmd/synapsex`
   - `go test ./...`

## 推荐实现顺序

1. 完成渠道运行时容错基线（通道级重试、不崩主进程）。
2. 先闭环 Telegram 双模式（含 webhook setWebhook 和验签）。
3. 接入 Feishu（challenge + 签名 + 文本消息）。
4. 接入 WeCom（URL 验证 + 签名/解密 + 文本消息）。
5. 实现 `synapsex config channel <name>` 增量配置。
6. 补齐跨渠道契约测试与回归文档。
7. 按 Phase 7 任务补齐 Wave 2~4 未实现渠道实现卡。

## 最小验收步骤（MVP: Telegram）

1. 启动 `mode=polling`，发送 `/new` 和普通文本，确认可用。
2. 切换到 `mode=webhook`，配置 `webhookUrl/webhookPath/webhookSecret`。
3. 重启服务并确认日志含 webhook 注册与路由日志。
4. 发送 `/list`、`/current`、普通文本，确认行为与 polling 一致。
5. 人为制造 Telegram 网络失败，确认仅 Telegram 适配器重试，服务不退出。

## 全渠道验收步骤

1. Feishu challenge 请求可通过。
2. Feishu 普通消息与控制命令可达。
3. WeCom URL 验证可通过。
4. WeCom 普通消息与控制命令可达。
5. 在四渠道分别执行 `/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`，确认语义一致。

## 自动化回归

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

## 指标门禁（SC-001 ~ SC-006）

- 样本窗口：最近 7 天。
- 每项样本：不少于 200。
- 关键阈值：
  - Telegram 双模式链路成功率 >= 99%
  - 单渠道故障下主进程存活率 = 100%
  - 控制命令跨渠道回归通过率 = 100%
  - 安全异常拦截率 = 100%

## 完成检查

- Telegram webhook 与 polling 均可用。
- Feishu 与 WeCom 接入链路可用。
- 增量配置不会覆盖其他渠道。
- `go test ./...` 通过。
