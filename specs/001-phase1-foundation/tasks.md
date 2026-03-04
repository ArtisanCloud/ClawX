# 任务清单：第一阶段基座能力

**输入**: `specs/001-phase1-foundation/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本阶段不强制采用测试先行流程，但每个用户故事都必须满足可独立验证条件；测试相关补充任务放在收尾阶段完成。

**组织方式**: 任务按用户故事分组，保证每个故事都可以独立实现、独立验证、独立交付。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（如 `US1`、`US2`、`US3`）
- 每个任务描述都包含明确文件路径

## 路径约定

- 仓库根目录实现使用：`cmd/`、`internal/`、`tests/`
- 按实现计划中的 Go + DDD 结构组织代码

## Phase 1：初始化（共享基础）

**目的**: 建立第一阶段实现所需的 Go 项目骨架与基础目录

- [X] T001 创建 Go 入口与基础目录结构于 cmd/synapsex/、internal/、tests/
- [X] T002 初始化 Go 模块并声明基础依赖于 go.mod
- [X] T003 [P] 创建应用启动入口骨架于 cmd/synapsex/main.go
- [X] T004 [P] 创建 DDD 分层目录占位文件于 internal/domain/.keep、internal/application/.keep、internal/infrastructure/.keep、internal/interfaces/.keep
- [X] T005 [P] 创建基础测试目录占位文件于 tests/unit/.keep、tests/integration/.keep、tests/contract/.keep

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的核心能力；此阶段完成前不得进入任何用户故事实现

**⚠️ 关键说明**: 本阶段是所有用户故事的阻塞前置能力

- [X] T006 创建会话领域模型与状态规则于 internal/domain/session/session.go
- [X] T007 [P] 创建执行记录领域模型与结果状态规则于 internal/domain/execution/run.go
- [X] T008 [P] 创建对话标识生成规则于 internal/domain/conversation/id.go
- [X] T009 定义会话仓储接口与锁接口于 internal/domain/session/repository.go
- [X] T010 实现内存态会话仓储与会话锁于 internal/infrastructure/persistence/session_memory_repository.go
- [X] T011 [P] 实现配置加载与运行边界校验于 internal/infrastructure/config/config.go
- [X] T012 [P] 实现结构化日志基础设施于 internal/infrastructure/logging/logger.go
- [X] T013 定义后端适配器接口与运行结果接口于 internal/domain/execution/backend.go
- [X] T014 定义统一渠道消息结构与发送接口于 internal/interfaces/chat/message.go
- [X] T015 实现会话管理应用服务于 internal/application/service/session_manager.go
- [X] T016 实现路由应用服务骨架，并加入渠道上下文准入校验与拒绝路径于 internal/application/service/router.go
- [X] T017 实现健康探针基础能力于 internal/infrastructure/health/probe.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - 在受控会话中执行任务（优先级：P1） 🎯 MVP

**目标**: 打通“新建会话 / 继续当前会话 / 恢复会话”的主执行链路

**独立验证**: 通过最小集成入口发起首次请求、继续请求和恢复会话请求，确认系统能在同一受控会话中稳定执行，并返回清晰状态反馈

### 用户故事 1 实现

- [X] T018 [P] [US1] 定义会话命令对象与输入 DTO 于 internal/application/command/session_command.go
- [X] T019 [US1] 实现后端适配器最小直连执行能力于 internal/infrastructure/backend/runner.go
- [X] T020 [US1] 在会话管理服务中实现新建会话逻辑于 internal/application/service/session_manager_create.go
- [X] T021 [US1] 在会话管理服务中实现恢复会话逻辑于 internal/application/service/session_manager_resume.go
- [X] T022 [US1] 在会话管理服务中实现当前会话继续执行逻辑于 internal/application/service/session_manager_continue.go
- [X] T023 [US1] 在路由服务中实现“新建 / 恢复 / 继续 / 会话繁忙拒绝”判定流程于 internal/application/service/router_session_flow.go
- [X] T024 [US1] 在后端适配器中实现超时与取消控制于 internal/infrastructure/backend/timeout.go
- [X] T025 [US1] 将会话主链路接入程序启动入口于 cmd/synapsex/main.go

**检查点**: 此时用户故事 1 应可独立运行并完成 MVP 验证

---

## Phase 4：用户故事 2 - 安全地控制会话状态（优先级：P2）

**目标**: 让用户能够通过标准控制命令安全管理会话状态

**独立验证**: 创建会话后，用户可列出会话、恢复指定会话、取消进行中的执行，并收到明确状态反馈

### 用户故事 2 实现

- [X] T026 [P] [US2] 定义控制命令解析结果与命令类型于 internal/application/command/control_command.go
- [X] T027 [US2] 在路由服务中实现 `/new`、`/resume`、`/list`、`/cancel` 控制命令分流于 internal/application/service/router_control_flow.go
- [X] T028 [US2] 在会话管理服务中实现会话列表查询于 internal/application/service/session_manager_list.go
- [X] T029 [US2] 在会话管理服务中实现取消执行与锁释放流程于 internal/application/service/session_manager_cancel.go
- [X] T030 [US2] 在后端适配器中实现取消执行的底层协作逻辑于 internal/infrastructure/backend/cancel.go
- [X] T031 [US2] 在接口层定义控制命令响应映射于 internal/interfaces/chat/control_response.go

**检查点**: 此时用户故事 1 和用户故事 2 都应可独立完成验证

---

## Phase 5：用户故事 3 - 在受支持渠道中稳定接收输出（优先级：P3）

**目标**: 确保长输出按渠道限制有序送达，并在部分失败时能清晰提示

**独立验证**: 触发长输出后，系统能按顺序分段发送到渠道，并在部分送达失败时明确提示结果可能不完整

### 用户故事 3 实现

- [ ] T032 [P] [US3] 定义输出片段领域对象于 internal/domain/execution/output_segment.go
- [ ] T033 [US3] 实现输出分段器与顺序控制于 internal/application/service/output_streamer.go
- [ ] T034 [P] [US3] 实现 Markdown 与代码块保留策略于 internal/application/service/output_formatter.go
- [ ] T035 [US3] 实现 Discord 渠道适配器 `send_text` / `send_error` 基础发送能力于 internal/interfaces/chat/discord/adapter.go
- [ ] T036 [US3] 实现 Telegram 渠道适配器 `send_text` / `send_error` 基础发送能力于 internal/interfaces/chat/telegram/adapter.go
- [ ] T037 [US3] 在渠道适配层实现统一入站消息归一化于 internal/interfaces/chat/normalize.go
- [ ] T038 [US3] 在输出分段器中实现发送失败重试与部分失败提示于 internal/application/service/output_delivery.go
- [ ] T039 [US3] 将渠道适配器与路由主链路接入程序启动入口于 cmd/synapsex/main.go

**检查点**: 此时三个用户故事都应具备独立可验证能力

---

## Phase 6：收尾与跨领域事项

**目的**: 完成影响多个用户故事的收尾工作，确保第一阶段达到可交付状态

- [ ] T040 [P] 完善统一错误映射与用户可见错误消息于 internal/interfaces/chat/error_response.go
- [ ] T041 完善健康检查接口与后端探针协作于 internal/interfaces/admin/health_handler.go
- [ ] T042 [P] 补充会话、执行、渠道、未授权上下文拒绝、并发互斥与时限反馈集成测试于 tests/integration/phase1_foundation_test.go
- [ ] T043 [P] 补充核心领域单元测试于 tests/unit/session_domain_test.go
- [ ] T044 [P] 补充渠道与健康契约测试于 tests/contract/channel_contract_test.go 和 tests/contract/health_contract_test.go
- [ ] T045 校验 quickstart 文档并同步必要实现说明于 specs/001-phase1-foundation/quickstart.md
- [ ] T046 整理 Phase 1 交付摘要并回写阶段文档于 docs/plans/phase_1_foundation.md

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1（初始化）**: 无依赖，可立即开始
- **Phase 2（基础能力）**: 依赖 Phase 1 完成；阻塞所有用户故事
- **Phase 3-5（用户故事）**: 全部依赖 Phase 2 完成
- **Phase 6（收尾）**: 依赖已完成的用户故事实现

### 用户故事依赖

- **用户故事 1（P1）**: 仅依赖基础能力阶段完成，是 MVP 最小范围
- **用户故事 2（P2）**: 依赖用户故事 1 已具备基本会话主链路
- **用户故事 3（P3）**: 依赖用户故事 1 的主执行链路与用户故事 2 的控制状态能力已可用

### 各故事内部依赖

- 领域对象与接口定义先于应用服务
- 应用服务先于渠道接入与程序装配
- 主流程实现先于收尾测试和文档校验

### 并行机会

- Phase 1 中标记 `[P]` 的任务可并行执行
- Phase 2 中不同领域或基础设施文件的任务可并行执行
- 用户故事内部标记 `[P]` 的模型、格式化、渠道适配任务可并行执行
- 收尾阶段中的测试任务可并行执行

---

## 并行执行示例

### 用户故事 1

```bash
任务: "定义会话命令对象与输入 DTO 于 internal/application/command/session_command.go"
任务: "实现后端适配器最小直连执行能力于 internal/infrastructure/backend/runner.go"
```

### 用户故事 3

```bash
任务: "实现 Discord 渠道适配器 send_text / send_error 基础发送能力于 internal/interfaces/chat/discord/adapter.go"
任务: "实现 Telegram 渠道适配器 send_text / send_error 基础发送能力于 internal/interfaces/chat/telegram/adapter.go"
任务: "实现 Markdown 与代码块保留策略于 internal/application/service/output_formatter.go"
```

---

## 实施策略

### MVP 优先（仅用户故事 1）

1. 完成 Phase 1：初始化
2. 完成 Phase 2：基础能力
3. 完成 Phase 3：用户故事 1
4. 停止并验证用户故事 1 的独立可用性
5. 若通过，即形成第一阶段 MVP 主链路

### 增量交付

1. 完成初始化与基础能力，形成稳定基座
2. 加入用户故事 1，完成最小可执行价值
3. 加入用户故事 2，补足会话控制能力
4. 加入用户故事 3，补足渠道输出稳定性
5. 最后完成收尾测试、健康检查与文档校验

### 推荐 MVP 范围

- 建议 MVP 仅包含：**用户故事 1**
- 这是最小可交付价值：用户可在受控会话中发起、继续和恢复执行
- 用户故事 2 与用户故事 3 可在 MVP 验证通过后继续推进

---

## 备注

- 总任务数：46
- 用户故事任务数：
  - US1: 8
  - US2: 6
  - US3: 8
- 并行机会：初始化阶段、基础能力阶段部分任务、US1/US3 中的独立文件任务、收尾测试任务
- 所有任务均遵循固定 checklist 格式
- 当前任务清单严格限定在第一阶段范围，不包含多窗口、多 Agent、自动路由或重型中台实现
