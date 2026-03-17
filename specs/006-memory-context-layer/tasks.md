# 任务清单：Memory Context Layer（项目/Agent 记忆隔离）

**输入**: `/home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本特性在规格中已显式要求独立测试与可度量验收，任务清单包含单元/集成/契约测试。  
**组织方式**: 任务按用户故事分组，确保每个故事可独立实现与独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/ClawX/cmd/`、`/home/ubuntu/workspace/ClawX/internal/`
- 测试路径：`/home/ubuntu/workspace/ClawX/tests/`
- 文档路径：`/home/ubuntu/workspace/ClawX/docs/`、`/home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/`

## Phase 1：初始化（共享基础）

**目的**: 建立 memory 功能骨架、测试占位与验收文档入口

- [X] T001 创建 Phase 6 验证文档占位于 /home/ubuntu/workspace/ClawX/docs/guides/phase_6/phase_6_memory_context_validation.md
- [X] T002 [P] 创建 memory 集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/memory_context_smoke_test.go
- [X] T003 [P] 创建 memory 命令契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/memory_command_contract_test.go
- [X] T004 创建 memory 领域文档骨架于 /home/ubuntu/workspace/ClawX/internal/domain/memory/doc.go
- [X] T005 [P] 创建 memory 应用服务文档骨架于 /home/ubuntu/workspace/ClawX/internal/application/memory/doc.go

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的作用域、加载、持久化与审计基础

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [X] T006 定义 MemoryScopeKey/MemoryProfile/MemoryLoadItem 领域模型于 /home/ubuntu/workspace/ClawX/internal/domain/memory/model.go
- [X] T007 [P] 定义 Memory 存储接口（template/journal/audit/digest）于 /home/ubuntu/workspace/ClawX/internal/domain/memory/repository.go
- [X] T008 [P] 实现记忆模板与 manifest 文件存储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/memory_template_file_store.go
- [X] T009 [P] 实现记忆日记文件存储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/memory_journal_file_store.go
- [X] T010 [P] 实现记忆审计记录文件存储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/memory_audit_file_store.go
- [X] T011 [P] 实现 digest 作业状态存储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/memory_digest_file_store.go
- [X] T012 扩展配置结构（owner allowlist、budget、digest auto 开关）于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T013 [P] 增加 memory 配置校验单测于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config_test.go
- [X] T014 实现 MemoryScope 解析器（agent+project+route+chat_mode）于 /home/ubuntu/workspace/ClawX/internal/application/memory/scope_resolver.go
- [X] T015 实现 MemoryLoader 基础编排（输入/输出/降级）于 /home/ubuntu/workspace/ClawX/internal/application/memory/loader.go
- [X] T016 [P] 增加 MemoryScope 与加载器基础单测于 /home/ubuntu/workspace/ClawX/tests/unit/memory_scope_loader_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - 项目记忆骨架与首轮加载（优先级：P1） 🎯 MVP

**目标**: 在项目创建/修复时初始化记忆骨架，并在首轮执行前完成记忆加载

**独立验证**: 创建新项目并触发 `/new` 后，目录骨架存在且首轮已注入记忆上下文

### 测试任务（US1）

- [X] T017 [P] [US1] 增加记忆模板初始化与自愈单测于 /home/ubuntu/workspace/ClawX/tests/unit/memory_template_bootstrap_test.go
- [X] T018 [P] [US1] 增加项目创建触发记忆骨架集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_project_bootstrap_test.go
- [X] T019 [P] [US1] 增加首轮执行前记忆加载集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_first_turn_load_test.go

### 实现任务（US1）

- [X] T020 [US1] 在项目创建流程接入记忆骨架初始化于 /home/ubuntu/workspace/ClawX/internal/application/project/service_create.go
- [X] T021 [US1] 在项目修复流程接入记忆骨架自愈于 /home/ubuntu/workspace/ClawX/internal/application/project/service_repair.go
- [X] T022 [US1] 实现模板版本与漂移检测逻辑于 /home/ubuntu/workspace/ClawX/internal/application/memory/template_manager.go
- [X] T023 [US1] 在会话首轮链路注入 MemoryLoader 于 /home/ubuntu/workspace/ClawX/internal/application/service/router_session_flow.go
- [X] T024 [US1] 在后端执行前拼接记忆上下文于 /home/ubuntu/workspace/ClawX/internal/infrastructure/backend/profile_runner.go

**检查点**: US1 完成后，应可独立演示“创建即有骨架 + 首轮可加载”

---

## Phase 4：用户故事 2 - 同项目多 Agent 记忆隔离（优先级：P1）

**目标**: 同项目多 agent 并发时，私有记忆严格隔离，默认写回私有层

