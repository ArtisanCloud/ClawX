# TC-L4-01 用 `pwd` 验证路由目录

## 目标
- 验证“命中哪个 agent，就在其 workspace 执行”。

## 预置
1. 在 `providers.profiles` 新增一个测试 profile（例如 `pwd-smoke`）：
   - `kind: generic-cli`
   - `command: pwd`
2. 新增两个 agent：
   - `main` 的 `workspace`: `~/.synapsex/workspaces/main`
   - `review` 的 `workspace`: `~/.synapsex/workspaces/review`
   - 两者都使用 `pwd-smoke`
3. 在 channel 配置绑定一个入口到 `review`：
   - Discord 示例：`"channel:<your_channel_id>": "review"`
   - Telegram 示例：`"chat:<your_chat_id>": "review"`

## 步骤
1. 重启服务使配置生效。
2. 在绑定入口发任意文本（如 `hello`）。
3. 在未绑定入口发任意文本（如 `hello`）。

## 预期
1. 绑定入口返回 `.../workspaces/review`。
2. 未绑定入口返回 `.../workspaces/main`。
