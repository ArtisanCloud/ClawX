# 任务清单：Skill Governance Orchestration（技能治理与编排）

**输入**: `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/` 下的设计文档  
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
- 文档路径：`/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/`

## Phase 1：初始化（共享基础）

**目的**: 建立 010 功能骨架与测试入口

- [x] T001 创建 010 回归入口测试文件于 /home/ubuntu/workspace/ClawX/tests/integration/skill_orchestrator_regression_suite_test.go
- [x] T002 [P] 创建技能注册与策略契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/skill_registry_policy_contract_test.go
- [x] T003 [P] 创建技能路由 action 契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/skill_routing_action_contract_test.go
- [x] T004 [P] 创建技能绑定解析契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/skill_binding_resolution_contract_test.go
- [x] T005 [P] 创建技能治理领域单测骨架于 /home/ubuntu/workspace/ClawX/tests/unit/skill_governance_domain_test.go
- [x] T006 校准 010 quickstart 验收步骤于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/quickstart.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成全部用户故事共享的技能控制面基建（模型、存储、策略、执行器）

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [x] T007 定义 Skill 元数据与版本模型于 /home/ubuntu/workspace/ClawX/internal/domain/skill/model.go
- [x] T008 [P] 定义 Skill Policy 模型与校验规则于 /home/ubuntu/workspace/ClawX/internal/domain/skill/policy.go
- [x] T009 [P] 定义 Skill Binding 模型与作用域常量于 /home/ubuntu/workspace/ClawX/internal/domain/skill/binding.go
- [x] T010 [P] 定义 SkillAction 结构与 schema 校验器（含 risk 枚举 `low|medium|high`）于 /home/ubuntu/workspace/ClawX/internal/domain/skill/action.go
- [x] T011 [P] 定义 Skill 审计事件模型于 /home/ubuntu/workspace/ClawX/internal/domain/skill/audit.go
- [x] T012 定义 registry/policy/binding 仓储接口于 /home/ubuntu/workspace/ClawX/internal/domain/skill/repository.go
- [x] T013 [P] 实现文件态 Skill Registry 仓储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/skill_registry_file_store.go
- [x] T014 [P] 实现文件态 Skill Policy 仓储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/skill_policy_file_store.go
- [x] T015 [P] 实现文件态 Skill Binding 仓储于 /home/ubuntu/workspace/ClawX/internal/infrastructure/persistence/skill_binding_file_store.go
- [x] T016 [P] 实现策略引擎（source/version/enable）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/policy_engine.go
- [x] T077 [P] 实现受限能力清单校验（禁止通用 marketplace 能力穿透）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/policy_engine.go
- [x] T017 [P] 实现绑定解析器（agent-local > project > global）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/binding_resolver.go
- [x] T018 [P] 实现统一 Skill Executor（权限/参数/执行）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/executor.go
- [x] T019 在主进程装配 Skill Orchestrator 依赖于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [x] T020 [P] 增加基础层单测（model/policy/binding/action）于 /home/ubuntu/workspace/ClawX/tests/unit/skill_governance_domain_test.go
- [x] T021 [P] 增加文件仓储读写与并发单测于 /home/ubuntu/workspace/ClawX/tests/unit/skill_store_test.go

**检查点**: 基础能力完成后，用户故事实现可开始

---

## Phase 3：用户故事 1 - 自然语言优先的技能路由（优先级：P1） 🎯 MVP

**目标**: 自然语言默认走 LLM 路由并生成结构化 SkillAction，命令入口映射到同一 action schema

**独立验证**: 同一会话中自然语言与命令请求得到等价 action，低置信度不执行

### 测试任务（US1）

- [x] T022 [P] [US1] 增加自然语言到 SkillAction 契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_routing_action_contract_test.go
- [x] T023 [P] [US1] 增加命令到 SkillAction 映射契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_command_action_mapping_contract_test.go
- [x] T024 [P] [US1] 增加低置信度澄清契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_low_confidence_contract_test.go
- [x] T078 [P] [US1] 增加“非技能意图回退原任务链路”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_non_intent_fallback_contract_test.go
- [x] T025 [P] [US1] 增加自然语言路由正向集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_nl_routing_flow_test.go
- [x] T026 [P] [US1] 增加命令与自然语言混合链路集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_dual_entry_flow_test.go
- [x] T079 [P] [US1] 增加“非技能意图回退原任务链路”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_non_intent_fallback_flow_test.go

### 实现任务（US1）

