# TC-L1-01 首次启动向导

## 目标
- 验证首次缺少配置时可自动引导。

## 步骤
1. 删除或移动当前 `~/.clawx/config.json`。
2. 执行：`go run ./cmd/clawx`
3. 按向导完成默认配置并确认写入。

## 预期
1. 生成 `~/.clawx/config.json`。
2. 默认 `main` agent 存在。
3. 默认 `main` workspace 为 `~/.clawx/workspaces/main`（未手工改时）。
4. 轻量主线推荐：数据库选 `否`，`database.enabled` 为 `false`。
