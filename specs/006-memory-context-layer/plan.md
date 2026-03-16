# Implementation Plan: 006-memory-context-layer

**Branch**: `006-memory-context-layer` | **Date**: 2026-03-14 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/spec.md](/home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/spec.md)
**Input**: Feature specification from `/specs/006-memory-context-layer/spec.md`

## Summary

为 ClawX 增加项目级与 agent 级文件化记忆层，在新会话首轮执行前完成分层加载，默认写回到 `agent+project` 私有层，并通过主会话 ACL、审计记录与模板版本治理保证隔离、安全和可运维。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、`gopkg.in/yaml.v3`、现有 Discord/Telegram/Feishu/WeCom 适配层  
**Storage**: 本地文件存储（`~/.clawx/workspaces/<project_id>/` 与 `~/.clawx/workspaces/<project_id>/.agents/<agent_id>/`）+ 现有 `~/.clawx/projects/*.json` 持久化  
**Testing**: `go test ./...` + `tests/unit`、`tests/integration`、`tests/contract` 的 memory 覆盖  
**Target Platform**: Linux server  
**Project Type**: Go 后端服务（DDD 分层）  
**Performance Goals**: 记忆加载阶段 p95 额外开销 < 120ms（SC-006）；新会话首轮加载成功率 >= 95%（SC-002）  
**Constraints**: 不破坏 `/new`、`/resume`、`/switch`、`/list`、`/current`、`/cancel` 语义；共享会话默认拒绝长期私有记忆；加载失败需可降级并可追踪  
**Scale/Scope**: 基线 5-20 并发会话；7 天滚动窗口每项指标不少于 200 次有效样本

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Phase 0 前检查

| Gate | 检查项 | 结果 |
|------|--------|------|
| I. Session-First Architecture | 记忆加载绑定 `session + route + project`，不以 Agent 替代 Session | PASS |
| II. Agent as Runtime Template | Agent 仅提供默认与私有层边界，不承担会话隔离主职责 | PASS |
| III. Direct Execution Before Heavy Orchestration | 保持 `Router -> Session Manager -> Backend Adapter`，仅增加轻量 `MemoryLoader` 前置步骤 | PASS |
| IV. Stable Boundaries and Replaceable Adapters | ACL/加载逻辑在应用层，文件读写在基础设施层，渠道层仅消费结果 | PASS |
| V. Docs Drive Scope | 以 `specs/006-memory-context-layer/spec.md` 与本计划为交付边界 | PASS |
| VI. Reference Is Input, Never Authority | 外部参考仅作为启发，规则已转译为 ClawX 本地约束 | PASS |
| VII. Go and DDD by Default | 实现保持 Go + DDD 分层，不引入越层依赖 | PASS |

### Phase 1 后复检

| Gate | 复检结论 |
|------|----------|
| I ~ VII | 设计产物（research/data-model/contracts/quickstart）均未引入宪章冲突，继续 PASS |

## Project Structure

### Documentation (this feature)

```text
specs/006-memory-context-layer/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── memory-command-contract.md
│   └── memory-loading-contract.md
└── tasks.md               # 由 /speckit.tasks 生成
```

### Source Code (repository root)

```text
cmd/
└── clawx/

internal/
├── application/
│   ├── command/
│   ├── memory/
│   ├── project/
│   └── service/
├── domain/
│   ├── memory/
│   ├── project/
│   └── session/
├── infrastructure/
│   ├── backend/
│   └── persistence/
└── interfaces/
    └── chat/

tests/
├── unit/
├── integration/
└── contract/
```

**Structure Decision**: 采用单体 Go 后端结构，在既有 DDD 边界内新增 memory 相关领域对象、应用编排和持久化实现，不新增独立服务。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | 无宪章违规 | N/A |
