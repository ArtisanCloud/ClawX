# Agent 创建与工作目录指南

## 先说结论
- 可以创建多个 Agent。
- 每个 Agent 可以有独立 `workspace`。
- 路由命中哪个 Agent，就在该 Agent 的 `workspace` 下执行。
- 默认配置文件位置是 `~/.synapsex/config.json`（可用 `SYNAPSEX_CONFIG` 覆盖）。

## Agent 是什么
- Agent 是“运行模板”，不是会话本身。
- 一个 Agent 定义了：
  - 使用哪个执行器 profile（`codex` / `claude` / 其他）
  - 默认工作目录（`workspace`）
  - 超时时间（`timeoutSeconds`）

## 第一步：准备 profile
在 `config.json` 的 `providers.profiles` 中定义执行器：

```json
{
  "providers": {
    "profiles": {
      "codex": {
        "kind": "codex-cli",
        "command": "codex",
        "args": [],
        "healthArgs": ["--version"]
      },
      "claude": {
        "kind": "claude-cli",
        "command": "claude",
        "args": [],
        "healthArgs": ["--version"]
      }
    }
  }
}
```

## 第二步：创建 Agent（含独立 workspace）
在 `agents` 段定义多个 Agent：

```json
{
  "agents": {
    "default": "main",
    "list": [
      {
        "id": "main",
        "profile": "codex",
        "workspace": "/workspace/project-a",
        "timeoutSeconds": 600,
        "default": true
      },
      {
        "id": "reviewer",
        "profile": "claude",
        "workspace": "/workspace/project-b",
        "timeoutSeconds": 900
      }
    ]
  }
}
```

说明：
- `id`：Agent 标识（全局唯一）。
- `profile`：必须引用已有 profile。
- `workspace`：该 Agent 的默认工作目录（独立）。
- `agents.default`：兜底 Agent。

## 第三步：把 Channel 路由到 Agent
可在渠道级和实例级配置默认 Agent 与绑定规则。

```json
{
  "channels": {
    "discord": {
      "defaultAgent": "main",
      "agentBindings": {
        "channel:123456789012345678": "reviewer"
      },
      "instances": [
        {
          "id": "discord-main",
          "enabled": true,
          "botToken": "YOUR_TOKEN",
          "defaultAgent": "main",
          "agentBindings": {
            "channel:223456789012345678": "reviewer"
          }
        }
      ]
    },
    "telegram": {
      "defaultAgent": "main",
      "agentBindings": {
        "chat:-1001234567890": "reviewer"
      },
      "instances": [
        {
          "id": "telegram-main",
          "enabled": true,
          "mode": "polling",
          "token": "YOUR_TOKEN",
          "defaultAgent": "main"
        }
      ]
    }
  }
}
```

## 通过 CLI 管理 Agent（推荐）
当前已支持以下命令：

```bash
# 查看 agent 列表
synapsex config agent list

# 新增/更新 agent
synapsex config agent add --id reviewer --profile claude

# 指定 workspace 与超时
synapsex config agent add --id project-a --profile codex --workspace /workspace/project-a --timeout 900

# 新增后直接设为默认 agent
synapsex config agent add --id project-b --profile codex --default

# 切换默认 agent
synapsex config agent default project-a
```

默认行为：
- 如果 `--workspace` 不传，自动使用：
  - `main` -> `~/.synapsex/workspaces/main`
  - 其他 agent（如 `reviewer`）-> `~/.synapsex/workspaces/reviewer`
- 写入 agent 时会自动补齐 `runtime.allowedRoots`，保证 workspace 可执行。

历史兼容说明：
- 若你之前配置的是 `~/.synapsex/workworkspace/...`，服务启动时会自动迁移到 `~/.synapsex/workspaces/...` 并改写配置。
- 若有同名冲突，优先保留 `workspaces` 下已有内容；冲突文件会留在旧目录作为备份。
- 你也可以用 `config agent add` 显式重写目标路径。

## 通过聊天自然语言改配置（MVP）
当前已支持“先计划，再确认执行”的两段式：

```text
/config plan 创建 agent review 使用 claude
/config show
/config apply
```

也支持英文结构化表达：

```text
/config plan add agent review profile claude workspace /workspace/review timeout 900 default
/config plan default agent review
```

说明：
- `plan` 只生成待确认变更，不会立即写入。
- `apply` 才会写入 `config.json`。
- 写入后需要重启服务生效。
- 可通过环境变量 `SYNAPSEX_CONFIG_ADMIN_USERS` 限制哪些用户可执行配置指令。

## 在频道内切换当前会话 Agent（无需重启）
当前已支持在聊天窗口直接切换“当前会话走哪个 Agent”：

```text
/agent list
/agent current
/agent use <agent_id>
/agent clear
```

说明：
- `use` 仅影响“当前会话范围”（当前 channel + bot instance + conversation）。
- `clear` 后恢复配置路由（`agentBindings/defaultAgent`）。
- 这组命令不会改 `config.json`，也不需要重启服务。

## 绑定 key 写法
- Discord 支持：
  - `channel:<channel_id>` 或直接 `<channel_id>`
  - `user:<user_id>`
  - `conversation:<conversation_id>`
- Telegram 支持：
  - `chat:<chat_id>` 或直接 `<chat_id>`
  - `user:<user_id>`
  - `conversation:<conversation_id>`

## 运行时路由优先级
- `实例 agentBindings`
- `渠道 agentBindings`
- `实例 defaultAgent`
- `渠道 defaultAgent`
- `agents.default`

## `/new` 与 Codex CLI 的关系
- `/new` 是 SynapseX 的会话命令，只创建 SynapseX 自己的 `session_id`。
- 当前不会调用 Codex CLI 的 `/new`。
- 普通消息会按当前命中的 agent，执行一次 `codex exec`（或对应 profile 命令）。
- 因此当前模型是：
  - 会话状态：由 SynapseX 管理
  - 模型执行：按消息触发单次 CLI 进程
  - 工作目录：由 agent `workspace` 决定

## 验证 Agent 是否按独立 workspace 执行
1. 确保 `runtime.allowedRoots` 覆盖所有 Agent 的 `workspace`。
2. 启动服务：`go run ./cmd/synapsex`
3. 观察启动日志包含：`synapsex service started with N runtime(s) ...`
4. 在命中不同绑定的频道/聊天发消息，确认分别由对应 Agent 响应。

## 当前限制
- `synapsex config` 向导当前只引导默认 Agent（`main`）的基础配置。
- 额外 Agent、多实例 Bot、精细 `agentBindings` 需要手工编辑 `config.json`。
