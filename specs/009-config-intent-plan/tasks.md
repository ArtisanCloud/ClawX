# 任务清单：Config Intent Plan（配置意图统一控制）

**输入**: `/home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 规格要求每个用户故事可独立验收，任务清单包含契约/集成/单元测试任务。  
**组织方式**: 任务按用户故事分组，确保每个故事可独立实现与独立验证。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务都包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/ClawX/cmd/`、`/home/ubuntu/workspace/ClawX/internal/`
- 测试路径：`/home/ubuntu/workspace/ClawX/tests/`
- 文档路径：`/home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/`

## Phase 1：初始化（共享基础）

**目的**: 建立 009 功能开发骨架与测试入口

- [x] T001 创建 009 任务回归入口测试文件于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_regression_suite_test.go
- [x] T002 [P] 创建配置意图契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/config_intent_contract_test.go
- [x] T003 [P] 创建配置计划生命周期契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_lifecycle_contract_test.go
- [x] T004 [P] 创建 pending plan 领域单测骨架于 /home/ubuntu/workspace/ClawX/tests/unit/config_plan_domain_test.go
- [x] T005 校准 009 快速验收步骤与当前实现目标一致于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/quickstart.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成全部用户故事共享的控制面基建（统一计划存储、补丁模型、审计事件）

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [x] T006 定义 pending plan 与 patch 领域结构于 /home/ubuntu/workspace/ClawX/internal/application/configplan/model.go
- [x] T007 [P] 定义 plan store 接口与会话作用域键规则于 /home/ubuntu/workspace/ClawX/internal/application/configplan/store.go
- [x] T008 [P] 实现内存态会话级 plan store 于 /home/ubuntu/workspace/ClawX/internal/application/configplan/store_memory.go
- [x] T009 [P] 实现 patch 应用与版本递增逻辑于 /home/ubuntu/workspace/ClawX/internal/application/configplan/patcher.go
- [x] T010 [P] 实现配置控制面审计事件结构与格式化输出于 /home/ubuntu/workspace/ClawX/internal/application/configplan/audit.go
- [x] T011 在聊天入口统一封装配置控制面调度器于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T012 在配置帮助文案中补齐“自然语言 patch 与权限边界”说明于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T013 [P] 增加基础层单测（store/patch/version）于 /home/ubuntu/workspace/ClawX/tests/unit/config_plan_store_test.go
- [x] T014 [P] 增加基础层并发一致性单测于 /home/ubuntu/workspace/ClawX/tests/unit/config_plan_concurrency_test.go

**检查点**: 基础能力完成后，用户故事实现可开始

---

## Phase 3：用户故事 1 - 自然语言与指令统一进入配置计划（优先级：P1） 🎯 MVP

**目标**: `/config` 与自然语言两种入口统一落入同一 pending plan 生命周期

**独立验证**: 同会话内先发 `/config plan`、再发自然语言创建 agent，`/config show` 可看到统一计划

### 测试任务（US1）

- [x] T015 [P] [US1] 增加“指令创建计划”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_command_create_contract_test.go
- [x] T016 [P] [US1] 增加“自然语言创建计划”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_nl_create_contract_test.go
- [x] T017 [P] [US1] 增加“show 返回统一结构”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_show_contract_test.go
- [x] T018 [P] [US1] 增加双入口混合链路集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_plan_dual_entry_flow_test.go

### 实现任务（US1）

- [x] T019 [US1] 实现自然语言创建计划解析入口于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T020 [US1] 将指令入口与自然语言入口统一写入同一 plan store 于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T021 [US1] 实现 show 输出统一摘要（含来源 source）于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T022 [US1] 在渠道入站路径保留配置命令优先级并复用统一处理器于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [x] T023 [US1] 增加统一入口日志字段（plan_source/plan_id）于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T024 [US1] 增加 US1 回归测试聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_regression_suite_test.go

**检查点**: US1 完成后，双入口应已连贯可用

---

## Phase 4：用户故事 2 - 自然语言可编辑已有计划（优先级：P1）

