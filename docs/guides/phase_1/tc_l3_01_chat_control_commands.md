# TC-L3-01 基础控制命令

## 目标
- 验证 `/new`、`/list`、`/current`、`/resume`、`/cancel`。

## 步骤
1. 启动服务并接通一个频道（建议先 Discord 私聊或 Telegram 私聊）。
2. 发送：`/new`
3. 发送：`/list`
4. 发送：`/current`
5. 取 `session_id` 后发送：`/resume <session_id>`
6. 发送：`/cancel`

## 预期
1. `/new` 返回已创建会话。
2. `/list` 返回会话列表，并标注 `(current)`。
3. `/current` 返回当前会话 id 与状态。
4. `/resume` 成功恢复指定会话。
5. `/cancel` 能取消当前会话执行。
