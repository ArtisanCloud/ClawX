# 架构设计（V1）

## 组件与职责
- **Channel Adapter (Discord/Telegram)**：接收事件、指令解析、@bot/Thread 识别、规范化消息结构。
- **Router**：按 `conversation_id` 路由到对应 Session；管理 /new /resume /list /cancel。
- **Session Manager**：创建/查询/切换 session，维护锁、状态、最近上下文与工作目录。
- **Codex CLI Adapter**：启动/恢复 Codex 进程，执行 `codex exec`，支持超时、取消、退出码与 stderr 回传。
- **Output Streamer**：将 CLI stdout/stderr 拆分成分段（或透传流式），保证顺序与最大长度；追加代码块包装。
- **Persistence (轻量)**：本地文件优先（`sessions.json + *.jsonl`）+ 内存缓存；数据库仅作为后续可选增强。
- **Config & Logging**：加载环境/文件配置；结构化日志；错误分级回显。

## 时序（示例：Discord @bot 修复函数）
1. Channel Adapter 收到消息，提取 channel/thread/user，生成 conversation_id。
2. Router 获取/创建 session，申请锁；若被占用则回显“正在执行”。
3. Codex CLI Adapter 以会话工作目录执行 `codex exec "<user msg>"`，附加超时。
4. Output Streamer 监听 stdout/stderr，分段发送回 Channel Adapter；附带处理中提示。
5. 结束后释放锁，记录状态/摘要；若异常则回显错误并保留诊断信息。

## 数据模型（简化）
- Session: id, channel, thread, user, cwd, status (idle/running), last_command, last_updated, lock_token.
- Execution: session_id, command, start_at, end_at, exit_code, output_chunks[], error?

## 运行约束
- 单次执行最长 10 分钟；超时可取消。
- 目标并发 5-20 个 session；同一 session 串行。
- 输出分段长度受渠道限制（Discord 2000 字符，Telegram 4096），优先整块代码块分割。

## 依赖与接口
- 依赖本地 Codex CLI 可用；可配置 CLI 路径与默认工作目录。
- Channel Adapter 需提供统一接口：`send_text(session, chunk, is_final=false)`；错误场景 `send_error(session, message)`。

## 演进预留
- Storage 默认为本地文件，后续可增加 SQLite 索引或外部数据库适配层。
- Output Streamer 可切换真实流式（若 CLI 支持）。
- Channel Adapter 可扩展新的渠道（Slack/PowerX），保持标准接口。