**独立验证**: Agent A/B 同项目并发，A 私有记忆不能被 B 读取

### 测试任务（US2）

- [X] T025 [P] [US2] 增加同项目多 agent 私有隔离集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_agent_isolation_test.go
- [X] T026 [P] [US2] 增加 agent 私有目录缺失自动初始化测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_agent_bootstrap_test.go
- [X] T027 [P] [US2] 增加跨 agent 路径访问拒绝契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/memory_agent_acl_contract_test.go
- [X] T028 [P] [US2] 增加跨项目记忆访问拒绝集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_cross_project_isolation_test.go

### 实现任务（US2）

- [X] T029 [US2] 实现 agent 私有路径解析与越界防护于 /home/ubuntu/workspace/ClawX/internal/application/memory/path_guard.go
- [X] T030 [US2] 在加载顺序中接入 `agent_private -> project_shared` 规则于 /home/ubuntu/workspace/ClawX/internal/application/memory/loader_layers.go
- [X] T031 [US2] 实现默认写回 agent 私有日记策略于 /home/ubuntu/workspace/ClawX/internal/application/memory/write_policy.go
- [X] T032 [US2] 输出跨 agent 拒绝审计字段于 /home/ubuntu/workspace/ClawX/internal/application/memory/audit.go

**检查点**: US2 完成后，应满足“同项目多 agent 不串读”

---

## Phase 5：用户故事 3 - 主会话与共享会话 ACL（优先级：P2）

**目标**: 主/共享会话差异化 ACL 生效，主会话判定采用“私聊 + owner allowlist”

**独立验证**: 共享会话必跳过长期私有层；主会话可在 ACL 通过时加载长期私有层

### 测试任务（US3）

- [X] T033 [P] [US3] 增加主会话判定单测（private + owner allowlist）于 /home/ubuntu/workspace/ClawX/tests/unit/memory_main_session_acl_test.go
- [X] T034 [P] [US3] 增加共享会话禁读 `MEMORY.md` 集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_shared_acl_test.go
- [X] T035 [P] [US3] 增加 ACL 冲突降级与审计集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_acl_degrade_test.go

### 实现任务（US3）

- [X] T036 [US3] 实现主会话分类器（chat_mode 决策）于 /home/ubuntu/workspace/ClawX/internal/application/memory/session_classifier.go
- [X] T037 [US3] 在加载器接入 `main_private` ACL 判定于 /home/ubuntu/workspace/ClawX/internal/application/memory/loader_acl.go
- [X] T038 [US3] 实现最小权限降级策略于 /home/ubuntu/workspace/ClawX/internal/application/memory/degrade_policy.go
- [X] T039 [US3] 记录 `memory_scope/memory_acl_mode` 审计字段于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go

**检查点**: US3 完成后，主/共享会话 ACL 可独立验收

---

## Phase 6：用户故事 4 - 记忆写回与可观测治理（优先级：P2）

**目标**: 交付 `/memory note|digest|audit`，完成写回、汇总与审计闭环

**独立验证**: 记忆记录、digest 汇总、审计输出三条路径独立可用

### 测试任务（US4）

- [X] T040 [P] [US4] 增加 `/memory note` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/memory_note_contract_test.go
- [X] T041 [P] [US4] 增加 `/memory digest` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/memory_digest_contract_test.go
- [X] T042 [P] [US4] 增加 `/memory audit` 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/memory_audit_contract_test.go
- [X] T043 [P] [US4] 增加 note/digest/audit 集成验收测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_commands_flow_test.go
- [X] T044 [P] [US4] 增加 `/memory note --shared` ACL 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/memory_note_shared_contract_test.go
- [X] T045 [P] [US4] 增加共享写入成功/拒绝集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_note_shared_flow_test.go

### 实现任务（US4）

- [X] T046 [US4] 新增 `/memory` CLI 命令入口于 /home/ubuntu/workspace/ClawX/cmd/clawx/memory.go
- [X] T047 [US4] 实现 `note` 应用服务（默认私有写回）于 /home/ubuntu/workspace/ClawX/internal/application/memory/service_note.go
- [X] T048 [US4] 实现 `digest` 应用服务（手工触发 + 自动默认关闭）于 /home/ubuntu/workspace/ClawX/internal/application/memory/service_digest.go
- [X] T049 [US4] 实现 `audit` 应用服务（缺失/权限/隔离异常汇总）于 /home/ubuntu/workspace/ClawX/internal/application/memory/service_audit.go
- [X] T050 [US4] 在控制流接入 `/memory` 命令优先级于 /home/ubuntu/workspace/ClawX/internal/application/service/router_control_flow.go
- [X] T051 [US4] 实现 `note --shared` 写入路径与 ACL 判定于 /home/ubuntu/workspace/ClawX/internal/application/memory/service_note.go
- [X] T052 [US4] 在 `/memory` 命令解析中接入 `--shared` 参数于 /home/ubuntu/workspace/ClawX/cmd/clawx/memory.go

