# TC-L5-01 plan/apply 两段式

## 目标
- 验证聊天内配置管理（先计划、后执行）。

## 步骤
1. 发送：`/config plan 创建 agent reviewer 使用 claude`
2. 发送：`/config show`
3. 发送：`/config apply`
4. 执行：`go run ./cmd/clawx config agent list`

## 预期
1. `plan` 返回待确认计划摘要。
2. `show` 可看到同一会话下待确认计划。
3. `apply` 写入成功并提示“重启服务生效”。
4. 列表中出现 `reviewer`。
