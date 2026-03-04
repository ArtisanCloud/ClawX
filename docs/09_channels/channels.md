# 渠道规范（V1）

## Discord
- 触发方式：私聊直接触发；群聊需 @bot；Thread 映射独立 session。
- 指令：/new /resume /list /cancel；自然语言转 `codex exec`。
- 分段：每条消息 ≤ 2000 字符；优先整段代码块拆分，必要时按行拆。
- 上下文：在回显中带 session id/状态提示（处理中/已取消/错误）。

## Telegram
- 触发方式：私聊直接触发；群聊需 @bot 或命令；可用 Topic/Thread 映射 session（若开启论坛模式）。
- 指令与行为同 Discord；分段上限 4096 字符。

## 通用要求
- 统一的规范化消息结构：`{conversation_id, user_id, text, reply_to?, attachments?}`。
- 渠道 Adapter 提供 `send_text` 与 `send_error`，由 Output Streamer 调用。
- 可靠性：发送失败需重试（最多 N 次）并记录；顺序由 Streamer 保证。