**检查点**: US4 完成后，写回与治理闭环可独立验收

---

## Phase 7：收尾与跨领域事项

**目的**: 指标、文档、回归与发布门禁收口

- [X] T053 [P] 补充 memory 命令契约文档细节于 /home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/contracts/memory-command-contract.md
- [X] T054 [P] 补充 memory 加载契约文档细节于 /home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/contracts/memory-loading-contract.md
- [X] T055 [P] 更新 memory 快速验收指南于 /home/ubuntu/workspace/ClawX/specs/006-memory-context-layer/quickstart.md
- [X] T056 输出 SC-001~SC-006 指标采集测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_context_metrics_report_test.go
- [X] T057 运行全量回归并修复问题（`go test ./...`）于 /home/ubuntu/workspace/ClawX/tests/
- [X] T058 [P] 增加 `/resume`、`/switch` 项目记忆接入后语义不回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_control_resume_switch_regression_test.go
- [X] T059 [P] 增加 `/list`、`/current`、`/cancel` 项目记忆接入后语义不回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_control_list_current_cancel_regression_test.go
- [X] T060 [P] 增加审计字段 `memory_loaded_files/memory_denied_files/error_summary` 检索测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_audit_fields_search_test.go
- [X] T061 [P] 增加 `/new` 在记忆接入后语义不回归测试于 /home/ubuntu/workspace/ClawX/tests/integration/memory_control_new_regression_test.go

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-6**: 依赖 Phase 2；建议顺序 US1 -> US2 -> US3 -> US4
- **Phase 7**: 依赖已实现的用户故事

### 用户故事依赖

- **US1 (P1)**: 无故事级前置依赖，可在 Foundation 后优先落地（MVP）
- **US2 (P1)**: 依赖 US1 的加载骨架与路径体系
- **US3 (P2)**: 依赖 US1/US2 的加载链路与作用域解析
- **US4 (P2)**: 依赖 US1~US3 的加载、ACL 与审计基础能力

### 并行机会

- Phase 1 中 T002/T003/T005 可并行
- Phase 2 中 T008/T009/T010/T011/T013 可并行
- US1 中 T017/T018/T019 可并行
- US2 中 T025/T026/T027/T028 可并行
- US3 中 T033/T034/T035 可并行
- US4 中 T040/T041/T042/T043/T044/T045 可并行
- Phase 7 中 T053/T054/T055/T056/T058/T059/T060/T061 可并行

---

## 并行执行示例

### US1 并行示例

```bash
Task: "T017 [US1] memory 模板初始化与自愈单测"
Task: "T018 [US1] 项目创建触发记忆骨架集成测试"
Task: "T019 [US1] 首轮执行前记忆加载集成测试"
```

### US2 并行示例

```bash
Task: "T025 [US2] 多 agent 私有隔离集成测试"
Task: "T026 [US2] agent 私有目录自动初始化测试"
Task: "T027 [US2] 跨 agent 路径访问拒绝契约测试"
Task: "T028 [US2] 跨项目记忆访问拒绝集成测试"
```

### US3 并行示例

```bash
Task: "T033 [US3] 主会话判定单测"
Task: "T034 [US3] 共享会话禁读 MEMORY.md 集成测试"
Task: "T035 [US3] ACL 冲突降级与审计测试"
```

### US4 并行示例

```bash
Task: "T040 [US4] /memory note 契约测试"
Task: "T041 [US4] /memory digest 契约测试"
Task: "T042 [US4] /memory audit 契约测试"
Task: "T043 [US4] note/digest/audit 集成验收测试"
Task: "T044 [US4] /memory note --shared ACL 契约测试"
Task: "T045 [US4] 共享写入成功/拒绝集成测试"
```

---

## 实施策略

### MVP 优先（US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即执行独立验收（项目骨架 + 首轮加载）

### 增量交付

1. 先交付骨架初始化与首轮加载（US1）
2. 再交付同项目多 agent 隔离（US2）
3. 再交付主/共享会话 ACL（US3）
4. 最后交付写回与审计治理（US4）

### 推荐 MVP 范围

- 推荐 MVP：**US1 + US2**
- 该范围可最早验证“记忆可用 + 多 agent 不串读”的核心价值

---

## 备注

- 总任务数：61
- US1 任务数：8
- US2 任务数：8
- US3 任务数：7
- US4 任务数：13
- 并行机会：23+
