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
- 会话 Agent 覆盖持久化文件：`~/.clawx/agent_overrides.json`（用于重启后恢复 conversation->agent 绑定）。

## Prompt Caching 观测与门禁
- 观测来源：`~/.clawx/logs/trace.jsonl` 中 `event=llm_io` 且 `phase=response` 的记录。
- 关键字段：
  - `prompt_tokens`: 本次请求 prompt token 总量
  - `prompt_cached_tokens`: 命中缓存的 prompt token 量
  - `intent_kind`: 分阶段执行线路（可作为 stage 维度）
  - `channel`、`agent_id`: 维度分组
- 命中率定义：`prompt_cached_tokens / prompt_tokens`（按聚合窗口统计）。

建议门禁阈值（7 天滚动窗口）：
- `hit_rate < 15%`：告警（缓存键稳定性可能不足，建议检查 key 是否包含高变字段）。
- `15% <= hit_rate < 35%`：关注（可继续优化分阶段上下文稳定段）。
- `hit_rate >= 35%`：健康（满足基础成本优化目标）。

快速检查（最近 200 条 llm 响应）：
```bash
tail -n 200 ~/.clawx/logs/trace.jsonl \
  | jq -r 'select(.event=="llm_io" and .phase=="response" and (.prompt_tokens // 0) > 0) | [.channel,.agent_id,.intent_kind,.prompt_cached_tokens,.prompt_tokens] | @tsv'
```

汇总报表（建议接入内部聚合器）：
- 代码位置：`internal/infrastructure/logging/prompt_cache_metrics.go`
- 能力：按 `channel/agent/stage` 输出 `responses/tokens/cached/hit_rate`。
- 可用于定时任务或日志管道的二次统计，并写入你们现有监控系统。

CLI 快速生成报表：
```bash
# 文本报表
clawx trace cache-report --file ~/.clawx/logs/trace.jsonl

# JSON 报表（便于管道接入）
clawx trace cache-report --file ~/.clawx/logs/trace.jsonl --json
```

## Token 用量与成本观测
- 写入文件：`~/.clawx/logs/token_usage.jsonl`（每次 LLM 响应一条记录）。
- 关键字段：`prompt_tokens`、`completion_tokens`、`total_tokens`、`prompt_cached_tokens`、`estimated_cost_usd`。
- 成本估算环境变量（可选）：
  - `CLAWX_TOKEN_COST_PROMPT_PER_1M`
  - `CLAWX_TOKEN_COST_COMPLETION_PER_1M`
- token 日志轮转环境变量：
  - `CLAWX_TOKEN_USAGE_LOG_FILE`（默认 `~/.clawx/logs/token_usage.jsonl`）
  - `CLAWX_TOKEN_USAGE_LOG_MAX_MB`（默认 `20`）
  - `CLAWX_TOKEN_USAGE_LOG_MAX_BACKUPS`（默认 `5`）

CLI 汇总：
```bash
# 文本报表
clawx trace token-report --file ~/.clawx/logs/token_usage.jsonl

# JSON 报表（便于接入外部日志/计费系统）
clawx trace token-report --file ~/.clawx/logs/token_usage.jsonl --json
```

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
