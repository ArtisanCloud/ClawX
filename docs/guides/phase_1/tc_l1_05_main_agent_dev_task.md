# TC-L1-05 main agent 开发任务验证

## 目标
- 验证 `main agent -> Codex` 的任务执行链路，不止能回聊天文本，还能处理开发类请求。

## 前置
1. 已完成 `tc_l1_03_main_agent_first_chat.md`。
2. `main` 的 profile 为 `codex`。
3. `main` workspace 已指向你当前项目目录（或可访问的测试目录）。

## 步骤
1. 在 Discord 私聊发送：`请输出当前工作目录绝对路径`
2. 发送：`请列出当前目录下前 10 个文件，并按用途分组说明`
3. 发送：`请读取 docs/guides/phase_1/README.md，并给出推荐执行顺序摘要`
4. 观察服务端日志：每条普通消息都应出现 `discord execute begin` 和 `discord execute done`
5. 记录步骤 1、2、3 对应日志中的 `backend_session_id`

## 预期
1. 返回内容与 `main` workspace 实际路径一致。
2. 能返回文件列表和结构化说明，而不是固定模板回包。
3. 能基于仓库真实文件返回摘要，说明已具备项目上下文读取能力。
4. 日志中的执行记录包含 `agent=main`、`profile_kind=codex-cli`、`profile_command=codex`，且 `state=success`。
5. 在同一个 ClawX 会话内，`backend_session_id` 保持一致（表示复用同一 Codex thread）。

## 失败排查
1. 若路径不对，检查 `~/.clawx/config.json` 中 `agents.list` 里 `main.workspace`。
2. 若没有开发任务结果，检查本机 `codex` 命令是否可执行。
3. 若回复超时，优先看服务日志中的路由和发送错误。
