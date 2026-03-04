# 核心概念（V1）

- **Channel**：消息来源，当前仅 Discord Bot、Telegram Bot。
- **Conversation**：渠道维度的对话唯一键；群聊 thread 单独成会话，私聊为独立会话。
- **Session (Codex Session)**：对话与 Codex CLI 的映射实例，包含运行状态、工作目录、锁与最近上下文。
- **Session Lock**：同一 session 内互斥执行，避免并发冲突；包含超时与取消。
- **Command**：来自用户的自然语言或控制指令（/new /list /resume /cancel）。
- **Job/Execution**：一次 codex exec 调用的生命周期，含启动、流式/分段输出、结束、错误回显。
- **Output Chunk**：CLI 输出的分段（模拟流式或真实流式），保持顺序。
- **Conversation Id 规则**：`{channel}:{guild?}:{thread?}:{user}`，确保群线程/私聊隔离。
- **Workspace**：Codex 执行的工作目录，可由指令/默认配置指定。
