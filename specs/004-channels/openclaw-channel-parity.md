# OpenClaw 渠道对齐矩阵（Phase 4 扩展）

## 目的
- 对齐 OpenClaw 线上渠道能力清单，明确 ClawX 当前缺口。
- 把“未实现渠道”转成可执行的技术规范与分波次实现计划。

## 参考基线
- OpenClaw 官方渠道总览：`https://docs.openclaw.ai/channels`
- 本机 OpenClaw 安装（`~/.openclaw`）命令补全清单中的可选渠道枚举。

> 注：OpenClaw 文档中有“plugin, installed separately”标记的渠道，表示该渠道由插件提供而非核心内建。

## 渠道对齐矩阵（2026-03-11）

| 渠道 | OpenClaw 状态 | ClawX 当前状态 | 对齐波次 |
| --- | --- | --- | --- |
| Discord | 内建 | 已实现 | 已完成 |
| Telegram | 内建 | 已实现（polling/webhook） | 已完成 |
| Feishu/Lark | 插件 | 规划中（Phase 4） | Wave 1 |
| WeCom（企业微信） | 非 OpenClaw 主清单项 | 规划中（Phase 4） | Wave 1（ClawX 特化） |
| Slack | 内建 | 未实现 | Wave 2 |
| WhatsApp | 内建 | 未实现 | Wave 2 |
| Signal | 内建 | 未实现 | Wave 2 |
| Google Chat | 内建 | 未实现 | Wave 2 |
| IRC | 内建 | 未实现 | Wave 2 |
| Matrix | 插件 | 未实现 | Wave 3 |
| Mattermost | 插件 | 未实现 | Wave 3 |
| Microsoft Teams | 插件 | 未实现 | Wave 3 |
| Nextcloud Talk | 插件 | 未实现 | Wave 3 |
| LINE | 插件 | 未实现 | Wave 3 |
| Nostr | 插件 | 未实现 | Wave 3 |
| Synology Chat | 插件 | 未实现 | Wave 3 |
| Twitch | 插件 | 未实现 | Wave 3 |
| Zalo | 插件 | 未实现 | Wave 3 |
| Zalo Personal | 插件 | 未实现 | Wave 3 |
| BlueBubbles（iMessage 推荐） | 内建（推荐路径） | 未实现 | Wave 4（可选） |
| iMessage legacy | legacy | 未实现 | Wave 4（不优先） |
| Tlon | 插件 | 未实现 | Wave 4（可选） |
| WebChat | 内建（Gateway UI） | 未实现 | Wave 4（可选） |

## 渠道实现建议（逐渠道落地）

> 说明：下表用于“同步到技术规范”的最小落地指引。每个渠道进入实现前，需先补契约文档并挂到 `specs/004-channels/contracts/`。

| 渠道 | 建议接入模式 | 安全/协议重点 | 技术规范落点 |
| --- | --- | --- | --- |
| Slack | Events API + webhook | `X-Slack-Signature` + 时间戳重放保护 | `contracts/slack-event-contract.md` |
| WhatsApp | Cloud API webhook | Meta 签名验证 + 回调重试幂等 | `contracts/whatsapp-event-contract.md` |
| Signal | CLI bridge / service bridge | 本地进程隔离 + 凭据文件保护 | `contracts/signal-bridge-contract.md` |
| Google Chat | webhook / app event | JWT/签名校验 + 空间/线程映射 | `contracts/googlechat-event-contract.md` |
| IRC | socket 连接 | 连接保活 + nick/channel 权限控制 | `contracts/irc-gateway-contract.md` |
| Matrix | plugin bridge | homeserver token + event 去重 | `contracts/matrix-event-contract.md` |
| Mattermost | plugin / webhook | token 验证 + slash 命令映射 | `contracts/mattermost-event-contract.md` |
| Microsoft Teams | Bot Framework webhook | AAD/JWT 校验 + tenant 隔离 | `contracts/msteams-event-contract.md` |
| Nextcloud Talk | plugin bridge | app secret 校验 + room 映射 | `contracts/nextcloud-talk-contract.md` |
| LINE | Messaging API webhook | `X-Line-Signature` 校验 | `contracts/line-event-contract.md` |
| Nostr | relay subscription | key 管理 + event id 幂等 | `contracts/nostr-relay-contract.md` |
| Synology Chat | webhook / bot API | token 验证 + 重放防护 | `contracts/synology-chat-contract.md` |
| Twitch | EventSub webhook | challenge + HMAC 签名校验 | `contracts/twitch-eventsub-contract.md` |
| Zalo | webhook | OA 签名与回调合法性校验 | `contracts/zalo-event-contract.md` |
| Zalo Personal | plugin bridge | 用户态凭据隔离 + 速率限制 | `contracts/zalouser-bridge-contract.md` |
| BlueBubbles | bridge API | bridge token + 消息映射一致性 | `contracts/bluebubbles-bridge-contract.md` |
| iMessage legacy | local bridge | 本地权限约束 + 进程稳定性 | `contracts/imessage-legacy-contract.md` |
| Tlon | plugin bridge | 插件鉴权与路由隔离 | `contracts/tlon-bridge-contract.md` |
| WebChat | 内置网关入口 | 会话 token + CSRF/XSS 边界 | `contracts/webchat-gateway-contract.md` |

## 技术规范同步（统一实现模板）

所有新增渠道必须遵循以下统一规范：

1. **接入层**
- 新建 `internal/interfaces/chat/<channel>/adapter.go`
- 实现统一输入归一化到 `chatiface.Message`
- 输出复用现有 `OutputDelivery`，避免渠道分叉实现

2. **启动编排**
- 在 `cmd/clawx/main.go` 注册渠道实例与生命周期
- 失败采用“通道级重试”，不得导致主进程退出

3. **配置模型**
- 在 `internal/infrastructure/config/config.go` 增加渠道配置结构
- 支持 `enabled/defaultAgent/instances` 最小字段集
- `clawx config channel <name>` 支持增量配置，不覆盖其他渠道

4. **控制语义**
- 必须支持统一命令：`/new /resume /list /current /switch /cancel`
- 控制命令优先级高于普通任务/skill

5. **安全要求**
- webhook 型渠道必须先验签再入路由
- socket/token 型渠道必须在适配层做凭据校验与最小权限约束
- 失败、重放、验签异常需要结构化日志

6. **测试要求**
- 每渠道至少：
  - 1 个 adapter 单元测试（签名/解析/错误）
  - 1 个控制命令集成测试
  - 1 个契约测试（统一控制语义）

## 分波次实施建议

### Wave 1（当前 Phase 4 必做）
- Feishu + WeCom
- 目标：企业场景可用、签名安全可控、与 Telegram/Discord 命令语义一致

### Wave 2（高价值通用渠道）
- Slack + WhatsApp + Signal + Google Chat + IRC
- 目标：覆盖高频团队协作与即时通讯渠道

### Wave 3（插件类/企业扩展）
- Matrix、Mattermost、Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo/Zalo Personal
- 目标：通过插件化适配策略降低主仓维护压力

### Wave 4（可选渠道）
- BlueBubbles、iMessage legacy、Tlon、WebChat
- 目标：按业务需求驱动，默认不阻塞主线发布

## 发布门禁（对齐版）

- Wave 1 完成后才可宣称“企业渠道对齐基线”。
- Wave 2 完成后才可宣称“主流渠道基础对齐”。
- Wave 3/4 以插件化与业务需求驱动，不作为 Phase 4 阻塞项。
