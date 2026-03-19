# Implementation Plan: 009-config-intent-plan

**Branch**: `009-config-intent-plan` | **Date**: 2026-03-18 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/spec.md](/home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/spec.md)
**Input**: Feature specification from `/specs/009-config-intent-plan/spec.md`

## Summary

为 ClawX 建立“配置意图统一控制面”：让 `/config` 指令与自然语言都能进入同一 pending plan 生命周期，支持会话内 patch 编辑、confirm-first 应用、权限分层与审计闭环；同时保证普通任务链路不被配置语义回归影响。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 `cmd/clawx/config_chat.go` 配置链路、现有 Router/Intent 管线、现有文件配置持久化模块  
**Storage**: 内存态 pending plan（按会话隔离）+ 控制面结构化摘要缓存（会话级）+ 本地配置文件写入（`~/.clawx/config.json`）+ 结构化日志审计  
**Testing**: `go test ./...` + `tests/contract`、`tests/integration`、`tests/unit`  
**Target Platform**: Linux server
**Project Type**: Go 后端服务（DDD 分层 + 聊天控制面）  
**Performance Goals**: 配置意图判定与路由 p95 < 120ms；会话内“创建计划到 apply”中位步骤 <= 4（见 SC-006）  
**Constraints**: 禁止隐式写盘；仅 `/config apply` 可落盘；非管理员不可 apply；保持现有 `/agent` 与普通任务链路语义稳定；上下文压缩必须“结构化可回放”且不可替代原始 patch 历史  
**Scale/Scope**: 基线 5-20 并发会话；单会话计划 patch 操作在百次内保持稳定一致

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Phase 0 前检查

| Gate | 检查项 | 结果 |
|------|--------|------|
| I. Session-First Architecture | pending plan 作用域按会话隔离（channel+instance+conversation），不以 Agent 替代会话 | PASS |
| II. Agent as Runtime Template | 新增能力仅增强配置控制面，不改变 Agent 作为模板角色 | PASS |
| III. Direct Execution Before Heavy Orchestration | 在现有链路前增加轻量配置意图桥，不引入重编排层 | PASS |
| IV. Stable Boundaries and Replaceable Adapters | 配置解析、意图判定、计划存储、写盘执行保持边界 | PASS |
| V. Docs Drive Scope | 范围来自 `docs/plans/phase_6_agent_context_intent_unification.md` 与 009 spec | PASS |
| VI. Reference Is Input, Never Authority | 方案基于本仓库计划与规格，不直接复制外部规则 | PASS |
| VII. Go and DDD by Default | 采用 Go 与现有 DDD 分层，不把域规则散落到适配层 | PASS |

### Phase 1 后复检

| Gate | 复检结论 |
|------|----------|
| I ~ VII | 设计产物（research/data-model/contracts/quickstart）均符合宪章约束，继续 PASS |

## Project Structure

### Documentation (this feature)

```text
specs/009-config-intent-plan/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── config-intent-contract.md
│   └── config-plan-lifecycle-contract.md
└── tasks.md               # 由 /speckit.tasks 生成
```

### Source Code (repository root)

```text
cmd/
└── clawx/
    ├── config_chat.go
    └── main.go

internal/
├── application/
│   ├── configplan/
│   │   ├── summary.go
│   │   └── projector.go
│   ├── intent/
│   ├── command/
│   └── service/
├── domain/
│   └── session/
├── infrastructure/
│   ├── config/
│   └── logging/
└── interfaces/
    └── chat/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: 保持单体 Go 服务结构，在现有配置命令链路上扩展“配置意图桥接 + 计划补丁 + 审计事件”，不引入新进程或新持久化中间件。

## Context Compression Addendum

- 控制面摘要由 `configplan` 内部维护，输入为 append-only patch 历史，输出为确定性结构化摘要（字段最终值/最近修改元数据/patch 计数）。
- `/config show` 默认展示摘要与最近轨迹窗口（例如最近 5 条），并提供摘要版本号用于比对。
- 摘要仅用于“判定辅助与展示优化”，不可替代原始 patch 历史；apply 前一律以原始历史回放结果为准。
- 当检测到摘要失效（版本不一致/字段缺失）时，触发摘要重建并记录审计事件。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | 无宪章违规 | N/A |
