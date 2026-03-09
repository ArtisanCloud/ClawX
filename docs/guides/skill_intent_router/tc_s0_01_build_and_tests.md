# TC-S0-01 编译与回归基线

## 目标
- 确认当前分支可编译、可测试，为后续 Skill/Intent 联调提供干净基线。

## 步骤
1. 执行：`go test ./...`
2. 执行：`go run ./cmd/synapsex skill list`

## 预期
1. `go test ./...` 全部通过。
2. `skill list` 可返回结果（即使为空，也不报错）。
