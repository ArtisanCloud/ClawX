# TC-L2-01 新增 Agent（默认 workspace）

## 目标
- 验证 CLI 管理 Agent 与默认目录规则。

## 步骤
1. 执行：`go run ./cmd/clawx config agent add --id project-a --profile codex`
2. 执行：`go run ./cmd/clawx config agent list`

## 预期
1. 列表中出现 `project-a`。
2. `workspace` 为 `~/.clawx/workspaces/project-a`。
3. 服务启动时会自动自检该目录；若目录不存在会自动创建。