- [x] T027 [US1] 实现 Skill Intent Router（LLM-first）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/router.go
- [x] T028 [US1] 实现命令入口到 SkillAction 映射于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/command_mapper.go
- [x] T029 [US1] 在 chat 控制入口接入技能路由调度于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [x] T030 [US1] 实现低置信度澄清与建议响应于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/clarify.go
- [x] T080 [US1] 实现路由置信度阈值配置加载（默认 0.70）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/router.go
- [x] T081 [US1] 实现未命中技能意图回退原任务链路于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/router.go
- [x] T031 [US1] 统一输出路由审计字段（intent/skill_id/confidence/source）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/audit_logger.go
- [x] T032 [US1] 增加 US1 回归聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/skill_orchestrator_regression_suite_test.go

**检查点**: US1 完成后，自然语言主路径应可独立演示

---

## Phase 4：用户故事 2 - 技能注册与策略治理（优先级：P1）

**目标**: 支持技能注册、安装、升级、禁用及策略拒绝语义

**独立验证**: 白名单源可装、非白名单拒绝、版本策略生效、禁用即时生效

### 测试任务（US2）

- [x] T033 [P] [US2] 增加 registry 元数据必填字段契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_registry_metadata_contract_test.go
- [x] T034 [P] [US2] 增加 source 白名单策略契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_policy_source_contract_test.go
- [x] T035 [P] [US2] 增加版本策略（pin/compatible/latest）契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_policy_version_contract_test.go
- [x] T036 [P] [US2] 增加禁用技能即时生效契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_disable_contract_test.go
- [x] T037 [P] [US2] 增加安装/升级治理集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_registry_policy_flow_test.go

### 实现任务（US2）

- [x] T038 [US2] 实现 Skill Registry 服务（register/list/get）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/registry_service.go
- [x] T039 [US2] 实现 Skill Install 服务（source/version 校验）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/install_service.go
- [x] T040 [US2] 实现 Skill Upgrade 服务（版本策略）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/upgrade_service.go
- [x] T041 [US2] 实现 Skill Disable/Enable 服务于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/toggle_service.go
- [x] T042 [US2] 实现策略拒绝错误码与用户反馈映射于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/error_mapper.go
- [x] T043 [US2] 在 skill 控制入口帮助文案补齐技能治理说明于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/help_text.go
- [x] T082 [P] [US2] 增加“009 agent 状态读取”契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_agent_state_integration_contract_test.go
- [x] T083 [US2] 实现 009 agent 状态读取适配器（workspace/profile/default）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/agent_state_provider.go
- [x] T084 [US2] 在 orchestrator 装配 agent 状态提供器于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [x] T044 [US2] 增加 US2 回归聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/skill_orchestrator_regression_suite_test.go

**检查点**: US2 完成后，技能治理边界可独立验收

---

## Phase 5：用户故事 3 - 多 Agent 技能共享与隔离（优先级：P1）

**目标**: 支持 global/project/agent-local 三层绑定与确定性解析

**独立验证**: 同名技能在三层冲突时按既定优先级解析，删除上层后可回落

### 测试任务（US3）

- [x] T045 [P] [US3] 增加三层绑定优先级契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_binding_resolution_contract_test.go
- [x] T046 [P] [US3] 增加同名技能冲突回落契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_binding_fallback_contract_test.go
- [x] T047 [P] [US3] 增加按 agent/project 查询有效技能视图契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_effective_view_contract_test.go
- [x] T048 [P] [US3] 增加多 agent 共享与隔离集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_binding_scope_flow_test.go

### 实现任务（US3）

- [x] T049 [US3] 实现 Skill Binding 服务（bind/unbind/list）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/binding_service.go
- [x] T050 [US3] 实现有效技能视图聚合器于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/effective_view.go
- [x] T051 [US3] 实现绑定冲突解释器（可读原因）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/resolution_explain.go
- [x] T052 [US3] 在执行前接入作用域解析与权限校验于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/executor.go
- [x] T053 [US3] 增加 US3 回归聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/skill_orchestrator_regression_suite_test.go

**检查点**: US3 完成后，共享/隔离策略应完全可预测

---

## Phase 6：用户故事 4 - 高风险确认、审计与回放（优先级：P2）

**目标**: 高风险技能 confirm-first，执行链路全量审计并可回放

**独立验证**: 高风险执行需确认，拒绝确认无副作用，审计可还原 action 与执行结果

### 测试任务（US4）

