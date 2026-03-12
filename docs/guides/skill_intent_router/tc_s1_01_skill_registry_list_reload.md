# TC-S1-01 Skill Registry list/reload

## 目标
- 验证 Skill 发现、刷新与状态展示。

## 前置
1. 服务至少启动过一次（会自动创建 `~/.clawx/skills/echo/SKILL.md`）。
2. 若你手工修改过该文件，需保留 `name` 与 `description` frontmatter。

## 步骤
1. 执行：`go run ./cmd/clawx skill list`
2. 执行：`go run ./cmd/clawx skill reload`
3. 再次执行：`go run ./cmd/clawx skill list`

## 预期
1. `echo` 可见。
2. 状态为 `active`。
3. `reload` 返回版本与条目统计。
