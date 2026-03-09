# 任务清单：第二阶段多窗口多会话

**输入**: `/home/ubuntu/workspace/SynapseX/specs/002-multi-session/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 规格中已给出独立测试方式与验收场景，因此本清单包含测试任务（单元/集成/契约）。

**组织方式**: 任务按用户故事分组，确保每个故事可独立实现与独立验证。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/SynapseX/internal/`、`/home/ubuntu/workspace/SynapseX/cmd/`
- 测试路径：`/home/ubuntu/workspace/SynapseX/tests/`
- 文档路径：`/home/ubuntu/workspace/SynapseX/docs/`、`/home/ubuntu/workspace/SynapseX/specs/002-multi-session/`

## Phase 1：初始化（共享基础）

**目的**: 建立 Phase 2 的验证入口与文档占位，不改变既有业务路径

- [X] T001 创建第二阶段验证文档占位于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_2/phase_2_validation.md
- [X] T002 [P] 创建多会话集成测试骨架文件于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_window_routing_test.go
- [X] T003 [P] 创建多会话契约测试骨架文件于 /home/ubuntu/workspace/SynapseX/tests/contract/multi_session_control_contract_test.go

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的窗口模型输入与仓储能力

**⚠️ 关键说明**: 本阶段完成前不得开始用户故事实现

- [ ] T004 扩展统一消息模型增加 `WindowID` 字段于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/message.go
- [ ] T005 [P] 扩展消息归一化输入支持显式 `window_id` 于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T006 [P] 实现缺省窗口兼容值生成 `compat:<conversation_id>` 于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T007 扩展会话命令对象支持 `WindowID` 于 /home/ubuntu/workspace/SynapseX/internal/application/command/session_command.go
- [ ] T008 [P] 扩展控制命令对象支持 `WindowID` 于 /home/ubuntu/workspace/SynapseX/internal/application/command/control_command.go
- [ ] T009 定义窗口绑定仓储接口（get/set/list）于 /home/ubuntu/workspace/SynapseX/internal/domain/session/repository.go
- [ ] T010 [P] 在内存仓储实现窗口绑定读写与窗口级查询于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/persistence/session_memory_repository.go
- [ ] T011 调整 Session Manager 依赖注入并暴露窗口绑定方法于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager.go
- [ ] T012 [P] 补充内存仓储窗口绑定单元测试于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/persistence/session_memory_repository_test.go

**检查点**: 基础能力完成后，用户故事可进入实现

---

## Phase 3：用户故事 1 - 在多个窗口中持有独立会话（优先级：P1） 🎯 MVP

**目标**: 同一用户多个窗口可稳定绑定不同会话，后续输入按窗口进入正确会话

**独立验证**: 为同一用户创建窗口 A/B，分别继续不同会话，确认互不串线

### 测试任务（US1）

- [ ] T013 [P] [US1] 增加窗口独立绑定单元测试于 /home/ubuntu/workspace/SynapseX/tests/unit/multi_session_domain_test.go
- [ ] T014 [P] [US1] 增加窗口 A/B 并行继续集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_window_routing_test.go

### 实现任务（US1）

- [ ] T015 [US1] 在新建会话流程写入 `WindowID` 并刷新窗口绑定于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_create.go
- [ ] T016 [US1] 在继续会话流程实现“窗口优先 -> conversation 回退 -> 新建”于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_continue.go
- [ ] T017 [US1] 在恢复会话流程刷新窗口绑定并校验上下文归属于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_resume.go
- [ ] T018 [US1] 在会话路由流程透传 `WindowID` 并按窗口决策于 /home/ubuntu/workspace/SynapseX/internal/application/service/router_session_flow.go
- [ ] T019 [US1] 在路由决策对象中保留 `WindowID` 上下文于 /home/ubuntu/workspace/SynapseX/internal/application/service/router.go
- [ ] T020 [US1] 在 new/resume/switch/执行成功后统一刷新 session 与 window 的 `last_used_at` 于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_create.go、/home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_resume.go、/home/ubuntu/workspace/SynapseX/internal/application/service/router_session_flow.go
- [ ] T021 [P] [US1] 增加最近使用时间刷新单元测试（覆盖 new/resume/switch/execute_success）于 /home/ubuntu/workspace/SynapseX/tests/unit/multi_session_recency_test.go

