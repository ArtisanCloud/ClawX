# 日志追踪指南（按 Session 全链路）

## 目标
- 给定一个 `session_id`，快速定位：
  - 服务处理过程日志
  - 会话持久化日志
  - Codex 执行日志

## 日志层级
1. 服务日志（runtime）  
- 来源：`go run ./cmd/clawx` 的 stdout/stderr  
- 建议落盘：

```bash
mkdir -p ~/.clawx/logs
go run ./cmd/clawx 2>&1 | tee -a ~/.clawx/logs/service.log
```

2. 会话持久化日志  
- 索引：`~/.clawx/agents/<agent_id>/sessions/sessions.json`
- 事件：`~/.clawx/agents/<agent_id>/sessions/<session_id>.jsonl`

3. Codex 执行追踪日志  
- 全局索引：`~/.clawx/logs/index.jsonl`
- 会话执行：`~/.clawx/logs/codex/<session_id>.jsonl`

可选：
- 设置 `CLAWX_LOG_DIR` 可覆盖默认日志根目录。

## 追踪步骤（推荐）
1. 从聊天里拿到 `session_id`（例如 `/list` 返回的 `sess-...`）。
2. 查服务日志中该 session 的路由和执行轨迹：

```bash
rg "sess-1772800389746985494|discord route|discord execute|backend_session_id" ~/.clawx/logs/service.log
```

3. 查会话持久化状态：

```bash
cat ~/.clawx/agents/main/sessions/sess-1772800389746985494.jsonl
```

4. 查 Codex 执行日志索引：

```bash
rg "sess-1772800389746985494" ~/.clawx/logs/index.jsonl
```

5. 打开索引对应的会话执行日志：

```bash
cat ~/.clawx/logs/codex/sess-1772800389746985494.jsonl
```

## 常用过滤模板
1. 看最近 20 条 Codex 执行索引：

```bash
tail -n 20 ~/.clawx/logs/index.jsonl
```

2. 只看错误：

```bash
rg "\"state\":\"failed\"|\"state\":\"timeout\"|\"error\":" ~/.clawx/logs/codex/*.jsonl
```

3. 按 backend session 追溯：

```bash
rg "019cc323-72a9-7821-83f2-21c31007beaa" ~/.clawx/logs/index.jsonl ~/.clawx/logs/codex/*.jsonl
```

## 注意事项
- 控制命令（如 `/list`、`/resume`）通常不会触发 Codex 执行日志，但会出现在服务日志和会话日志。
- 执行日志可能包含输入与输出文本，生产环境请按需要做脱敏与访问控制。
