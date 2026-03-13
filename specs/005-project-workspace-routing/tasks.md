# 任务清单：项目空间隔离与路由

**输入**: `/home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`quickstart.md`

**测试**: 本阶段要求项目隔离与命令语义不回归，任务清单包含单元/集成/契约测试。

**组织方式**: 任务按用户故事分组，保证每个故事可独立实现和独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/ClawX/cmd/`、`/home/ubuntu/workspace/ClawX/internal/`
- 测试路径：`/home/ubuntu/workspace/ClawX/tests/`
- 文档路径：`/home/ubuntu/workspace/ClawX/docs/`、`/home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/`

## Phase 1：初始化（共享基础）

**目的**: 建立项目隔离功能骨架与测试占位

- [X] T001 创建 Phase 5 验证文档占位于 /home/ubuntu/workspace/ClawX/docs/guides/phase_5/phase_5_project_workspace_validation.md
- [X] T002 [P] 创建项目隔离集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/project_workspace_smoke_test.go
- [X] T003 [P] 创建项目命令契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/project_command_contract_test.go
- [X] T004 创建项目领域目录骨架于 /home/ubuntu/workspace/ClawX/internal/domain/project/
- [X] T005 [P] 创建项目应用服务目录骨架于 /home/ubuntu/workspace/ClawX/internal/application/project/

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的项目注册、绑定、路由基础

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [X] T006 定义 Project/Binding/Proposal 领域模型于 /home/ubuntu/workspace/ClawX/internal/domain/project/model.go
- [X] T007 [P] 定义项目仓储接口于 /home/ubuntu/workspace/ClawX/internal/domain/project/repository.go
- [X] T008 [P] 实现文件存储 `projects.json` 读写于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/project_registry_file_store.go
- [X] T009 [P] 实现文件存储 `bindings.json` 读写于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/project_binding_file_store.go
- [X] T010 [P] 实现文件存储 `proposals.json` 读写于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/project_proposal_file_store.go
- [X] T011 实现项目应用服务（create/list/use/current）于 /home/ubuntu/workspace/ClawX/internal/application/project/service.go
- [X] T012 [P] 扩展配置结构支持项目目录前缀与默认项目于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T013 [P] 增加项目配置校验单测于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config_test.go
- [X] T014 在消息归一化层补充 route key 生成于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/normalize.go
- [X] T015 在路由入口注入项目判定（binding -> default fallback）于 /home/ubuntu/workspace/ClawX/internal/application/service/router.go
- [X] T016 [P] 增加 route key 归一化单测（含 thread 优先）于 /home/ubuntu/workspace/ClawX/tests/unit/project_route_key_test.go
- [X] T017 [P] 增加项目 fallback 路由集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_fallback_routing_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - 显式管理项目与 workspace（优先级：P1） 🎯 MVP

**目标**: 交付 `/project create/list/use/current` 和 workspace 隔离闭环

**独立验证**: 两个项目可创建、可切换、可查询，且目录独立

### 测试任务（US1）

- [X] T018 [P] [US1] 增加 `/project create/list/current` 命令单测于 /home/ubuntu/workspace/ClawX/tests/unit/project_command_create_list_current_test.go
- [X] T019 [P] [US1] 增加 `/project use` 切换集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_use_switch_test.go
- [X] T020 [P] [US1] 增加 workspace 自动创建与状态校验测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_workspace_bootstrap_test.go

### 实现任务（US1）

- [X] T021 [US1] 新增 `/project` CLI 命令入口于 /home/ubuntu/workspace/ClawX/cmd/clawx/project.go
- [X] T022 [US1] 接入 `/project create` 与项目目录初始化于 /home/ubuntu/workspace/ClawX/internal/application/project/service_create.go
- [X] T023 [US1] 接入 `/project list` 与状态聚合于 /home/ubuntu/workspace/ClawX/internal/application/project/service_list.go
- [X] T024 [US1] 接入 `/project use` 与当前 route key 绑定于 /home/ubuntu/workspace/ClawX/internal/application/project/service_use.go
- [X] T025 [US1] 接入 `/project current` 查询于 /home/ubuntu/workspace/ClawX/internal/application/project/service_current.go

**检查点**: US1 完成后，应可独立演示项目管理主路径

---

## Phase 4：用户故事 2 - 同一 bot 并发多项目隔离（优先级：P1）

**目标**: route key 级项目隔离稳定，`/new` 语义不变

**独立验证**: 两个 route key 并发任务不串项目，`/new` 只新建会话

### 测试任务（US2）

- [X] T026 [P] [US2] 增加跨 route key 项目隔离集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_route_isolation_test.go
- [X] T027 [P] [US2] 增加 `/new` 不切项目回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_new_semantics_test.go
- [X] T028 [P] [US2] 增加会话键含 `project_id` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/project_session_key_contract_test.go

### 实现任务（US2）

- [X] T029 [US2] 扩展会话键生成加入 `project_id` 于 /home/ubuntu/workspace/ClawX/internal/application/service/session_manager_create.go
- [X] T030 [US2] 在继续会话链路透传 `project_id` 于 /home/ubuntu/workspace/ClawX/internal/application/service/session_manager_continue.go
- [X] T031 [US2] 在控制流中锁定 `/new` 不触发项目切换于 /home/ubuntu/workspace/ClawX/internal/application/service/router_control_flow.go
- [X] T032 [US2] 增加项目维度审计字段输出于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go

