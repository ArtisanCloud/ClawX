# 任务清单：Unified Scheduler Center（统一定时任务中心）

**输入**: `/home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本特性规格明确要求每个用户故事可独立验证，任务清单包含契约/集成/单元测试任务。  
**组织方式**: 任务按用户故事分组，确保每个故事可独立实现与独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/ClawX/cmd/`、`/home/ubuntu/workspace/ClawX/internal/`
- 测试路径：`/home/ubuntu/workspace/ClawX/tests/`
- 文档路径：`/home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/`

## Phase 1：初始化（共享基础）

**目的**: 建立 scheduler 功能骨架、文档入口与测试占位

- [X] T001 创建 scheduler 领域文档骨架于 /home/ubuntu/workspace/ClawX/internal/domain/scheduler/doc.go
- [X] T002 [P] 创建 scheduler 应用服务文档骨架于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/doc.go
- [X] T003 [P] 创建 schedule 契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_command_contract_test.go
- [X] T004 [P] 创建 schedule 集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_runtime_flow_test.go
- [X] T005 校准 007 快速验收脚本与最终实现一致性于 /home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/quickstart.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成全部用户故事共享的领域模型、仓储、命令解析与调度 runner 骨架

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [X] T006 定义 ScheduleScope/ScheduleJob/ScheduleRunRecord 领域模型于 /home/ubuntu/workspace/ClawX/internal/domain/scheduler/model.go
- [X] T007 [P] 定义 scheduler 仓储接口与执行器接口于 /home/ubuntu/workspace/ClawX/internal/domain/scheduler/repository.go
- [X] T008 [P] 实现 scheduler 任务文件仓储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/scheduler_file_store.go
- [X] T009 [P] 实现 scheduler 运行记录文件仓储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/scheduler_run_file_store.go
- [X] T010 [P] 新增 `/schedule` 命令解析器于 /home/ubuntu/workspace/ClawX/internal/application/command/schedule_command.go
- [X] T011 实现 scheduler 应用服务基础编排（CRUD + run）于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service.go
- [X] T012 [P] 实现任务执行锁与并发防护于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/execution_lock.go
- [X] T013 接入 Router 控制命令识别 `/schedule` 于 /home/ubuntu/workspace/ClawX/internal/application/service/router.go
- [X] T014 接入控制流处理 `/schedule` 于 /home/ubuntu/workspace/ClawX/internal/application/service/router_control_flow.go
- [X] T015 在运行时装配 scheduler service 于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T016 [P] 增加 schedule 命令解析单测于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_command_parse_contract_test.go
- [X] T017 [P] 增加 scheduler 仓储读写单测于 /home/ubuntu/workspace/ClawX/tests/unit/scheduler_file_store_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - 统一注册与管理定时任务（优先级：P1） 🎯 MVP

**目标**: 交付 `/schedule add|list|status|pause|resume|run|remove` 全流程管理能力

**独立验证**: 单会话内完成新增->查询->暂停->恢复->立即执行->删除，并验证状态流转正确

### 测试任务（US1）

- [X] T018 [P] [US1] 增加 `/schedule add/list/status` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_command_contract_test.go
- [X] T019 [P] [US1] 增加 `/schedule pause/resume/remove` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_state_contract_test.go
- [X] T020 [P] [US1] 增加 `/schedule run` 并发冲突契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_run_contract_test.go
- [X] T021 [P] [US1] 增加命令全链路集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_command_flow_test.go

### 实现任务（US1）

- [X] T022 [US1] 实现任务新增与作用域内重名冲突检测于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_create.go
- [X] T023 [US1] 实现任务列表与详情查询于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_query.go
- [X] T024 [US1] 实现 pause/resume 状态流转于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_state.go
- [X] T025 [US1] 实现 remove 逻辑并保留历史执行记录于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_remove.go
- [X] T026 [US1] 实现 run-now 手工触发执行于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_run.go
- [X] T027 [US1] 在控制流返回结构化状态响应于 /home/ubuntu/workspace/ClawX/internal/application/service/router_control_flow.go

**检查点**: US1 完成后，应可独立演示完整管理闭环