**检查点**: US1 完成后，应可独立演示“多窗口不串线”

---

## Phase 4：用户故事 2 - 在当前窗口中查看并切换会话（优先级：P2）

**目标**: 当前窗口可查看会话、恢复会话、切换当前会话并保持后续输入一致

**独立验证**: 在同一窗口创建/恢复多个会话后，`/list` 可见当前标记，`/switch` 后输入进入目标会话

### 测试任务（US2）

- [ ] T022 [P] [US2] 增加窗口语义控制流集成测试（list/current/switch）于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_control_switch_test.go
- [ ] T023 [P] [US2] 增加控制命令窗口契约测试于 /home/ubuntu/workspace/SynapseX/tests/contract/multi_session_control_contract_test.go

### 实现任务（US2）

- [ ] T024 [US2] 扩展控制命令解析新增 `/switch <session_id>` 于 /home/ubuntu/workspace/SynapseX/internal/application/command/control_command.go
- [ ] T025 [US2] 在控制流实现 `/switch` 仅更新窗口绑定不执行于 /home/ubuntu/workspace/SynapseX/internal/application/service/router_control_flow.go
- [ ] T026 [US2] 将 `/new`、`/resume`、`/list`、`/cancel`、`/current` 全量切换到窗口语义于 /home/ubuntu/workspace/SynapseX/internal/application/service/router_control_flow.go
- [ ] T027 [US2] 扩展会话列表服务支持窗口级列表与当前会话标记于 /home/ubuntu/workspace/SynapseX/internal/application/service/session_manager_list.go
- [ ] T028 [US2] 更新控制命令响应映射支持窗口当前会话反馈于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/control_response.go

**检查点**: US1 与 US2 均应可独立验证

---

## Phase 5：用户故事 3 - 在保留旧行为的同时升级到窗口模型（优先级：P3）

**目标**: 无显式 `window_id` 路径保持可用，且内建控制命令优先级稳定

**独立验证**: 旧入口不传 `window_id` 仍可继续会话；控制命令不会被自然语言路径误判

### 测试任务（US3）

- [ ] T029 [P] [US3] 增加缺省 `window_id` 兼容回退集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_compat_fallback_test.go
- [ ] T030 [P] [US3] 增加命令优先级回归测试（控制命令优先）于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_command_priority_test.go
- [ ] T031 [P] [US3] 增加“同一 session 并发请求必须串行/拒绝”集成回归测试于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_serial_execution_test.go
- [ ] T032 [P] [US3] 增加窗口绑定字段持久化与查询测试（`window_id/current_session_id/conversation_id/updated_at/last_used_at`）于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_binding_fields_test.go

### 实现任务（US3）

- [ ] T033 [US3] 固化缺省窗口兼容策略并统一错误返回于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T034 [US3] 在 Router 固化内建控制命令优先分流于 /home/ubuntu/workspace/SynapseX/internal/application/service/router.go
- [ ] T035 [US3] 扩展错误映射覆盖窗口绑定异常场景于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/error_response.go
- [ ] T036 [US3] 同步“无 window_id 兼容路径”冒烟验证到 /home/ubuntu/workspace/SynapseX/tests/integration/phase1_foundation_test.go

**检查点**: 三个用户故事均可独立通过各自验收场景

---

## Phase 6：收尾与跨领域事项

**目的**: 完成跨故事收尾、回归验证和文档闭环

