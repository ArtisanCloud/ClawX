# 快速启动：项目空间隔离与路由

## 目标
验证 Phase 5 的最小闭环：
- 项目注册与 workspace 隔离
- route key 项目绑定与 `/project` 控制命令
- `/new` 只在当前项目内新建会话
- 意图切换建议（confirm-first）

## 开发前准备
1. 确认分支：`005-project-workspace-routing`
2. 阅读：
   - `docs/plans/phase_5_project_workspace_routing.md`
   - `specs/005-project-workspace-routing/spec.md`
   - `specs/005-project-workspace-routing/plan.md`
3. 确认本地可运行：
   - `go run ./cmd/clawx serve`
   - `go run ./cmd/clawx config`
   - `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...`

## 建议目录布局（.clawx）

```text
~/.clawx/
├── projects/
│   ├── projects.json
│   ├── bindings.json
│   └── proposals.json
└── workspaces/
    ├── main/
    ├── bid/
    └── nba/
```

## 手工验收脚本

### A. 项目初始化
1. 执行 `/project create bid`。
2. 执行 `/project create nba`。
3. 执行 `/project list`，确认 `main/bid/nba` 均可见。

预期：
- 目录存在：`~/.clawx/workspaces/bid`、`~/.clawx/workspaces/nba`
- 注册表存在对应项目元数据。

### B. route key 绑定
1. 在 route key A（例如频道线程 A）执行 `/project use bid`。
2. 在 route key B（例如频道线程 B）执行 `/project use nba`。
3. 分别执行 `/project current`。

预期：
- A 返回 `bid`；B 返回 `nba`。
- `bindings.json` 出现两条不同 route key 绑定。

### C. `/new` 语义校验
1. 在 A 执行 `/new`。
2. 在 B 执行 `/new`。
3. 分别发送普通任务请求。

预期：
- A 的会话与执行目录落在 `bid`。
- B 的会话与执行目录落在 `nba`。
- `/new` 未创建新项目、未改写已有项目绑定。

### D. 意图切换建议
1. 在 A（当前 `bid`）发送明显属于 `nba` 的任务。
2. 系统返回“建议切到 `nba`”。
3. 执行确认命令后继续发送任务。

预期：
- 确认前不切换。
- 确认后 A 绑定更新为 `nba`。
- 审计日志包含 `proposal_created` 与 `project_switch`。

## 自动化回归

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

建议重点回归：
- `tests/unit/project_*`
- `tests/integration/project_routing_*`
- `tests/contract/project_command_contract_test.go`

## 完成检查
- `/project` 主命令闭环可用（create/list/use/current/bind/unbind）。
- 同一 bot 并发多项目无 workspace 串线。
- `/new` 语义未退化。
- 意图切换必须确认后才生效。