---

## Phase 4：用户故事 2 - 自然语言注册图片清理任务（优先级：P1）

**目标**: 用户发送“每周清理图片记录”即可自动注册默认策略任务（周日 03:00 + 保留 30 天）

**独立验证**: 仅通过自然语言即可完成任务创建、查询与立即执行

### 测试任务（US2）

- [X] T028 [P] [US2] 增加“每周清理图片记录”自然语言映射集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_nl_mapping_test.go
- [X] T029 [P] [US2] 增加默认时间与默认保留策略契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_default_policy_contract_test.go
- [X] T030 [P] [US2] 增加图片清理任务执行结果校验集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_image_cleanup_flow_test.go

### 实现任务（US2）

- [X] T031 [US2] 扩展路由自然语言到 `/schedule add` 的映射规则于 /home/ubuntu/workspace/ClawX/internal/application/service/router.go
- [X] T032 [US2] 实现 `image.cleanup` 执行器（默认保留 30 天）于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/task_image_cleanup.go
- [X] T033 [US2] 实现自然语言映射模板结构于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/intent_template.go
- [X] T034 [US2] 在调度执行输出中增加清理摘要格式于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/run_summary.go

**检查点**: US2 完成后，应满足“自然语言直接注册并执行”

---

## Phase 5：用户故事 3 - 多项目与多 Agent 隔离（优先级：P2）

**目标**: 定时任务严格按 `project+agent` 隔离，跨域不可见、不可改、不可执行

**独立验证**: 在不同项目或不同 agent 下创建同名任务，查询/执行互不影响

### 测试任务（US3）

- [X] T035 [P] [US3] 增加跨项目任务不可见集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_scope_project_isolation_test.go
- [X] T036 [P] [US3] 增加同项目跨 agent 任务不可见集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_scope_agent_isolation_test.go
- [X] T037 [P] [US3] 增加跨作用域修改/执行拒绝契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_scope_acl_contract_test.go
- [X] T038 [P] [US3] 增加作用域变更显式声明与审计集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_scope_change_audit_test.go

### 实现任务（US3）

- [X] T039 [US3] 实现 scheduler scope 解析与标准化于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/scope_resolver.go
- [X] T040 [US3] 在服务层强制作用域校验于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/scope_guard.go
- [X] T041 [US3] 在既有仓储中补充作用域过滤规则与回归于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/scheduler_file_store.go
- [X] T042 [US3] 增加越权访问与作用域变更审计字段输出于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/audit.go

**检查点**: US3 完成后，应满足“同名任务跨作用域不串扰”

---

## Phase 6：用户故事 4 - 统一可观测与恢复（优先级：P2）

**目标**: 提供结构化执行记录、失败诊断与重启恢复调度能力

**独立验证**: 人为制造失败并重启服务后，任务可恢复且最近执行信息可查询

### 测试任务（US4）

- [X] T043 [P] [US4] 增加执行记录字段完整性契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/schedule_run_record_contract_test.go
- [X] T044 [P] [US4] 增加单任务并发执行锁集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_single_active_run_test.go
- [X] T045 [P] [US4] 增加服务重启后任务恢复集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_restart_recovery_test.go
- [X] T046 [P] [US4] 增加单任务失败不阻塞其他任务集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_failure_isolation_test.go
- [X] T047 [P] [US4] 增加“失败后保持 active 并重算 next_run”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_fail_keep_active_test.go

### 实现任务（US4）

- [X] T048 [US4] 实现调度 runner 周期扫描与触发于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/runner.go
- [X] T049 [US4] 实现 next_run 计算与更新时间逻辑于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/schedule_calc.go
- [X] T050 [US4] 实现执行记录写入与最近结果聚合于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/run_record_service.go
- [X] T051 [US4] 在服务启动流程接入 runner 生命周期管理于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T052 [US4] 在状态查询中输出最近失败原因与下次执行时间于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/service_query.go
- [X] T053 [US4] 实现错误码到可行动 `next_action` 映射于 /home/ubuntu/workspace/ClawX/internal/application/scheduler/error_mapper.go

