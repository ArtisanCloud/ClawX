# 实现计划：项目空间隔离与路由

**分支**: `005-project-workspace-routing` | **日期**: 2026-03-13 | **规格**: `/home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/spec.md`  
**输入**: 功能规格来自 `/home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/spec.md`

## 摘要

本阶段在现有 `channel -> window -> session` 路由之上新增 `project` 维度，实现“同一 bot 并行多项目”的可控隔离。核心是三件事：
1. 项目注册表与 workspace 映射；
2. route key 到项目的绑定；
3. `/project` 命令与意图切换建议（confirm-first）闭环。

设计原则：`/new` 只创建会话，不承担项目切换；项目切换必须显式或确认后执行；同一 main agent 可服务多个项目，但项目目录与会话索引必须隔离。

## 技术上下文

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 ClawX DDD 模块、现有 Router/Session Manager、现有 config/persistence 组件  
**Storage**: 本地文件（`~/.clawx/projects/*.json` + `~/.clawx/workspaces/<project_id>`）  
**Testing**: Go `testing`（unit/integration/contract），`go test ./...`  
**Target Platform**: Linux server / bot runtime  
**Project Type**: 聊天驱动后端服务 / CLI 控制网关  
**Performance Goals**: 项目判定路径 p95 < 120ms（不含后端执行）  
**Constraints**: 兼容 Phase 2 窗口语义；控制命令不回归；文件写入需原子化  
**Scale/Scope**:
- 支持 1 个 agent 下 3-20 个项目并发
- 每项目支持独立 workspace、独立会话索引
- route key 至少覆盖 `channel/instance/peer/thread`

## 宪章检查

*门禁：必须在 Phase 0 研究前通过；Phase 1 设计完成后再次检查。*

- Session-First：通过。项目维度叠加在会话索引，不替代会话。
- Agent 作为模板：通过。同一 agent 可承载多项目，不引入重型多 agent 编排。
- 直连执行链路：通过。新增项目解析后仍进入既有 Router 与 Session Manager。
- 稳定边界：通过。渠道层只负责 route key 归一化；项目判定在应用层。
- Docs 驱动范围：通过。范围来自本 spec 与 Phase 5 计划文档。
- Go + DDD：通过。新增模块落在 `application/domain/infrastructure/interfaces` 分层内。

当前无需要豁免的宪章违例。

### Phase 1 设计后复核

- 复核结论：通过。
- `research.md` 明确了 route key 粒度、`/new` 与 `/project` 边界、意图切换策略。
- `data-model.md` 明确了 registry/binding/proposal 实体与约束。

## 项目结构

### 文档（本功能）

```text
/home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── spec.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### 源码（仓库根目录）

```text
/home/ubuntu/workspace/ClawX/
├── cmd/
│   └── clawx/
│       ├── main.go
│       ├── project.go                 # 新增：/project 命令入口
│       └── config.go
├── internal/
│   ├── application/
│   │   ├── service/
│   │   │   ├── router.go
│   │   │   └── project_router.go      # 新增：项目判定编排
│   │   └── project/                   # 新增：项目应用服务
│   ├── domain/
│   │   └── project/                   # 新增：Project/Binding/Proposal 领域模型
│   ├── infrastructure/
│   │   ├── config/
│   │   └── persistence/
│   │       └── project_file_store.go  # 新增：registry/bindings/proposals 持久化
│   └── interfaces/
│       └── chat/
│           └── normalize.go           # route key 归一化补充
└── tests/
    ├── unit/
    ├── integration/
    └── contract/
```

**结构决策**: 延续单服务 Go + DDD；项目相关状态先用文件存储，不引入数据库。`/project` 命令与自动建议均复用同一应用服务，避免双逻辑分叉。

## 里程碑

- M1: 项目注册表 + route binding + `/project create/list/use/current`。
- M2: 会话键纳入 `project_id`，`/new` 语义锁定“当前项目内新会话”。
- M3: 意图切换建议（confirm-first）+ 审计与恢复命令。

## 风险清单（当前）

- 路由键归一化不稳定会导致误绑定。
- 并发写 `registry/bindings` 可能产生竞态。
- 意图切换建议过于频繁会干扰用户操作。
- 老数据迁移时缺少 `project_id` 会导致回退路径不一致。
