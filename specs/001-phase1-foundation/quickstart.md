# 快速启动：第一阶段基座能力

## 目标
帮助开发者快速启动当前第一阶段实现，并按最小可用路径完成本地 smoke test。

## 开发前准备
1. 确认当前分支为 `001-phase1-foundation`
2. 阅读以下文档：
   - `.specify/memory/constitution.md`
   - `docs/plans/phase_1_foundation.md`
   - `docs/guides/phase_1/phase_1_mvp_guide.md`
   - `specs/001-phase1-foundation/spec.md`
3. 确认 Go 版本为 `1.23`

## 最小启动步骤
1. 初始化本地配置：

```bash
go run ./cmd/clawx config
```

如果 `config.json` 不存在，这条命令会进入交互式向导（支持上下键切换、数字直达、回车确认），写入前会先显示配置摘要，并在确认后写入默认结构的 `config.json`。

2. 使用最小 smoke test 配置：

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
      "enabled": false
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

3. 执行编译测试：

```bash
go test ./...
```

4. 启动服务：

```bash
go run ./cmd/clawx
```

如果你跳过了第 1 步，且当前目录还没有 `config.json`，这条命令会先进入交互式配置向导；向导完成后会继续启动服务。

5. 在另一个终端检查健康状态：

```bash
curl -s http://127.0.0.1:8080/healthz
```

## Telegram 最小接入（可选）
如果需要验证真实渠道链路，可在 `config.json` 中开启：

```json
{
  "channels": {
    "telegram": {
      "enabled": true,
      "mode": "polling",
      "token": "你的_bot_token",
      "botUsername": "你的bot用户名",
      "requireCommandOrMention": true
    }
  }
}
```

然后重启服务，并在 Telegram 私聊里验证：
- `/new`
- 普通文本 `hello`

在默认 `cat` 后端下，预期：
- `/new` 返回已创建会话
- `hello` 被原样回显为 `hello`

## Discord 最小接入（可选）
如果需要验证 Discord，可在 `config.json` 中开启：

```json
{
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

然后重启服务，并在 Discord 中验证：
- 私聊 Bot 发送 `/new`
- 再发送普通文本 `hello`

在默认 `cat` 后端下，预期：
- `/new` 返回已创建会话
- `hello` 被原样回显为 `hello`

## 第一阶段当前已完成
- 常驻服务启动
- 健康检查对外可访问
- Telegram 最小真实接入
- Discord Gateway 运行时代码
- `codex-cli` / `claude-cli` 配置模型与本地 smoke fallback
- 本地集成测试覆盖最小“入站 -> 路由 -> 执行 -> 出站”链路

## 第一阶段当前仍建议继续补强
- 把默认 agent 切到真实 `codex` 或 `claude`
- 完成一次带真实 Telegram / Discord 凭证的人工验收
- 按实际验收结果更新阶段文档
- 如需让 `.env` 继续参与覆盖，显式设置 `CLAWX_LOAD_DOTENV=true`

## 明确不做
- 多窗口管理
- 多 Agent 管理
- 自动路由
- 重型中台或复杂调度层
