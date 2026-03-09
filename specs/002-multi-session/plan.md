# 实现计划：第二阶段多窗口多会话

**分支**: `002-multi-session` | **日期**: 2026-03-09 | **规格**: `/home/ubuntu/workspace/SynapseX/specs/002-multi-session/spec.md`
**输入**: 功能规格来自 `/home/ubuntu/workspace/SynapseX/specs/002-multi-session/spec.md`

## 摘要

本阶段将 SynapseX 从单会话倾向升级为多窗口多会话默认模型：输入优先按 `window_id` 命中当前会话，在缺省 `window_id` 时回退到兼容路径，确保既有入口不回退。实现保持现有 `Router -> Session Manager -> Backend Adapter` 直连链路，新增窗口绑定能力、窗口级会话列表与切换语义，并固定内建控制命令优先级。

## 技术上下文

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 SynapseX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化模块  
**Storage**: 内存会话仓储（已存在）+ 计划新增窗口绑定持久化结构（内存优先，兼容后续文件/数据库扩展）  
**Testing**: Go 原生 `testing`（unit/integration/contract）+ `go test ./...`  
**Target Platform**: Linux server  
**Project Type**: 聊天驱动后端服务 / CLI 控制网关  
**Performance Goals**: 路由与会话决策保持当前量级；窗口绑定读写需为常数级或近似常数级  
**Constraints**: 同一 Session 串行执行、超时与取消语义不变、缺省无 `window_id` 入口必须可用、内建控制命令优先于技能/自然语言路径  
**Scale/Scope**: 单实例 5-20 并发会话；覆盖 Discord/Telegram 入口与统一消息接口

## 宪章检查

*门禁：必须在 Phase 0 研究前通过；Phase 1 设计完成后再次检查。*

- Session-First：通过。窗口模型仅决定会话选择，不引入 Agent 作为前置依赖。
- Agent 作为模板：通过。本阶段不引入 Agent Registry、Binding 规则引擎。
- 直连执行优先：通过。保留 `Router -> Session Manager -> Backend Adapter`。
- 稳定边界：通过。窗口状态由会话仓储侧承载，渠道适配器只做上下文归一化。
- Docs 驱动范围：通过。范围来源于本规格与 `docs/plans/phase_2_multi_session.md`。
- Go + DDD：通过。所有改动落在既有分层边界内。

当前无需豁免的宪章违例。

### Phase 1 设计后复核

- 复核结论：通过。
- `research.md` 已消除关键实现歧义（窗口绑定模型、兼容策略、命令语义）。
- `data-model.md`、`contracts/` 与 `quickstart.md` 未引入宪章冲突项。

## 项目结构

### 文档（本功能）

```text
/home/ubuntu/workspace/SynapseX/specs/002-multi-session/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── window-routing-contract.md
│   └── control-command-window-contract.md
├── checklists/
│   └── requirements.md
└── tasks.md              # 由 /speckit.tasks 生成
```

### 源码（仓库根目录）

```text
/home/ubuntu/workspace/SynapseX/
├── cmd/
│   └── synapsex/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── conversation/
│   │   ├── execution/
│   │   └── session/
│   ├── application/
│   │   ├── command/
│   │   └── service/
│   ├── infrastructure/
│   │   ├── backend/
│   │   ├── config/
│   │   ├── health/
│   │   ├── logging/
│   │   └── persistence/
│   └── interfaces/
│       ├── admin/
│       └── chat/
│           ├── discord/
│           └── telegram/
└── tests/
    ├── unit/
    ├── integration/
    └── contract/
```

**结构决策**: 沿用单服务 Go + DDD 结构，不新增重型中间层。多窗口能力由 `session` 领域与仓储扩展承载，路由层按窗口优先规则做决策，渠道层仅承接 `window_id` 输入与兼容值生成。

## 复杂度跟踪

当前无已批准的复杂度豁免。
