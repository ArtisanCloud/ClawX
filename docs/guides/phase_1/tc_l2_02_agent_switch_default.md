# TC-L2-02 切换默认 Agent

## 目标
- 验证默认 agent 切换。

## 步骤
1. 执行：`go run ./cmd/synapsex config agent default project-a`
2. 执行：`go run ./cmd/synapsex config agent list`

## 预期
1. `project-a` 标记为 `(default)`。
