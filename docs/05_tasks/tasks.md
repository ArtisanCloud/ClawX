# 任务看板（V1 背景板）

## Ready
- Discord Bot 事件接入与 @bot 解析
- conversation_id 规则与生成器
- Session Manager（内存版）+ 互斥锁
- Codex CLI Adapter：exec/resume/cancel，超时参数
- Output 分段器：按渠道上限切块，代码块包装
- /new /list /resume /cancel 指令处理
- 配置加载（env + yaml），结构化日志

## In Progress
- Telegram Bot 适配与 thread 映射
- 错误分级回显与重试策略

## Backlog
- 健康检查与最小监控指标
- 持久化 Session/History（可选 redis/sqlite）
- CLI 流式透传模式（取决于 Codex 支持）
- 部署/故障手册完善
