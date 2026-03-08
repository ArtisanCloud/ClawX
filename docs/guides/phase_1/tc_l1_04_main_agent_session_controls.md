# TC-L1-04 main agent 会话控制

## 目标
- 验证在只使用 `main agent` 的前提下，会话控制命令可稳定使用。

## 前置
1. 已完成 `tc_l1_03_main_agent_first_chat.md`。
2. 在同一个 Discord 私聊窗口内继续操作。

## 步骤
1. 发送：`/new`
2. 发送：`/new`
3. 发送：`/list`
4. 从列表中任选一个 `session_id`，发送：`/resume <session_id>`
5. 发送：`/cancel`

## 预期
1. 能连续创建多个会话。
2. `/list` 返回会话列表，且包含刚创建的会话。
3. `/resume` 后返回已切换到目标会话。
4. `/cancel` 能结束当前会话任务，不影响服务进程存活。

## 说明
- 这一组命令是 SynapseX 会话层命令，不是 Codex CLI 自身的 `/new`。
