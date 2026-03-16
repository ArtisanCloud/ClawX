# Discord Bot 创建与接入指南

## 目标
- 说明如何创建 Discord Bot。
- 说明如何提前准备 Discord 接入所需的账号和权限。
- 明确当前 ClawX 在 Discord 上的支持状态，避免误判为已经可直接联调。

## 当前支持状态
- 当前分支已提供 `Bot Token + Gateway WebSocket` 运行时代码。
- 你可以在完成 Bot 创建、邀请、权限和 `config.json` 配置后直接联调。
- 我没有替你做外部 Discord 联网验收；这一步仍需要你自己的 Bot Token 和服务器环境。

## 第一步：创建 Discord Application
1. 打开 Discord Developer Portal
2. 创建一个新的 Application
3. 填写应用名称
4. 进入该应用的 `Bot` 页面

## 第二步：创建 Bot
1. 在 `Bot` 页面添加 Bot
2. 创建后记录：
   - Bot Token
   - Application ID（Client ID）

这些信息在当前分支的 Discord 运行时里就会直接用到。

## 第三步：打开必要权限
建议至少确认：
- 可读取消息
- 可发送消息
- 可读取消息历史

如果未来要做群聊 @bot 和线程行为，还应关注：
- 线程相关权限
- 频道可见权限

## 第四步：开启必要 Intents
在 `Bot` 配置页中，至少关注：
- `MESSAGE CONTENT INTENT`

否则未来即使接入 Discord Gateway，也可能读不到消息正文。

## 第五步：邀请 Bot 进入服务器
1. 在 OAuth2 / URL Generator 中生成邀请链接
2. 选择：
   - `bot`
3. 勾选所需权限
4. 用生成的链接把 bot 邀请到目标服务器

## 推荐的触发模型
根据当前项目文档，Discord 预期行为是：
- 私聊：直接触发
- 群聊：需要 `@bot`
- Thread：映射独立 session

这是当前代码的目标行为模型。

## 建议你现在先准备的内容
建议你至少准备并保管：
- Bot Token
- Application ID
- 目标服务器 ID
- 目标频道 / Thread 的使用范围

这样联调时不需要再回头补账号信息。

## 第六步：配置 ClawX
在 `config.json` 中填写：

```json
{
  "agents": {
    "default": "main"
  },
  "channels": {
    "discord": {
      "enabled": true,
      "botToken": "你的_discord_bot_token",
      "apiBaseUrl": "https://discord.com/api/v10",
      "gatewayUrl": "wss://gateway.discord.gg/?v=10&encoding=json",
      "allowedChannelIds": [],
      "requireMention": true
    }
  }
}
```

说明：
- `agents.default = "main"` 表示复用默认 agent（建议让它指向 `codex` 或 `claude`）
- `channels.discord.allowedChannelIds` 为空数组表示不限制频道范围
- `channels.discord.requireMention=true` 表示群聊里默认需要 `@bot`
- 私聊不受 mention 限制

## 第七步：启动服务
执行：

```bash
go run ./cmd/clawx config
go run ./cmd/clawx
```

如果 `config.json` 不存在，第一条命令会进入交互式向导；如果你直接执行第二条，程序也会先进入向导，完成后继续启动。

预期日志包含：

```text
discord gateway adapter started
clawx service started with agent "main" using profile "codex"
```

## 第八步：最小人工联调
建议先用私聊验证：
1. 私聊 Bot 发送 `/new`
2. 预期返回：`已创建新会话: sess-...`
3. 发送 `/current`（确认当前会话）
4. 再发送 `hello`
5. 预期返回执行结果

如果在群聊验证，默认写法是：
- `@bot /new`
- `@bot hello`

说明：
- 当前版本会在 Discord READY 后自动同步基础 Slash Commands：`/new`、`/list`、`/current`、`/resume`、`/cancel`。
- 若刚启动后命令列表未立即出现，等待 Discord 全局命令同步完成后再试（可能有延迟）。

说明：
- 当前默认执行器如果仍是 `cat`，普通文本会被原样回显
- 这是有意的 smoke test 行为，不是最终业务执行器

## 当前结论
- Discord 应该有独立文档
- 当前分支已经有可运行的 Discord Gateway 聊天代码
- 真正是否联调成功，取决于你的 Bot Token、权限和服务器环境
