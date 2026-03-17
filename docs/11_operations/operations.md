# 运维与运行手册（V1）

## 部署假设
- 环境：Linux，已安装 Node/Python（依据 Codex CLI 要求），可访问 Codex。
- 配置：env 文件或 yaml，含 bot tokens、Codex 路径、默认工作目录、超时时间。
- 进程管理：建议 systemd/pm2/supervisor 之一，保持单实例。

## 运行与监控
- 健康检查：进程存活；与 Codex CLI 的探针执行（短命令）。
- 指标基线：执行次数、成功/失败/超时、平均时长、在途执行数；输出分段失败重试次数。
- 日志：结构化 JSON，字段含 timestamp, level, session_id, channel, command, duration, exit_code。

## 服务安装（Linux 用户级 systemd）
- 新增命令：`clawx install-service`
- 作用：安装并启用 `systemd --user` 服务，默认服务名 `clawx.service`，执行 `clawx run`（由 `service.run` 决定目标）。

一键方式（推荐）：
```bash
clawx setup-service --target discord
```
该命令会自动执行：
- `go build` 到 `~/.local/bin/clawx`
- 写入 `service.run=<target>`
- 安装并启用用户服务
- 立即启动服务

推荐流程：
```bash
# 1) 构建稳定二进制（不要用 go run 临时路径）
go build -o /usr/local/bin/clawx ./cmd/clawx

# 2) 安装并立即启动
clawx install-service --binary /usr/local/bin/clawx --start

# 3) 查看状态
systemctl --user status clawx.service
journalctl --user -u clawx.service -f
```

可选参数：
- `--name <service-name>`：自定义服务名（如 `clawx-discord`）。
- `--start`：安装后立即启动（会执行 `restart`）。
- `--force`：覆盖已存在 unit 文件。
- `--binary <abs-path>`：显式指定可执行文件路径。

注意：
- `install-service` 仅支持 Linux。
- 如果你是通过 `go run ./cmd/clawx ...` 执行命令，必须传 `--binary`，否则会拒绝安装（因为 `go run` 生成的是临时可执行文件）。

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
