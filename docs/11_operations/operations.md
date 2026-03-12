# 运维与运行手册（V1）

## 部署假设
- 环境：Linux，已安装 Node/Python（依据 Codex CLI 要求），可访问 Codex。
- 配置：env 文件或 yaml，含 bot tokens、Codex 路径、默认工作目录、超时时间。
- 进程管理：建议 systemd/pm2/supervisor 之一，保持单实例。

## 运行与监控
- 健康检查：进程存活；与 Codex CLI 的探针执行（短命令）。
- 指标基线：执行次数、成功/失败/超时、平均时长、在途执行数；输出分段失败重试次数。
- 日志：结构化 JSON，字段含 timestamp, level, session_id, channel, command, duration, exit_code。

## 日志路径（当前实现）
- 服务运行日志：默认输出到 stdout/stderr。
- 会话持久化：
  - `~/.clawx/agents/<agent_id>/sessions/sessions.json`
  - `~/.clawx/agents/<agent_id>/sessions/<session_id>.jsonl`
- Codex 执行追踪：
  - 索引：`~/.clawx/logs/index.jsonl`
  - 会话日志：`~/.clawx/logs/codex/<session_id>.jsonl`

可选覆盖：
- 设置 `CLAWX_LOG_DIR` 可改为其他日志根目录。

## Workspace 目录规范与迁移
- 当前默认目录：`~/.clawx/workspaces/<agent_id>`。
- 历史版本可能使用过：`~/.clawx/workworkspace/<agent_id>`。
- 服务启动时会自动尝试把旧目录迁移到新目录，并改写 `config.json` 中对应的 workspace 路径。
- 若新旧目录下存在同名冲突文件，系统会保留新目录版本，旧目录中冲突文件作为备份保留。

迁移建议（以 `main` 为例）：
```bash
go run ./cmd/clawx config agent add --id main --profile codex --workspace /home/ubuntu/.clawx/workspaces/main --default
```

## 索引查询示例
- 查看最近执行索引：
```bash
tail -n 20 ~/.clawx/logs/index.jsonl
```
- 按会话检索：
```bash
rg "sess-1772800389746985494" ~/.clawx/logs/index.jsonl
```
- 打开该会话的 Codex 追踪日志：
```bash
cat ~/.clawx/logs/codex/sess-1772800389746985494.jsonl
```

## 故障处理
- Codex CLI 不可用：快速回显错误；自动降级为“请检查 CLI 环境”提示。
- 会话锁未释放：超时强制释放；记录异常。
- 渠道发送失败：重试 N 次；仍失败则写错误日志并提示“部分输出可能缺失”。
