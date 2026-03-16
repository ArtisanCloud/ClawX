# 实现计划：第一阶段基座能力

**分支**: `001-phase1-foundation` | **日期**: 2026-03-03 | **规格**: `specs/001-phase1-foundation/spec.md`
**输入**: 来自 `specs/001-phase1-foundation/spec.md` 的功能规格说明

## 摘要

本阶段实现 ClawX 的最小可运行基座：通过直连执行链路将受支持聊天渠道中的请求映射到受控执行会话，保证单窗口单会话、会话互斥、超时取消、长输出分段，以及 Discord 与 Telegram 的基础接入能力。技术方案采用 Go 语言，并按 DDD 分层组织代码，优先建立稳定的 `Router -> Session Manager -> Backend Adapter` 主链路，再补齐渠道适配、输出处理与运维基线。

## 技术上下文

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、`discordgo`、Telegram Bot SDK（Go）、`gopkg.in/yaml.v3`  
**Storage**: 内存态会话存储 + 环境变量 / YAML 配置文件  
**Testing**: Go 原生 `testing` + 集成测试 + 契约测试  
**Target Platform**: Linux 服务器  
**Project Type**: 聊天驱动的后端服务 / CLI 控制网关  
**Performance Goals**: 5 秒内进入可见处理状态；支持 5-20 个并发会话；长输出按渠道限制有序送达  
**Constraints**: 同一会话串行；单次执行默认上限 10 分钟；执行目录必须位于允许范围内；Discord 单条 2000 字符；Telegram 单条 4096 字符  
**Scale/Scope**: 单实例部署；双渠道接入；第一阶段仅覆盖基座能力，不实现多窗口、多 Agent、自动路由

## 宪章检查

*门禁：必须在 Phase 0 研究前通过；Phase 1 设计完成后再次检查。*

- `Session` 优先：通过。所有状态化执行均围绕 `Session` 建模，不引入 `Agent` 作为前置概念。
- `Agent` 作为模板：通过。第一阶段只预留兼容字段，不实现 `Agent Registry` 或路由规则。
- 直连执行默认：通过。主链路固定为 `Router -> Session Manager -> Backend Adapter`。
- Go 优先：通过。计划采用 Go 1.23 实现核心运行模块。
- DDD 默认：通过。代码按 `Domain / Application / Infrastructure / Interface` 分层。
- 文档驱动范围：通过。范围来源于宪章、Phase 1 计划与功能规格；不依赖参考资料作为正式规范源。
- 参考资料限制：通过。若需要研究外部模式，仅作为补充输入，不直接转化为实现规则。

当前无需要豁免的宪章违例。

## 项目结构

### 文档（本功能）

```text
specs/001-phase1-foundation/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── channel-command-contract.md
│   └── health-contract.md
└── tasks.md              # 由 /speckit.tasks 后续生成
```

### 源码（仓库根目录）

```text
.
├── cmd/
│   └── clawx/
│       └── main.go
├── internal/
│   ├── domain/
│   │   ├── session/
│   │   ├── execution/
│   │   └── conversation/
│   ├── application/
│   │   ├── command/
│   │   ├── service/
│   │   └── dto/
│   ├── infrastructure/
│   │   ├── backend/
│   │   ├── persistence/
│   │   ├── config/
│   │   ├── logging/
│   │   └── health/
│   └── interfaces/
│       ├── chat/
│       │   ├── discord/
│       │   └── telegram/
│       └── admin/
└── tests/
    ├── unit/
    ├── integration/
    └── contract/
```

**结构决策**: 采用单服务仓库结构，以 Go 的 `cmd/` + `internal/` 组织代码。领域规则放入 `internal/domain/`，用例编排放入 `internal/application/`，后端执行、持久化、配置和日志放入 `internal/infrastructure/`，聊天输入输出、渠道适配与健康接口等交互边界放入 `internal/interfaces/`。该结构直接匹配宪章中的 Go + DDD 约束，并为后续阶段扩展多窗口、多会话和多 Agent 保留清晰边界。

## 复杂度跟踪

当前无已批准的复杂度豁免。
