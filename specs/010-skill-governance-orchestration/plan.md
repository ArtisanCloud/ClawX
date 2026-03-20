# Implementation Plan: 010-skill-governance-orchestration

**Branch**: `010-skill-governance-orchestration` | **Date**: 2026-03-19 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/spec.md](/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/spec.md)
**Input**: Feature specification from `/specs/010-skill-governance-orchestration/spec.md`

## Summary

为 ClawX 建立统一 Skill Control Plane：通过 Skill Registry/Policy/Binding 管理内建与第三方技能，采用自然语言 LLM-first 路由生成结构化 SkillAction，并统一进入安全执行、确认门禁与审计闭环；同时保持命令入口可用但不再作为主交互路径。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 Router/Intent 管线、现有配置与持久化组件、现有 `skills` 基础能力  
**Storage**: 本地文件存储（`~/.clawx/state/skills/` 元数据与绑定）+ 结构化审计日志 + 会话内上下文摘要缓存  
**Testing**: `go test ./...` + `tests/contract`、`tests/integration`、`tests/unit`  
**Target Platform**: Linux server  
**Project Type**: Go 后端服务（DDD 分层 + 聊天控制面）  
**Performance Goals**: 自然语言技能路由 p95 < 200ms（不含外部技能执行时间）；skill catalog 200 条规模下摘要构建中位数 < 300ms  
**Constraints**: 自然语言默认走 LLM-first；高风险技能必须 confirm-first；命令入口映射到同一 action schema；低置信度不得直接执行；禁止演化为“通用插件市场”模式（仅白名单源 + 受限能力）  
**Scale/Scope**: 基线 5-20 并发会话；每 agent 可绑定数十到上百技能

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Phase 0 前检查

| Gate | 检查项 | 结果 |
|------|--------|------|
| I. Session-First Architecture | 技能路由与执行不替代会话管理，仍按会话隔离上下文与确认状态 | PASS |
| II. Agent as Runtime Template | 技能绑定围绕 agent/project/global 三层，不改变 Agent 模板定位 | PASS |
| III. Direct Execution Before Heavy Orchestration | 采用轻量 Skill Router + Executor，不引入分布式编排平台 | PASS |
| IV. Stable Boundaries and Replaceable Adapters | 注册、策略、绑定、路由、执行、审计边界清晰且可替换 | PASS |
| V. Docs Drive Scope | 范围由 `docs/plans/phase_7_agent_skill_governance_orchestration.md` 与 010 spec 驱动 | PASS |
| VI. Reference Is Input, Never Authority | 第三方 skill 市场仅作输入源，最终行为以本地策略与 schema 校验为准 | PASS |
| VII. Go and DDD by Default | 沿用 Go + DDD 分层实现，避免跨层耦合 | PASS |

### Phase 1 后复检

| Gate | 复检结论 |
|------|----------|
| I ~ VII | 设计产物（research/data-model/contracts/quickstart）需继续对齐上述约束，目标 PASS |

## Project Structure

### Documentation (this feature)

```text
specs/010-skill-governance-orchestration/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── skill-registry-policy-contract.md
│   ├── skill-routing-action-contract.md
│   └── skill-binding-resolution-contract.md
└── tasks.md               # 由 /speckit.tasks 生成
```

### Source Code (repository root)

```text
cmd/
└── clawx/
    ├── main.go
    └── (chat/control handlers)

internal/
├── application/
│   ├── intent/
│   ├── service/
│   └── skillorchestrator/     # 新增：路由与执行编排
├── domain/
│   └── skill/                 # 新增：元数据/策略/绑定/执行记录模型
├── infrastructure/
│   ├── skills/
│   ├── persistence/
│   └── logging/
└── interfaces/
    └── chat/

tests/
├── contract/
├── integration/
└── unit/
```

**Structure Decision**: 在现有单体 Go 架构内引入 `skillorchestrator` 与 `domain/skill`，复用现有 intent 管线与技能基础设施，不新增独立服务进程；技能控制入口不耦合 `config_chat`，通过独立 skill 控制处理器接入。

## LLM-First Routing Addendum

- 自然语言请求先进入 `Skill Intent Router`，输入为 `message + context_digest + skill_catalog_digest`。
- LLM 输出统一结构化 `SkillAction`，必须通过 schema 与 policy 校验后才能进入执行。
- 命令请求（如 `/skill ...`）先解析，再映射为同一 `SkillAction`，与自然语言共享执行链路。
- 当 `confidence` 低于阈值时，不执行；进入澄清流程并记录 `routing_rejected_low_confidence` 审计事件。
- 阈值配置键为 `CLAWX_SKILL_ROUTER_CONFIDENCE_THRESHOLD`，默认值 `0.70`，有效范围 `0.0~1.0`。

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| 引入 LLM 路由层 | 自然语言主路径必须由语义识别驱动，避免命令心智负担 | 纯规则解析无法覆盖复杂语义且扩展成本高 |
| 引入三层绑定解析 | 满足全局共享与局部隔离并存需求 | 单层绑定无法支持多 agent 实际协作场景 |
