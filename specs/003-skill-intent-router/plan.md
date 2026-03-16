# 实现计划：Skill Registry 与意图路由

**分支**: `003-skill-intent-router` | **日期**: 2026-03-08 | **规格**: `specs/003-skill-intent-router/spec.md`  
**输入**: 来自 `specs/003-skill-intent-router/spec.md` 的功能规格说明

## 摘要

本功能在现有 `Router -> Session Manager -> Backend Adapter` 主链路上引入 Skill Registry 与 Intent Router，使系统可以复用 Claude Code 规范 Skill（`SKILL.md`），并把入站消息稳定分流到控制命令、Skill 执行或普通任务。  
关键策略已经澄清并固化：默认“规则优先 + LLM 兜底”、多候选自动选择（不二次询问用户）、同名 Skill 按来源优先级生效并将其余标记为 `shadowed`、默认权限为“频道白名单 + DM pairing”。

## 技术上下文

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 ClawX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化组件  
**Storage**: 本地文件存储（`~/.clawx`）+ 现有会话存储；Skill 索引以文件缓存形式维护  
**Testing**: Go 原生 `testing`（单元/集成/契约）+ `go test ./...`  
**Target Platform**: Linux 服务器  
**Project Type**: 聊天驱动后端服务 / CLI 控制网关  
**Performance Goals**: 意图路由判定 p95 < 100ms（不含后端执行时间）；Skill 刷新在 1 秒内对新请求生效  
**Constraints**: 同一 Session 串行执行；控制命令优先级最高；多候选自动选单一 Skill；不得要求用户二次选择 Skill  
**Scale/Scope**: 单实例 5-20 并发会话；每个工作区最多约 200 个可用 Skill；支持 Discord 与 Telegram 双渠道

## 宪章检查

*门禁：必须在 Phase 0 研究前通过；Phase 1 设计完成后再次检查。*

- Session-First：通过。Skill 路由只决定“请求如何进入执行链路”，不改变 Session 作为执行与隔离主单位。
- Agent 作为模板：通过。未引入“Agent 替代 Session”的行为，仍由 Session 承载运行态上下文。
- 直连执行链路：通过。Skill 路径仍复用现有 `Router -> Session Manager -> Backend Adapter`。
- 稳定边界：通过。Skill Registry、Intent Router、Channel Adapter、Backend Adapter 责任边界明确。
- Docs 驱动范围：通过。实现范围来源于 `docs/features/skill_registry_intent_router/*` 与本分支规格。
- Reference 限制：通过。参考仅用于对比，不作为产品规范来源。
- Go + DDD：通过。新增能力按 `domain/application/infrastructure/interfaces` 分层设计。
- “不做重型插件系统”约束：通过。本功能是受限 Skill 兼容与路由，不包含通用插件运行时、中台或第三方代码执行沙箱。

当前无需要豁免的宪章违例。

### Phase 1 设计后复核

- 复核结论：通过。
- `research.md`、`data-model.md`、`contracts/` 与 `quickstart.md` 未引入宪章违例项。
- 多候选自动选择、权限默认策略与日志审计要求均已在契约与数据模型中显式约束。

## 项目结构

### 文档（本功能）

```text
specs/003-skill-intent-router/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── intent-router-contract.md
│   └── skill-registry-contract.md
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
│   │   └── skill/                  # 新增：Skill 领域模型与校验规则
│   ├── application/
│   │   ├── service/                # 现有 Router/SessionManager
│   │   ├── intent/                 # 新增：意图路由编排
│   │   └── skillregistry/          # 新增：Skill 注册与索引刷新
│   ├── infrastructure/
│   │   ├── persistence/
│   │   ├── config/
│   │   └── skills/                 # 新增：文件系统扫描与索引落盘
│   └── interfaces/
│       └── chat/
│           ├── discord/
│           └── telegram/
└── tests/
    ├── unit/
    ├── integration/
    └── contract/
```

**结构决策**: 采用单服务 Go 工程结构，在不拆分现有运行链路的前提下增量扩展 `skill` 与 `intent` 相关模块。领域规则沉淀在 `internal/domain/skill`，路由与注册编排放在 `internal/application`，文件扫描和索引持久化放在 `internal/infrastructure/skills`，渠道输入输出仍经 `internal/interfaces/chat` 进入统一 Router。

## 复杂度跟踪

当前无已批准的复杂度豁免。

## 阶段交付摘要

- 已交付 Skill Registry 主链路：扫描、解析、冲突处理、状态落盘、刷新生效。
- 已交付 Intent Router 主链路：控制命令优先、显式 `/skill`、规则匹配、LLM 兜底、阈值回退。
- 已交付治理能力：`enable/disable`、默认权限检查、拒绝错误分类、DM pairing 生命周期存储。
- 已交付 CLI：`clawx skill list|reload|enable|disable`。
- 已交付测试覆盖：新增 7 个集成测试与 2 个契约测试，`go test ./...` 通过。

## 风险清单（当前）

- LLM 兜底目前为启发式实现，后续接入真实模型后需重新标定阈值与误判率。
- 权限默认策略中 DM pairing 流程已具备状态机，但尚未提供用户引导式“配对命令”交互。
- 内置 Skill 目录当前默认路径为 `internal/skills/builtin`，上线前需明确随发布包的分发方式。
