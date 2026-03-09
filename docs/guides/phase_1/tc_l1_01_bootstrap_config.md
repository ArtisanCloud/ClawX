# TC-L1-01 首次启动向导

## 目标
- 验证首次缺少配置时可自动引导。

## 步骤
1. 删除或移动当前 `~/.synapsex/config.json`。
2. 执行：`go run ./cmd/synapsex`
3. 按向导完成默认配置并确认写入。

## 预期
1. 生成 `~/.synapsex/config.json`。
2. 默认 `main` agent 存在。
3. 默认 `main` workspace 为 `~/.synapsex/workspaces/main`（未手工改时）。
4. 轻量主线推荐：数据库选 `否`，`database.enabled` 为 `false`。
