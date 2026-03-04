# 运维与运行手册（V1）

## 部署假设
- 环境：Linux，已安装 Node/Python（依据 Codex CLI 要求），可访问 Codex。
- 配置：env 文件或 yaml，含 bot tokens、Codex 路径、默认工作目录、超时时间。
- 进程管理：建议 systemd/pm2/supervisor 之一，保持单实例。

## 运行与监控
- 健康检查：进程存活；与 Codex CLI 的探针执行（短命令）。
- 指标基线：执行次数、成功/失败/超时、平均时长、在途执行数；输出分段失败重试次数。
- 日志：结构化 JSON，字段含 timestamp, level, session_id, channel, command, duration, exit_code。

## 故障处理
- Codex CLI 不可用：快速回显错误；自动降级为“请检查 CLI 环境”提示。
- 会话锁未释放：超时强制释放；记录异常。
- 渠道发送失败：重试 N 次；仍失败则写错误日志并提示“部分输出可能缺失”。\n*** End Patch
