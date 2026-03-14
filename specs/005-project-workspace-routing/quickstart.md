# 快速启动：项目空间隔离与路由

## 目标
完成 Phase 5 全量验收闭环：
- 项目注册、绑定、切换与审计命令可用
- 同一 bot 并发多项目隔离稳定
- `/new` 语义不回退（仅新建会话）
- 意图切换遵循 confirm-first
- SC-001 ~ SC-006 指标可采集并可门禁

## 开发前准备
1. 确认分支：`005-project-workspace-routing`
2. 阅读：
   - `docs/plans/phase_5_project_workspace_routing.md`
   - `specs/005-project-workspace-routing/spec.md`
   - `specs/005-project-workspace-routing/plan.md`
   - `specs/005-project-workspace-routing/contracts/project-command-contract.md`
   - `specs/005-project-workspace-routing/contracts/project-routing-contract.md`
3. 基础命令可运行：
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

## 手工验收脚本（按故事）

### A. US1 显式管理项目与 workspace
1. 执行 `/project create bid Bid`。
2. 执行 `/project create nba NBA`。
3. 执行 `/project list`。
4. 在目标 route 执行 `/project use bid`。
5. 执行 `/project current`。

预期：
- `~/.clawx/workspaces/bid` 与 `~/.clawx/workspaces/nba` 已创建。
- `projects.json` 可见 `main/bid/nba`。
- `current` 返回 `bid` 与 `mode=binding`。

### B. US2 同一 bot 并发多项目隔离
1. 在 route key A 执行 `/project use bid`。
2. 在 route key B 执行 `/project use nba`。
3. A/B 各执行一次 `/new` 并发送普通任务。
4. A/B 分别执行 `/project current`。

预期：
- A 绑定并命中 `bid`，B 绑定并命中 `nba`。
- 两侧会话 ID 与窗口绑定不会互相覆盖。
- `/new` 不会创建新项目，不会改写 route 绑定。

### C. US3 意图辅助切换（confirm-first）
1. 在 A（当前 `bid`）发送明显属于 `nba` 的任务。
2. 收到建议后，记录 `/project confirm <proposal_id>`。
3. 执行确认命令并再次请求。
4. 重新执行 `/project current`。

预期：
- 建议阶段不会自动切换项目。
- 仅确认后 route 绑定更新到 `nba`。
- 提案过期后确认会失败（可用过期测试复核）。

### D. US4 可观测与恢复
1. 执行 `/project audit`，确认统计输出存在。
2. 执行 `/project bind <route_key> bid` 后再 `/project unbind <route_key>`。
3. 触发项目异常后执行 `/project repair <project_id>`。
4. 验证 `/project delete <project_id>` 在有活动绑定或活动会话时被保护。

预期：
- 审计输出包含 `projects/active/broken/bindings/broken_bindings`。
- bind/unbind 生效并可回退到 default project。
- repair 能从 `broken` 恢复到 `active`。
- delete 默认安全保护生效，`--force` 才可强删。

## 自动化回归

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

建议重点回归：
- `tests/unit/project_*`
- `tests/integration/project_*`
- `tests/contract/project_*`

## 指标采集与门禁（SC-001~SC-006）

### 1) 合成数据门禁测试

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
  go test ./tests/integration -run TestPhase5ProjectRoutingMetricsReportGateWithSyntheticDataset
```

### 2) 真实日志门禁测试（可选）
准备 JSONL（字段：`timestamp`、`sc_id`，以及 `success` 或 `latency_ms`）。

```bash
export CLAWX_PHASE5_METRICS_JSONL=/path/to/phase5_metrics.jsonl
export CLAWX_PHASE5_METRICS_REPORT=docs/guides/phase_5/phase_5_metrics_report.md
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
  go test ./tests/integration -run TestPhase5ProjectRoutingMetricsReportFromJSONL
```

门禁规则：
- SC-001 >= 95%
- SC-002 = 100%
- SC-003 >= 95%
- SC-004 = 100%
- SC-005 = 100%
- SC-006 p95 < 120ms

## 完成检查
- `/project` 命令闭环可用（create/list/use/current/bind/unbind/audit/delete/repair/confirm）。
- 同一 bot 并发多项目无 workspace 串线。
- `/new` 语义未退化。
- 意图切换必须确认后才生效。
- 指标门禁测试可输出报告并用于发布拦截。