**目标**: 支持会话内自然语言 patch 编辑 pending plan，并完整记录 patch 历史

**独立验证**: 创建计划后发送“改 workspace/timeout”，`/config show` 反映最终值与变更轨迹

### 测试任务（US2）

- [x] T025 [P] [US2] 增加自然语言 patch 字段映射契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_patch_mapping_contract_test.go
- [x] T026 [P] [US2] 增加“无 pending plan 禁止 patch”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_no_pending_patch_contract_test.go
- [x] T027 [P] [US2] 增加“同字段多次 patch 最终值一致”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_plan_patch_overwrite_flow_test.go
- [x] T028 [P] [US2] 增加 patch 历史完整保留集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_plan_patch_history_flow_test.go

### 实现任务（US2）

- [x] T029 [US2] 实现自然语言 patch 解析器（workspace/profile/timeout/default）于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T030 [US2] 将 patch 操作接入统一 patcher 与版本控制于 /home/ubuntu/workspace/ClawX/internal/application/configplan/patcher.go
- [x] T031 [US2] 实现“无计划不可 patch”错误语义与反馈于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T032 [US2] 实现 show 输出中 patch 轨迹摘要于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T033 [US2] 增加 US2 回归测试聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_regression_suite_test.go

**检查点**: US2 完成后，计划编辑闭环应可独立验收

---

## Phase 5：用户故事 3 - 所有配置落盘都走确认与权限治理（优先级：P1）

**目标**: 仅 `/config apply` 可写盘；非管理员不可 apply；未 apply 前配置文件不变

**独立验证**: 非管理员 apply 被拒绝；管理员 apply 成功；未 apply 前配置无变化

### 测试任务（US3）

- [x] T034 [P] [US3] 增加“仅 apply 可写盘”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_apply_only_write_contract_test.go
- [x] T035 [P] [US3] 增加“非管理员 apply 拒绝”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_apply_permission_contract_test.go
- [x] T036 [P] [US3] 增加“管理员 apply 成功写盘”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_apply_success_flow_test.go
- [x] T037 [P] [US3] 增加“未 apply 不落盘”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_no_apply_no_write_flow_test.go

### 实现任务（US3）

- [x] T038 [US3] 在 apply 路径固化唯一写盘出口于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T039 [US3] 实现非管理员“可建改看取消、不可 apply”权限边界于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T040 [US3] apply 成功后结束计划生命周期并清理活动计划于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T041 [US3] apply 失败场景返回标准错误语义（权限/无效计划/写盘失败）于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T042 [US3] 增加 US3 回归测试聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_regression_suite_test.go

**检查点**: US3 完成后，治理边界应完全可测

---

## Phase 6：用户故事 4 - 配置意图链路可审计、可追踪（优先级：P2）

**目标**: 配置计划全生命周期产生可检索审计事件与差异摘要

**独立验证**: 走“创建->patch->cancel”与“创建->patch->apply”两条链路，可检索完整事件

### 测试任务（US4）

- [x] T043 [P] [US4] 增加计划创建/补丁/取消/应用事件覆盖契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_audit_event_contract_test.go
- [x] T044 [P] [US4] 增加 patch/apply diff 摘要完整性契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_audit_diff_contract_test.go
- [x] T045 [P] [US4] 增加双链路审计可追踪集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_audit_lifecycle_flow_test.go

### 实现任务（US4）

- [x] T046 [US4] 在计划创建时记录 plan_created 审计事件于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T047 [US4] 在 patch/cancel/apply/reject 路径记录对应审计事件于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T048 [US4] 输出统一审计字段（conversation_scope/plan_id/actor/source/diff_summary）于 /home/ubuntu/workspace/ClawX/internal/application/configplan/audit.go
- [x] T049 [US4] 在日志中补充配置控制面关键指标埋点于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [x] T050 [US4] 增加 US4 回归测试聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_regression_suite_test.go

**检查点**: US4 完成后，应满足可观测与可追溯目标

---

## Phase 7：收尾与跨领域事项

