# Telegram Bot 创建与接入指南

## 目标
- 说明如何创建 Telegram Bot。
- 说明如何把 Telegram Bot 配置到当前 SynapseX。
- 说明当前阶段可完成到哪一步，以及怎么做最小人工验收。

## 当前支持状态
- 当前 `001-phase1-foundation` 已接通 Telegram 的最小真实运行链路。
- 当前支持：
  - Telegram Bot API 长轮询
  - 私聊消息接收
  - 控制命令进入 Router
  - 普通文本进入执行链路
  - 输出回写到 Telegram
- 当前不建议先做群聊复杂联调；第一次验收优先用私聊。

## 第一步：创建 Telegram Bot
1. 打开 Telegram，找到 `@BotFather`
2. 发送 `/newbot`
3. 按提示输入：
   - Bot 显示名称
   - Bot 用户名（必须以 `bot` 结尾）
4. 创建成功后，记录两项信息：
   - Bot Token
   - Bot 用户名

你后面会用到：
- `telegram_token`
- `telegram_bot_username`

## 第二步：准备本地配置
在仓库根目录执行：

```bash
go run ./cmd/synapsex config
```

如果 `config.json` 不存在，这条命令会进入交互式向导。你可以在向导里直接选择是否启用 Telegram；也可以先跳过，再手工编辑 `config.json`。

然后编辑 `config.json`，至少填这些：

```json
{
  "providers": {
    "profiles": {
      "local-smoke": {
        "kind": "generic-cli",
        "command": "cat",
        "args": [],
        "healthArgs": []
      }
    }
  },
  "agents": {
    "default": "local-smoke",
    "list": [
      {
        "id": "local-smoke",
        "profile": "local-smoke",
        "workspace": ".",
        "timeoutSeconds": 600
      }
    ]
  },
  "runtime": {
    "allowedRoots": ["."],
    "defaultCwd": ".",
    "timeoutSeconds": 600
  },
  "execution": {
    "command": "cat",
    "args": [],
    "healthArgs": []
  },
  "channels": {
    "discord": {
      "enabled": false
    },
    "telegram": {
      "enabled": true,
      "mode": "polling",
      "token": "你的_bot_token",
      "botUsername": "你的bot用户名",
      "allowedChatIds": [],
      "requireCommandOrMention": true,
      "pollingSeconds": 30
    }
  },
  "gateway": {
    "listenAddr": ":8080",
    "health": {
      "enabled": true,
      "path": "/healthz"
    }
  }
}
```

说明：
- 如果你已经装好了 `codex` 或 `claude`，更推荐直接复用 `config.example.json` 里的 `providers + agents`
- 上面这段是最小 smoke test 写法，方便先验证 Telegram 链路
- 如果你还保留了旧 `.env`，默认情况下它不会再覆盖已经存在的 `config.json`

## 配置项说明
- `channels.telegram.enabled`
  - 是否启用 Telegram 运行时
- `channels.telegram.mode`
  - 当前只支持 `polling`
- `channels.telegram.token`
  - BotFather 提供的 token
- `channels.telegram.botUsername`
  - bot 用户名，不带 `@` 也可以
- `channels.telegram.allowedChatIds`
  - 可选，数组
  - 留空表示不限制 chat
- `channels.telegram.requireCommandOrMention`
  - 群聊中是否要求命令或提及
  - 私聊不受这个限制
- `channels.telegram.pollingSeconds`
  - 长轮询等待时间

## 第三步：启动 SynapseX
在仓库根目录执行：

```bash
go test ./...
go run ./cmd/synapsex
```

预期日志类似：

```text
http runtime listening on http://:8080
health probe available at http://:8080/healthz
telegram adapter started in polling mode
synapsex service started with agent "local-smoke" using profile "local-smoke"
```

## 第四步：做最小人工验收
建议先用 Telegram 私聊 bot。

### 验证 1：控制命令
发送：

```text
/new
```

预期：
- 返回 `已创建新会话: sess-...`

### 验证 2：普通文本执行
发送：

```text
hello
```

预期：
- 返回 `hello`

说明：
- 当前 smoke test 默认 agent 指向 `local-smoke`
- 所以这是“真实本地命令执行 + 原样回显”的 smoke test

### 验证 3：会话命令
继续测试：

```text
/list
```

预期：
- 返回当前 conversation 下的会话列表

如果你有一个已有 session id，还可以测试：

```text
/resume sess-xxxxxxxx
```

## 群聊与提及说明
- 当前第一次验收不建议先在群里做
- 如果你要在群里验证：
  - 建议保持 `channels.telegram.requireCommandOrMention=true`
  - 只让命令或显式提及时触发

当前代码行为：
- 命令消息会进入控制流
- 私聊普通文本会直接进入执行流
- 群聊普通文本默认需要命令或提及

## 常见问题

### 1. 没有任何响应
检查：
- `channels.telegram.enabled=true`
- `channels.telegram.token` 是否正确
- 服务是否仍在运行

### 2. 启动时报配置错误
检查：
- `channels.telegram.token` 是否为空
- `channels.telegram.mode` 是否为 `polling`

### 3. 健康检查正常，但 Telegram 不回消息
说明：
- 健康检查只说明服务和本地执行器正常
- 不代表 Telegram token、权限或 bot 配置正确

优先检查：
- token 是否正确
- bot 是否已创建成功
- 你是否真的给 bot 发了私聊

## 当前限制
- 当前只优先打通 Telegram
- 当前示例里的默认 agent 是 `local-smoke`，不是最终业务执行器
- 如果你要用于实际业务，需要把默认 agent 切到 `codex` 或 `claude`

## 下一步建议
1. 先用私聊完成一次最小 smoke test
2. 再把默认 agent 切到真实 `codex` 或 `claude`
3. 最后再考虑群聊、Topic 或更复杂触发规则
