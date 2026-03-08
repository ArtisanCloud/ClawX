# 第一阶段主线联调指南（main + Discord + Codex）

## 目标
- 先验证最小可用主线：`main agent` 能在 Discord 私聊中接收消息，并把任务转发给 Codex 执行后回传结果。
- 本文不要求你先创建额外 agent。
- 本文默认不启用数据库。

## 结果判定
- 你在 Discord 私聊 Bot 发送 `/new`、普通开发指令，能收到 SynapseX 回复。
- 服务端日志出现 Discord 入站和路由日志，并且没有发送失败错误。

## Step 1: 前置条件
1. 当前分支：`001-phase1-foundation`。
2. Go 版本：`1.23.x`。
3. 本机已可执行 Codex CLI（`codex` 命令可用）。
4. 已准备 Discord Bot Token，并已邀请 Bot 到你的服务器（私聊测试不依赖服务器频道权限）。

## Step 2: 首次生成配置（只做 main）
执行：

```bash
go run ./cmd/synapsex
```

当 `~/.synapsex/config.json` 不存在时，会自动进入引导。建议选择：
1. 默认执行器：`Codex`
2. Channel：`只配置 Discord`
3. 填写 `Discord Bot Token`
4. `群聊要求 @bot 触发`：保持默认 `是`
5. `启用 PostgreSQL 持久化 Session`：`否`
6. 确认写入配置：`是`

说明：
- `main agent` 会自动创建并作为默认 agent。
- 不需要在这一步新增其他 agent。

## Step 3: 校验关键配置
确认文件存在：

```bash
ls -la ~/.synapsex/config.json
```

至少确认这些关键项：
- `agents.default` 是 `main`
- `agents.list` 里有 `id: "main"`
- `agents.list[main].profile` 是 `codex`
- `agents.list[main].workspace` 指向 `~/.synapsex/workspaces/main`（或你手动指定的路径）
- `channels.discord.enabled` 为 `true`
- `database.enabled` 为 `false`

## Step 4: 启动服务
执行：

```bash
go run ./cmd/synapsex
```

预期日志包含：

```text
synapsex service started with ... runtime(s); default agent "main"
session store initialized: driver=file state_dir=...
discord gateway adapter started
discord gateway ready: bot_user_id=...
http runtime listening on http://:8080
health probe available at http://:8080/healthz
```

## Step 5: Discord 私聊验证（主线）
在 Discord 私聊 Bot，按顺序发送：
1. `/new`
2. `请输出你当前工作目录，并列出前 5 个文件名`

预期：
1. `/new` 返回会话已创建（session id）。
2. 第二条消息触发执行并回传文本结果。
3. 本地生成会话文件：`~/.synapsex/agents/main/sessions/sessions.json` 与对应 `*.jsonl`。

如果你只想先测控制命令，再发：
1. `/list`
2. `/resume <session_id>`
3. `/cancel`

对应分场景用例：
1. `tc_l1_03_main_agent_first_chat.md`
2. `tc_l1_04_main_agent_session_controls.md`
3. `tc_l1_05_main_agent_dev_task.md`

## Step 6: 失败时看哪里
如果 Discord 显示 `The application did not respond`，先看服务日志是否出现：
- `discord inbound message`
- `discord route begin`
- `send discord message: ...`

常见原因：
1. 代理或网络重置（例如 `connection reset by peer`）。
2. Token 配置错误或 Bot 会话失效。
3. 服务虽然启动，但并未收到 Gateway 事件（没有 `discord gateway ready`）。

追踪建议：
1. 服务日志建议落盘：
```bash
mkdir -p ~/.synapsex/logs
go run ./cmd/synapsex 2>&1 | tee -a ~/.synapsex/logs/service.log
```
2. 按 session 检索：
```bash
rg "sess-|backend_session_id|discord execute" ~/.synapsex/logs/service.log
```
3. 查看 Codex 执行索引：
```bash
tail -n 20 ~/.synapsex/logs/index.jsonl
```
4. 用 `session_id` 追溯 Codex 运行日志：
```bash
cat ~/.synapsex/logs/codex/<session_id>.jsonl
```

更完整的排障路径见：`../log_tracing.md`。

## Step 7: 目录命名修正说明
- 当前默认 workspace 根目录是：`~/.synapsex/workspaces`。
- 如果你的历史配置还在用 `~/.synapsex/workworkspace`，服务启动会自动迁移并更新配置。
- 若发生同名冲突，系统保留 `workspaces` 现有内容，旧目录冲突项作为备份保留。
- 建议手工更新 `main` 的 workspace 后重启：

```bash
go run ./cmd/synapsex config agent add --id main --profile codex --workspace /home/ubuntu/.synapsex/workspaces/main --default
```

## 当前阶段边界
- 第一阶段主线先保证 `main` 可用。
- 多 agent（如 `reviewer`）属于进阶，不阻塞你验证 Discord->Codex 基础链路。
- 当前主线不要求数据库；默认使用本地文件持久化。

进阶文档：
- `tc_l2_01_agent_add_default_workspace.md`
- `tc_l2_02_agent_switch_default.md`
- `tc_l4_01_agent_workspace_routing_pwd.md`
- `../agent_setup.md`
- `../storage_local_first.md`