- [x] T054 [P] [US4] 增加高风险确认门禁契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_risk_confirm_contract_test.go
- [x] T055 [P] [US4] 增加确认拒绝无副作用契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_confirm_reject_contract_test.go
- [x] T056 [P] [US4] 增加执行审计字段完整性契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_audit_event_contract_test.go
- [x] T057 [P] [US4] 增加审计回放一致性契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_audit_replay_contract_test.go
- [x] T058 [P] [US4] 增加高风险执行端到端集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_risk_execution_flow_test.go

### 实现任务（US4）

- [x] T059 [US4] 实现风险分级判定与确认状态机于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/risk_guard.go
- [x] T060 [US4] 在执行链路接入确认门禁于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/executor.go
- [x] T061 [US4] 实现统一审计写入（决策输入/action/执行结果）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/audit_service.go
- [x] T062 [US4] 实现审计回放查询接口于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/audit_replay.go
- [x] T063 [US4] 增加 US4 回归聚合入口于 /home/ubuntu/workspace/ClawX/tests/integration/skill_orchestrator_regression_suite_test.go

**检查点**: US4 完成后，治理与可追溯目标可独立验收

---

## Phase 7：上下文压缩与性能门禁（跨故事）

**目的**: 落实 skill 维度 context digest 与路由性能指标

- [x] T064 [P] 新增 Skill Context Digest 模型与版本字段于 /home/ubuntu/workspace/ClawX/internal/domain/skill/context_digest.go
- [x] T065 [P] 实现调用轨迹到 digest 的投影器于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/digest_projector.go
- [x] T066 [P] 实现摘要失效重建逻辑于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/digest_rebuild.go
- [x] T067 [P] 在路由输入接入 `context_digest + skill_catalog_digest` 于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/router.go
- [x] T085 [P] 实现 `skill_catalog_digest` 构建器于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/catalog_digest.go
- [x] T086 [P] 增加 `skill_catalog_digest` 一致性单元测试于 /home/ubuntu/workspace/ClawX/tests/unit/skill_catalog_digest_test.go
- [x] T068 [P] 增加 digest 一致性单元测试于 /home/ubuntu/workspace/ClawX/tests/unit/skill_context_digest_test.go
- [x] T069 [P] 增加大规模 catalog 路由性能集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_routing_performance_test.go

---

## Phase 8：收尾与跨领域事项

**目的**: 文档收口、全量回归、SC 对照验收

- [x] T070 [P] 更新 010 契约文档与最终行为一致于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-registry-policy-contract.md
- [x] T071 [P] 更新 010 路由契约文档与最终行为一致于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-routing-action-contract.md
- [x] T072 [P] 更新 010 绑定契约文档与最终行为一致于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-binding-resolution-contract.md
- [x] T073 [P] 更新 010 quickstart 的最终验收步骤于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/quickstart.md
- [x] T074 运行 010 专项测试并修复回归于 /home/ubuntu/workspace/ClawX/tests/
- [x] T075 [P] 执行全量回归 `go test ./...` 并记录结果于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/
- [x] T076 [P] 补充 010 交付摘要（任务完成度与 SC 对照）于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/tasks.md

---

## 010 交付摘要（T076）

### 任务完成度

- 总任务数：86
- 已完成：86
- 未完成：0
- 完成率：100%

### 关键交付物

- 契约文档：
  - `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-registry-policy-contract.md`
  - `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-routing-action-contract.md`
  - `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/contracts/skill-binding-resolution-contract.md`
- 最终验收脚本：
  - `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/quickstart.md`
- 验证记录：
  - `/home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/verification.md`

### SC 对照

- SC-001（NL action 解析成功率）：
  - 由 `skill_routing_action` + `skill_nl_routing_flow` + `skill_dual_entry_flow` 验证，达成。
- SC-002（高风险未确认执行率 0%）：
  - 由 `skill_risk_confirm_contract` + `skill_risk_execution_flow` 验证，达成。
- SC-003（策略拒绝准确率）：
  - 由 `skill_policy_source_contract` + `skill_policy_version_contract` 验证，达成。
- SC-004（执行审计覆盖率）：
  - 由 `skill_audit_event_contract` + `skill_audit_replay_contract` 验证，达成。
- SC-005（三层绑定解析一致性）：
  - 由 `skill_binding_resolution_contract` + `skill_binding_fallback_contract` + `skill_binding_scope_flow` 验证，达成。
- SC-006（低置信度误执行率）：
  - 由 `skill_low_confidence_contract` + `skill_non_intent_fallback` 验证，达成。
- SC-007（摘要构建/路由性能门禁）：
  - 由 `skill_routing_performance_test` 验证，达成（中位数 <= 300ms）。

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-6**: 依赖 Phase 2，建议顺序 US1 -> US2 -> US3 -> US4
- **Phase 7**: 依赖 Phase 3-6 的执行记录与路由输入链路
- **Phase 8**: 依赖前述阶段全部完成

