# SynapseX 文档总览（V1）

第一阶段目标：把 Discord / Telegram 消息映射成对 Codex CLI 的远程控制，提供会话管理、流式/分段回传和基础运维能力，先跑通“远程 Codex CLI 控制网关”。

## 范围边界
- 支持：Codex CLI 启动/exec/resume/取消；会话隔离；消息分段；基础 Markdown/代码块；Linux 部署；日志与错误回显。
- 渠道：Discord Bot、Telegram Bot，私聊/群聊/Thread 识别与指令（/new /list /resume）。
- 不做：多模型调度、Claude/Cursor、插件/RAG、权限/审计、多租户、Web 控制台、PowerX 集成。

## 模块视图
- Channel Adapter：接收并规范化渠道事件。
- Session Manager：conversation_id 生成、映射、锁、超时与切换。
- Codex CLI Adapter：启动/恢复/取消 CLI，处理工作目录与超时。
- Message Dispatcher：分段/流式回传，Markdown/代码块格式化。
- Ops 基础：配置、日志、错误处理与健康检查。

## 成功判据
- 在 Discord/Telegram 中可稳定驱动 Codex，正确维护多会话并能处理长输出且 10 分钟内完成单次执行，无严重崩溃。