- [ ] T037 [P] 补充窗口语义渠道契约说明于 /home/ubuntu/workspace/SynapseX/specs/002-multi-session/contracts/window-routing-contract.md
- [ ] T038 [P] 补充控制命令窗口语义与错误契约说明于 /home/ubuntu/workspace/SynapseX/specs/002-multi-session/contracts/control-command-window-contract.md
- [ ] T039 运行全量回归测试并修复问题（`go test ./...`）于 /home/ubuntu/workspace/SynapseX/tests/
- [ ] T040 更新第二阶段验证步骤与人工验收脚本于 /home/ubuntu/workspace/SynapseX/specs/002-multi-session/quickstart.md
- [ ] T041 回写阶段交付摘要与风险状态于 /home/ubuntu/workspace/SynapseX/docs/plans/phase_2_multi_session.md
- [ ] T042 [P] 生成 SC-001~SC-004 指标采集与统计脚本（7 天窗口、>=200 样本）于 /home/ubuntu/workspace/SynapseX/tests/integration/multi_session_metrics_report_test.go
- [ ] T043 执行 SC-005 人工验收并输出通过率报告于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_2/phase_2_validation.md
- [ ] T044 [P] 将 SC 指标采集与人工验收结论回写发布门禁说明于 /home/ubuntu/workspace/SynapseX/docs/plans/phase_2_multi_session.md

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1（初始化）**: 无依赖，可立即开始
- **Phase 2（基础能力）**: 依赖 Phase 1 完成；阻塞所有用户故事
- **Phase 3-5（用户故事）**: 全部依赖 Phase 2 完成
- **Phase 6（收尾）**: 依赖已完成的用户故事实现

### 用户故事依赖

- **用户故事 1（P1）**: 仅依赖基础能力，是 MVP 最小范围
- **用户故事 2（P2）**: 依赖用户故事 1 已具备窗口优先路由
- **用户故事 3（P3）**: 依赖用户故事 1/2 的窗口语义与控制流稳定

### 各故事内部依赖

- 测试任务先于对应故事实现任务
- 命令与仓储接口变更先于路由与会话流程
- 核心实现先于跨领域文档与回归收尾

### 并行机会

- Phase 1：T002 与 T003 可并行
- Phase 2：T005、T006、T008、T010、T012 可并行
- US1：T013、T014 与 T021 可并行
- US2：T022 与 T023 可并行
- US3：T029、T030、T031 与 T032 可并行
- Phase 6：T037、T038、T042 与 T044 可并行

---

## 并行执行示例

### 用户故事 1

```bash
任务: "增加窗口独立绑定单元测试于 tests/unit/multi_session_domain_test.go"
任务: "增加窗口 A/B 并行继续集成测试于 tests/integration/multi_session_window_routing_test.go"
```

### 用户故事 2

```bash
任务: "增加窗口语义控制流集成测试于 tests/integration/multi_session_control_switch_test.go"
任务: "增加控制命令窗口契约测试于 tests/contract/multi_session_control_contract_test.go"
```

---

## 实施策略

### MVP 优先（仅用户故事 1）

1. 完成 Phase 1：初始化
2. 完成 Phase 2：基础能力
3. 完成 Phase 3：用户故事 1
4. 停止并验证用户故事 1 的独立可用性

### 增量交付

1. 先完成共享基础与窗口模型
2. 交付 US1（多窗口独立会话）
3. 交付 US2（窗口内查看与切换）
4. 交付 US3（兼容回退与命令优先级）
5. 最后完成收尾回归与文档闭环

### 推荐 MVP 范围

- 建议 MVP 仅包含：**用户故事 1**
- 该范围可最早验证“多窗口多会话不串线”的核心价值

---

## 备注

- 总任务数：44
- 用户故事任务数：
  - US1: 9（T013-T021）
  - US2: 7（T022-T028）
  - US3: 8（T029-T036）
- 并行机会：8 组（见 `[P]` 标记）
- 所有任务均符合 checklist 格式：`- [ ] Txxx [P?] [US?] 描述 + 文件路径`
