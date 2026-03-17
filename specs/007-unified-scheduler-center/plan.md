# Implementation Plan: 007-unified-scheduler-center

**Branch**: `007-unified-scheduler-center` | **Date**: 2026-03-17 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/spec.md](/home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/spec.md)
**Input**: Feature specification from `/specs/007-unified-scheduler-center/spec.md`

## Summary

为 ClawX 增加统一定时任务中心，支持 `/schedule` 全生命周期管理、自然语言注册“每周清理图片记录”、按 `project+agent` 默认隔离、执行可观测与重启恢复。默认策略为每周日 03:00 执行、保留最近 30 天、失败记录后等待下周期。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、现有 Router/Intent 管线、现有本地文件持久化组件  
**Storage**: 本地文件存储（`~/.clawx/workspaces/<project_id>/.agents/<agent_id>/scheduler/`）  
**Testing**: `go test ./...` + `tests/contract`、`tests/integration`、`tests/unit`  
**Target Platform**: Linux server
**Project Type**: Go 后端服务（DDD 分层）  
**Performance Goals**: 周期任务触发准时率 >= 95%（计划后 1 分钟内开始）；任务查询在常规规模下保持用户可接受响应（SC-003/SC-004）  
**Constraints**: 不破坏现有 `/new`、`/resume`、`/memory`、`/service` 语义；单任务同一时刻仅允许一个活动执行；单任务失败不得阻塞其他任务  
**Scale/Scope**: 基线 5-20 并发会话；同项目同 agent 支持数十级别定时任务并发调度

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Phase 0 前检查

| Gate | 检查项 | 结果 |
|------|--------|------|
| I. Session-First Architecture | 调度能力作为控制命令与后台 runner，不替代 Session，会话执行语义不变 | PASS |
| II. Agent as Runtime Template | 默认隔离基于 `project+agent`，符合 Agent 作为运行时边界模板 | PASS |
| III. Direct Execution Before Heavy Orchestration | 在现有 `Router -> Session Manager -> Backend Adapter` 主链外增加轻量 scheduler 服务，不引入重编排层 | PASS |
| IV. Stable Boundaries and Replaceable Adapters | 命令解析、应用服务、持久化、渠道映射分别落在既有分层 | PASS |
| V. Docs Drive Scope | 功能范围已在 `docs/plans/feature/007-unified-scheduler-center/plan.md` 与 spec 明确 | PASS |
| VI. Reference Is Input, Never Authority | 方案以 ClawX 本地文档和 spec 为准，未把 reference 作为产品真相 | PASS |
| VII. Go and DDD by Default | 采用 Go + DDD 分层实现，不跨层注入业务规则 | PASS |

### Phase 1 后复检

| Gate | 复检结论 |
|------|----------|
| I ~ VII | 设计产物（research/data-model/contracts/quickstart）均符合宪章约束，继续 PASS |

## Project Structure

### Documentation (this feature)

```text
specs/007-unified-scheduler-center/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── schedule-command-contract.md
│   └── schedule-runtime-contract.md
└── tasks.md               # 由 /speckit.tasks 生成
```

### Source Code (repository root)

```text
cmd/
└── clawx/

internal/
├── application/
│   ├── command/
│   ├── scheduler/
│   └── service/
├── domain/
│   └── scheduler/
├── infrastructure/
│   └── persistence/
└── interfaces/
    └── chat/

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: 采用现有单体 Go 服务结构，在 DDD 边界内新增 scheduler 领域、应用编排、文件仓储与控制命令接入，不新增独立运行进程。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | 无宪章违规 | N/A |