**目的**: 回归、文档收口、门禁验证

- [x] T051 [P] 更新 009 契约文档与实际行为一致性于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/contracts/config-intent-contract.md
- [x] T052 [P] 更新 009 生命周期契约与权限语义一致性于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/contracts/config-plan-lifecycle-contract.md
- [x] T053 [P] 更新 009 quickstart 的最终验收步骤于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/quickstart.md
- [x] T054 运行 009 相关测试并修复回归（contract/integration/unit）于 /home/ubuntu/workspace/ClawX/tests/
- [x] T055 [P] 执行全量回归 `go test ./...` 并记录结果于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/
- [x] T056 [P] 补充 009 交付摘要（任务完成度与 SC 对照）于 /home/ubuntu/workspace/ClawX/specs/009-config-intent-plan/tasks.md

---

## Phase 8：一致性缺口补齐（Analyze Top Issues）

**目的**: 补齐 analyze 识别的高优先缺口（FR-018/FR-019/FR-022/SC-006）

- [x] T057 [P] [US1] 增加“混合意图先澄清”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_intent_clarify_conflict_contract_test.go
- [x] T058 [P] [US1] 增加“混合意图澄清后分流”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_clarify_flow_test.go
- [x] T059 [US1] 在配置入口实现混合意图澄清状态机于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T060 [P] [US1] 增加“低置信度仅建议”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_intent_low_confidence_contract_test.go
- [x] T061 [P] [US1] 增加“低置信度不改计划且回退可用”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_low_confidence_fallback_test.go
- [x] T062 [US1] 在配置入口实现低置信度 suggest-only 策略于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T063 [P] [US3] 增加“非管理员可建改看取消”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_non_admin_allowed_ops_contract_test.go
- [x] T064 [P] [US3] 增加“非管理员正向操作链路”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_non_admin_allowed_flow_test.go
- [x] T065 [P] 增加 SC-006 交互步数统计门禁测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_plan_metrics_step_count_test.go
- [x] T066 增加“交互步数”指标采集与输出于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go

---

## Phase 9：Agent Context 压缩机制对齐（P6.5）

**目的**: 对齐 `docs/plans/phase_6_agent_context_intent_unification.md` 中“控制面上下文压缩”要求，补齐结构化摘要、展示与回放一致性门禁

- [x] T067 [P] [US2] 增加“摘要由 patch 历史确定性生成”单元测试于 /home/ubuntu/workspace/ClawX/tests/unit/config_plan_summary_projector_test.go
- [x] T068 [P] [US2] 增加“摘要失效后重建”单元测试于 /home/ubuntu/workspace/ClawX/tests/unit/config_plan_summary_rebuild_test.go
- [x] T069 [P] [US2] 增加“show 同时返回摘要与最近轨迹”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_show_summary_contract_test.go
- [x] T070 [P] [US2] 增加“多轮 patch 后意图连续性”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_intent_context_continuity_test.go
- [x] T071 [US2] 在领域层新增控制面摘要模型与版本号于 /home/ubuntu/workspace/ClawX/internal/application/configplan/model.go
- [x] T072 [US2] 新增摘要投影器（patch history -> summary）于 /home/ubuntu/workspace/ClawX/internal/application/configplan/projector.go
- [x] T073 [US2] 新增摘要增量更新与重建逻辑于 /home/ubuntu/workspace/ClawX/internal/application/configplan/summary.go
- [x] T074 [US2] 在 patch/apply/cancel 路径接入摘要维护与一致性校验于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T075 [US1] 在 `/config show` 输出加入摘要版本、字段概览与最近轨迹窗口于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_chat.go
- [x] T076 [US4] 在审计事件补充 `summary_version/summary_rebuild_reason` 字段于 /home/ubuntu/workspace/ClawX/internal/application/configplan/audit.go
- [x] T077 [P] [US4] 增加“摘要与最终 apply 一致性”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/config_plan_summary_apply_consistency_contract_test.go
- [x] T078 [P] [US4] 增加“高 patch 数 show 性能门禁”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/config_plan_show_performance_test.go

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-6**: 依赖 Phase 2，建议顺序 US1 -> US2 -> US3 -> US4
- **Phase 7**: 依赖已完成的用户故事
- **Phase 8**: 依赖 Phase 7，补齐一致性缺口后再进入全面实现
- **Phase 9**: 依赖 Phase 8，用于补齐 Agent Context 压缩机制与一致性门禁

