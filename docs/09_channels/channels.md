# 渠道规范（V1）

## 当前状态（2026-03-11）

- 已实现：Discord、Telegram（polling/webhook）
- Wave 1（基线实现中）：Feishu、WeCom
- Wave 2~4（对齐 OpenClaw 渠道矩阵）：见 `/home/ubuntu/workspace/ClawX/specs/004-channels/openclaw-channel-parity.md`

## Discord
- 目标主路径：Bot Token + Gateway WebSocket。
- 目标触发方式：私聊直接触发；群聊需 @bot；Thread 映射独立 session。
- 当前实现状态：当前分支已提供 Gateway 运行时代码；外部联调依赖真实 Bot Token 与服务器配置。
- 指令：`/new` `/resume` `/list` `/cancel` `/current`；自然语言转后端执行。
- Discord Slash Commands：启动后自动注册上述基础命令（Gateway + Interaction）。
- 支持多 Bot 实例：`channels.discord.instances[]`。
- 支持 Agent 路由：`defaultAgent` + `agentBindings`（实例级优先，channel 级兜底）。
- 分段：每条消息 ≤ 2000 字符；优先整段代码块拆分，必要时按行拆。
- 上下文：在回显中带 session id/状态提示（处理中/已取消/错误）。

## Telegram
- 主路径：Bot API Long Polling（当前实现）或 Webhook（后续可补）。
- 触发方式：私聊直接触发；群聊需 @bot 或命令；可用 Topic/Thread 映射 session（若开启论坛模式）。
- 指令与行为同 Discord；分段上限 4096 字符。
- 支持多 Bot 实例：`channels.telegram.instances[]`。
- 支持 Agent 路由：`defaultAgent` + `agentBindings`（实例级优先，channel 级兜底）。

## 后续扩展渠道（OpenClaw 对齐）

- Wave 2：Slack、WhatsApp、Signal、Google Chat、IRC
- Wave 3：Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal
- Wave 4：BlueBubbles、iMessage legacy、Tlon、WebChat

所有新增渠道必须复用统一控制命令语义与适配器模板：
- 控制命令：`/new` `/resume` `/list` `/current` `/switch` `/cancel`
- 安全要求：先验签再路由，拒绝重放与非法请求
- 容错要求：单渠道失败重试，不得拖垮主进程

## 协议边界
- 平台接入协议不统一：
  - Discord 聊天主模式：WebSocket（Gateway）
  - Telegram 聊天主模式：Long Polling 或 Webhook
- ClawX 自身若提供前端流式接口，可另行支持：
  - WebSocket
  - SSE
- 不应把平台接入协议和 ClawX 自身流式协议混为一层。

## 通用要求
- 统一的规范化消息结构：`{conversation_id, user_id, text, reply_to?, attachments?}`。
- 渠道 Adapter 提供 `send_text` 与 `send_error`，由 Output Streamer 调用。
- 可靠性：发送失败需重试（最多 N 次）并记录；顺序由 Streamer 保证。
- 会话隔离维度：`conversation_id + channel instance + agent_id`，避免多 Bot/多 Agent 串线。
