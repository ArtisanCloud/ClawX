# 能力清单（V1）

## 必须支持
- Codex CLI：启动、`exec`、`resume`、工作目录指定、超时、取消
- 会话管理：channel/thread/user 唯一键；列出/切换；互斥锁
- 渠道：Discord/Telegram，私聊与群聊 thread；@bot 识别；指令 /new /list /resume /cancel
- 消息：长文本自动分段；代码块/Markdown；顺序保证；处理中提示
- 错误：CLI 失败/超时/并发冲突/取消的回显
- 运行：Linux 部署；配置文件；结构化日志

## 非目标（V1 不做）
- 多模型调度；Claude/Cursor 支持
- 插件系统、RAG、Tool Skill 框架
- 多租户、权限/审计系统
- Web 控制台
- PowerX 集成