### 用户故事依赖

- **US1 (P1)**: Foundation 后可独立开始
- **US2 (P1)**: 依赖 US1 的统一计划入口
- **US3 (P1)**: 依赖 US1/US2 的计划生命周期能力
- **US4 (P2)**: 依赖 US1~US3 的事件与状态闭环

### 并行机会

- Phase 1: T002/T003/T004 可并行
- Phase 2: T007/T008/T009/T010/T013/T014 可并行
- US1: T015/T016/T017/T018 可并行
- US2: T025/T026/T027/T028 可并行
- US3: T034/T035/T036/T037 可并行
- US4: T043/T044/T045 可并行
- Phase 7: T051/T052/T053/T055 可并行
- Phase 8: T057/T058/T060/T061/T063/T064/T065 可并行
- Phase 9: T067/T068/T069/T070/T077/T078 可并行

---

## 并行执行示例

### US1 并行示例

```bash
Task: "T015 [US1] 指令创建计划契约测试"
Task: "T016 [US1] 自然语言创建计划契约测试"
Task: "T017 [US1] show 统一结构契约测试"
Task: "T018 [US1] 双入口混合链路集成测试"
```

### US2 并行示例

```bash
Task: "T025 [US2] patch 字段映射契约测试"
Task: "T026 [US2] 无 pending plan 禁止 patch 契约测试"
Task: "T027 [US2] 同字段多次 patch 集成测试"
Task: "T028 [US2] patch 历史完整保留集成测试"
```

### US3 并行示例

```bash
Task: "T034 [US3] 仅 apply 可写盘契约测试"
Task: "T035 [US3] 非管理员 apply 拒绝契约测试"
Task: "T036 [US3] 管理员 apply 成功集成测试"
Task: "T037 [US3] 未 apply 不落盘集成测试"
```

---

## 实施策略

### MVP 优先（US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即执行 US1 独立验收

### 增量交付

1. 先交付双入口统一（US1）
2. 再交付自然语言 patch（US2）
3. 再交付权限治理（US3）
4. 最后交付审计闭环（US4）

### 推荐 MVP 范围

- 推荐 MVP：**US1**
- 该范围可最早验证“指令与自然语言融合是否打通”

---

## 备注

- 总任务数：78
- US1 任务数：16
- US2 任务数：17
- US3 任务数：11
- US4 任务数：12
- 并行机会：36+

---

## 交付摘要（2026-03-18）

### 已完成任务（本轮）

- Phase 8: `T057 ~ T066` 已完成并落地（混合意图澄清、低置信建议、非管理员正向链路、交互步数指标与门禁测试）。
- Phase 7: `T051 ~ T056` 已完成（契约/quickstart 对齐、009 专项回归、全量回归与交付摘要）。

### SC 对照

- `SC-001`：通过自然语言创建与 patch 行为测试覆盖（`cmd/clawx/config_chat_test.go`）。
- `SC-002`：通过“仅 apply 写盘”测试覆盖（`TestHandleConfigChatCommandApplyIsOnlyWritePath`）。
- `SC-003`：通过“非配置消息回退”测试覆盖（`TestHandleConfigChatCommandNaturalLanguageNonConfigFallsBack`）。
- `SC-004`：通过“非管理员不可 apply、可建改看取消”测试覆盖。
- `SC-005`：创建/补丁/取消/应用/拒绝路径均记录审计事件。
- `SC-006`：通过交互步数门禁测试覆盖（目标链路步数=4）。

### 回归记录

- 009 专项：`go test ./tests/contract ./tests/integration ./cmd/clawx`（通过）
- 全量回归：`go test ./...`（通过）