**检查点**: US1+US2 完成后，应满足“同 bot 多项目并发可用”

---

## Phase 5：用户故事 3 - 意图辅助切换（优先级：P2）

**目标**: 高置信命中非当前项目时仅建议切换，确认后生效

**独立验证**: 建议、确认、拒绝、超时四条路径都可验证

### 测试任务（US3）

- [X] T033 [P] [US3] 增加建议切换生成规则单测于 /home/ubuntu/workspace/ClawX/tests/unit/project_switch_proposal_test.go
- [X] T034 [P] [US3] 增加确认式切换集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_switch_confirm_test.go
- [X] T035 [P] [US3] 增加 proposal 超时失效测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_switch_proposal_expire_test.go

### 实现任务（US3）

- [X] T036 [US3] 在意图路由中接入项目候选判定于 /home/ubuntu/workspace/ClawX/internal/application/intent/pipeline.go
- [X] T037 [US3] 实现 proposal 创建与持久化于 /home/ubuntu/workspace/ClawX/internal/application/project/service_proposal.go
- [X] T038 [US3] 实现 `/project confirm <proposal_id>` 命令于 /home/ubuntu/workspace/ClawX/cmd/clawx/project.go
- [X] T039 [US3] 实现 proposal 生命周期回收任务于 /home/ubuntu/workspace/ClawX/internal/application/project/proposal_gc.go

**检查点**: US3 完成后，意图切换流程可独立验收

---

## Phase 6：用户故事 4 - 可观测与恢复（优先级：P3）

**目标**: 提供绑定修复、审计导出、异常状态治理

**独立验证**: registry/binding 异常可识别可修复

### 测试任务（US4）

- [ ] T040 [P] [US4] 增加损坏 registry 检测测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_registry_broken_state_test.go
- [ ] T041 [P] [US4] 增加绑定修复命令集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_binding_repair_test.go
- [ ] T042 [P] [US4] 增加项目删除保护测试（活动绑定/会话）于 /home/ubuntu/workspace/ClawX/tests/integration/project_delete_guard_test.go

### 实现任务（US4）

- [ ] T043 [US4] 实现 `/project bind` 与 `/project unbind` 于 /home/ubuntu/workspace/ClawX/cmd/clawx/project.go
- [ ] T044 [US4] 实现 `/project audit` 汇总命令于 /home/ubuntu/workspace/ClawX/internal/application/project/service_audit.go
- [ ] T045 [US4] 实现 `/project delete` 安全检查逻辑于 /home/ubuntu/workspace/ClawX/internal/application/project/service_delete.go
- [ ] T046 [US4] 增加 broken 状态修复逻辑于 /home/ubuntu/workspace/ClawX/internal/application/project/service_repair.go

**检查点**: US4 完成后，运维治理闭环可独立验收

---

## Phase 7：收尾与跨领域事项

**目的**: 契约、文档、回归、发布门禁收口

- [ ] T047 [P] 补充项目命令契约文档于 /home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/contracts/project-command-contract.md
- [ ] T048 [P] 补充项目路由契约文档于 /home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/contracts/project-routing-contract.md
- [ ] T049 [P] 更新 Phase 5 人工验收指南于 /home/ubuntu/workspace/ClawX/specs/005-project-workspace-routing/quickstart.md
- [ ] T050 更新 Phase 5 阶段计划状态于 /home/ubuntu/workspace/ClawX/docs/plans/phase_5_project_workspace_routing.md
- [ ] T051 运行全量回归并修复问题（`go test ./...`）于 /home/ubuntu/workspace/ClawX/tests/
- [ ] T052 [P] 输出 SC-001~SC-006 指标采集测试于 /home/ubuntu/workspace/ClawX/tests/integration/project_routing_metrics_report_test.go

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-6**: 依赖 Phase 2；建议顺序 US1 -> US2 -> US3 -> US4
- **Phase 7**: 依赖已实现的用户故事

### 用户故事依赖

- **US1 (P1)**: 无故事级前置依赖，可在 Foundation 后先落地（MVP）
- **US2 (P1)**: 依赖 US1 的项目命令与绑定能力
- **US3 (P2)**: 依赖 US1/US2 的项目判定与上下文
- **US4 (P3)**: 依赖 US1~US3 已建立的持久化状态

### 并行机会

- Phase 1 中 T002 与 T003 可并行
- Phase 2 中 T008/T009/T010 可并行
- US1 中 T018/T019/T020 可并行
- US2 中 T026/T027/T028 可并行
- US3 中 T033/T034/T035 可并行
- US4 中 T040/T041/T042 可并行
- Phase 7 中 T047/T048/T052 可并行

---

## 实施策略

### MVP 优先（US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即进行独立验收（项目创建/切换/查询）

### 增量交付

1. 先交付显式项目管理能力（US1）
2. 再交付并发隔离与 `/new` 语义守护（US2）
3. 再交付意图建议切换（US3）
4. 最后交付恢复治理（US4）

### 推荐 MVP 范围

- 推荐 MVP：**US1 + US2**
- 该范围可最早验证“同一 bot 并发多项目隔离”的核心价值

---

## 备注

- 总任务数：52
- US1 任务数：8
- US2 任务数：7
- US3 任务数：7
- US4 任务数：7
- 并行机会：10+
