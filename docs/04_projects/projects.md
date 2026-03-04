# 里程碑规划（V1）

| 里程碑 | 范围 | 验收标准 |
| --- | --- | --- |
| M0 基座 | 最小可运行骨架；内存 Session 管理；本地 Codex CLI 直连；Discord Bot 单渠道；/new /resume /list /cancel | 在 Discord 私聊成功触发 codex exec 并拿到分段输出；会话互斥；错误回显 |
| M1 双渠道 | 接入 Telegram；渠道抽象统一；Thread 映射 session；基础配置文件 | Discord/Telegram 均可用；群聊 @bot + thread 独立 session；配置可切换工作目录 |
| M2 稳定性 | 超时/取消完善；日志/监控基线；输出切块健壮；重试策略 | 长输出不截断且按序；超时可取消；日志含 session id；基本健康检查 |
| M3 运行化 | 部署脚本；env 模板；最小监控指标；故障手册 | 可在 Linux 服务器一键部署；有操作手册与告警基线 |