### 用户故事依赖

- **US1 (P1)**: Foundation 后可独立开始
- **US2 (P1)**: 依赖 US1 的统一 SkillAction 路由/执行链路
- **US3 (P1)**: 依赖 US2 的 registry/policy 可用
- **US4 (P2)**: 依赖 US1~US3 的 action 与执行闭环

### 并行机会

- Phase 1: T002/T003/T004/T005 可并行
- Phase 2: T008/T009/T010/T011/T013/T014/T015/T016/T017/T018/T020/T021 可并行
- US1: T022/T023/T024/T025/T026 可并行
- US2: T033/T034/T035/T036/T037 可并行
- US3: T045/T046/T047/T048 可并行
- US4: T054/T055/T056/T057/T058 可并行
- Phase 7: T064/T065/T066/T067/T068/T069 可并行
- Phase 8: T070/T071/T072/T073/T075/T076 可并行

---

## 实施策略

### MVP 优先（US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即执行 US1 独立验收

### 增量交付

1. 先交付自然语言 LLM-first 路由（US1）
2. 再交付 registry/policy 治理（US2）
3. 再交付共享与隔离绑定（US3）
4. 最后交付高风险确认与回放审计（US4）

### 推荐 MVP 范围

- 推荐 MVP：**US1**
- 该范围可最早验证“自然语言主路径 + 结构化 action”是否打通

---

## 备注

- 总任务数：86
- US1 任务数：16
- US2 任务数：16
- US3 任务数：9
- US4 任务数：10
- 并行机会：45+

## Phase 9 完成度（Post-MVP）

- 任务数：14（T087-T100）
- 已完成：14
- 未完成：0
- 完成率：100%

---

## Phase 9：Post-MVP 增量（内置技能资产与安装器）

**目的**: 补齐可直接使用的内置技能包（含 web 能力）与第三方技能真实安装流水线  
**说明**: 本阶段为 010 后续增量，不计入 T076 的 MVP 完成率

### 测试任务（US2/US3 增量）

- [x] T087 [P] [US2] 增加内置技能发现契约测试（builtin 目录可见）于 /home/ubuntu/workspace/ClawX/tests/contract/skill_builtin_discovery_contract_test.go
- [x] T088 [P] [US2] 增加 `web-search` 技能清单字段契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_builtin_web_search_contract_test.go
- [x] T089 [P] [US2] 增加 `web-fetch` 技能清单字段契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/skill_builtin_web_fetch_contract_test.go
- [x] T090 [P] [US2] 增加第三方技能安装流程集成测试（下载/解包/登记）于 /home/ubuntu/workspace/ClawX/tests/integration/skill_marketplace_install_flow_test.go
- [x] T091 [P] [US3] 增加“全局内置技能 + agent 覆盖”解析集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/skill_builtin_override_resolution_flow_test.go

### 实现任务（US2/US3 增量）

- [x] T092 [US2] 新增内置技能目录与基础技能包骨架于 /home/ubuntu/workspace/ClawX/internal/skills/builtin/
- [x] T093 [US2] 落地 `web-search` 内置技能包（SKILL.md + metadata）于 /home/ubuntu/workspace/ClawX/internal/skills/builtin/web-search/SKILL.md
- [x] T094 [US2] 落地 `web-fetch` 内置技能包（SKILL.md + metadata）于 /home/ubuntu/workspace/ClawX/internal/skills/builtin/web-fetch/SKILL.md
- [x] T095 [US2] 修正 builtin 路径解析（确保从任意 cwd 稳定发现）于 /home/ubuntu/workspace/ClawX/cmd/clawx/skill_cli.go
- [x] T096 [US2] 实现第三方技能下载与解包安装器（受策略白名单约束）于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/install_service.go
- [x] T097 [US2] 实现安装后索引刷新与失败回滚于 /home/ubuntu/workspace/ClawX/internal/application/skillregistry/refresh.go
- [x] T098 [US3] 增加用户目录/工作区目录/内置目录优先级说明与冲突可视化于 /home/ubuntu/workspace/ClawX/internal/application/skillorchestrator/resolution_explain.go

### 文档与验收任务

- [x] T099 [P] 更新 quickstart：补充 builtin web 技能启用与验证脚本于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/quickstart.md
- [x] T100 [P] 更新 verification：记录 Post-MVP 回归结果于 /home/ubuntu/workspace/ClawX/specs/010-skill-governance-orchestration/verification.md