**检查点**: US4 完成后，应可独立验证可观测与恢复闭环

---

## Phase 7：收尾与跨领域事项

**目的**: 文档收口、回归验证与发布门禁

- [X] T054 [P] 更新 007 命令契约细节（含时区与可行动错误）于 /home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/contracts/schedule-command-contract.md
- [X] T055 [P] 更新 007 运行时契约细节于 /home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/contracts/schedule-runtime-contract.md
- [X] T056 [P] 更新 007 快速验收脚本于 /home/ubuntu/workspace/ClawX/specs/007-unified-scheduler-center/quickstart.md
- [X] T057 增加调度功能端到端回归测试入口于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_regression_suite_test.go
- [X] T058 [P] 增加 `/service` 与 `/schedule` 共存回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/service_schedule_coexist_test.go
- [X] T059 [P] 增加 `/memory` 与 `/schedule` 共存回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_schedule_coexist_test.go
- [X] T060 [P] 增加 SC-001 首次任务操作耗时门禁测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_metrics_first_use_time_test.go
- [X] T061 [P] 增加 SC-003 调度准时率指标门禁测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_metrics_timeliness_test.go
- [X] T062 [P] 增加 SC-006 自然语言映射成功率指标门禁测试于 /home/ubuntu/workspace/ClawX/tests/integration/schedule_metrics_nl_success_test.go
- [X] T063 运行全量测试并修复回归（`go test ./...`）于 /home/ubuntu/workspace/ClawX/tests/

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-6**: 依赖 Phase 2；建议顺序 US1 -> US2 -> US3 -> US4
- **Phase 7**: 依赖已实现的用户故事

### 用户故事依赖

- **US1 (P1)**: Foundation 后可独立开始
- **US2 (P1)**: 依赖 US1 的命令闭环与执行器入口
- **US3 (P2)**: 依赖 US1 的任务存储与查询路径
- **US4 (P2)**: 依赖 US1~US3 的任务模型、作用域与执行路径

### 并行机会

- Phase 1: T002/T003/T004 可并行
- Phase 2: T008/T009/T010/T012/T016/T017 可并行
- US1: T018/T019/T020/T021 可并行
- US2: T028/T029/T030 可并行
- US3: T035/T036/T037/T038 可并行
- US4: T043/T044/T045/T046/T047 可并行
- Phase 7: T054/T055/T056/T058/T059/T060/T061/T062 可并行

---

## 并行执行示例

### US1 并行示例

```bash
Task: "T018 [US1] add/list/status 契约测试"
Task: "T019 [US1] pause/resume/remove 契约测试"
Task: "T020 [US1] run 并发冲突契约测试"
Task: "T021 [US1] 命令全链路集成测试"
```

### US2 并行示例

```bash
Task: "T028 [US2] 自然语言映射集成测试"
Task: "T029 [US2] 默认策略契约测试"
Task: "T030 [US2] 图片清理执行集成测试"
```

### US3 并行示例

```bash
Task: "T035 [US3] 跨项目隔离测试"
Task: "T036 [US3] 跨 agent 隔离测试"
Task: "T037 [US3] 越权拒绝契约测试"
```

### US4 并行示例

```bash
Task: "T042 [US4] 执行记录契约测试"
Task: "T043 [US4] 单任务并发锁测试"
Task: "T044 [US4] 重启恢复测试"
Task: "T045 [US4] 失败隔离测试"
Task: "T046 [US4] 失败后保持 active 测试"
```

---

## 实施策略

### MVP 优先（US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即执行独立验收（命令闭环）

### 增量交付

1. 先交付统一命令管理闭环（US1）
2. 再交付自然语言图片清理（US2）
3. 再交付跨项目/跨 agent 隔离（US3）
4. 最后交付可观测与恢复能力（US4）

### 推荐 MVP 范围

- 推荐 MVP：**US1**
- 该范围可最早验证“统一定时任务中心可用”

---

## 备注

- 总任务数：63
- US1 任务数：10
- US2 任务数：7
- US3 任务数：8
- US4 任务数：11
- 并行机会：30+
