# 任务清单：Skill Registry 与意图路由

**输入**: `specs/003-skill-intent-router/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本特性未要求严格 TDD；测试任务放在各用户故事收尾与最终 Polish 阶段，保证独立验收可执行。

**组织方式**: 任务按用户故事分组，确保每个故事可独立实现与验证。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`cmd/`、`internal/`、`tests/`
- 文档路径：`docs/features/skill_registry_intent_router/`

## Phase 1：初始化（共享基础）

**目的**: 建立 Skill Registry 与 Intent Router 的基础目录和入口挂载点

- [X] T001 创建技能与意图模块目录骨架于 internal/domain/skill/、internal/application/intent/、internal/application/skillregistry/、internal/infrastructure/skills/
- [X] T002 [P] 新增技能与意图配置结构占位于 internal/infrastructure/config/config.go
- [X] T003 [P] 新增技能相关 CLI 子命令入口骨架于 cmd/clawx/skill_cli.go
- [X] T004 在程序启动流程中预留 Skill Registry 初始化与热刷新挂载点于 cmd/clawx/main.go

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的核心能力；此阶段完成前不得进入用户故事实现

- [X] T005 实现 Skill 定义与目录条目领域模型于 internal/domain/skill/skill.go
- [X] T006 [P] 实现路由决策领域模型（kind/reason/confidence）于 internal/domain/skill/intent_decision.go
- [X] T007 [P] 实现 Skill 注册快照与状态枚举于 internal/domain/skill/registry_snapshot.go
- [X] T008 实现 Skill 索引文件读写与原子替换于 internal/infrastructure/skills/index_store.go
- [X] T009 实现 Skill Registry 应用服务接口与基础刷新流程于 internal/application/skillregistry/service.go
- [X] T010 在 Router 中注入 Skill Registry 与 Intent Router 依赖于 internal/application/service/router.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - 可复用现有 Skill 资产（优先级：P1） 🎯 MVP

**目标**: 支持加载 Claude Code 规范 Skill，并可查询状态与刷新结果

**独立验证**: 放入合法与非法 `SKILL.md` 后，系统可正确显示 `active/invalid/shadowed/disabled` 状态

- [X] T011 [P] [US1] 实现 `SKILL.md` frontmatter 解析与校验于 internal/infrastructure/skills/manifest_parser.go
- [X] T012 [P] [US1] 实现多来源扫描（user/workspace/builtin）于 internal/infrastructure/skills/discovery.go
- [X] T013 [US1] 实现同名冲突决策与 `shadowed` 标记逻辑于 internal/application/skillregistry/conflict_resolver.go
- [X] T014 [US1] 实现 Registry 刷新编排与快照发布于 internal/application/skillregistry/refresh.go
- [X] T015 [US1] 实现 `skill list` 命令输出于 cmd/clawx/skill_cli.go
- [X] T016 [US1] 实现 `skill reload` 命令与运行时生效于 cmd/clawx/skill_cli.go
- [X] T017 [US1] 补充 US1 集成测试（加载/冲突/状态）于 tests/integration/skill_registry_load_test.go

**检查点**: US1 完成后，应可独立演示 Skill 发现、校验与状态可观测

---

## Phase 4：用户故事 2 - 消息稳定分流到正确处理路径（优先级：P2）

**目标**: 将消息稳定路由到控制命令、Skill 执行或普通任务，并遵循固定优先级

**独立验证**: 同一会话下发送控制命令、显式 `/skill`、普通文本，路由结果符合优先级且多候选自动单选

- [X] T018 [P] [US2] 实现意图路由优先级流水线于 internal/application/intent/pipeline.go
- [X] T019 [P] [US2] 实现显式 `/skill <name>` 解析于 internal/application/intent/explicit_skill.go
- [X] T020 [P] [US2] 实现精确名与别名匹配器于 internal/application/intent/rule_matcher.go
- [X] T021 [US2] 实现 LLM 兜底匹配接口与默认实现于 internal/application/intent/llm_fallback.go
- [X] T022 [US2] 实现多候选自动选择规则（禁止二次询问）于 internal/application/intent/selector.go
- [X] T023 [US2] 将 Intent Router 接入现有路由主流程于 internal/application/service/router.go
- [X] T024 [US2] 在 Discord/Telegram 入站处理中接入 Skill 路径分支于 cmd/clawx/main.go
- [X] T025 [US2] 补充 US2 集成测试（优先级、回退、多候选自动选择）于 tests/integration/intent_router_flow_test.go
- [X] T026 [US2] 增加 LLM 兜底阈值配置与默认值（0.72）于 internal/infrastructure/config/config.go 和 internal/application/intent/llm_fallback.go
- [X] T027 [US2] 补充低置信阈值回退集成测试于 tests/integration/intent_router_threshold_test.go

**检查点**: US2 完成后，应可独立验证三类路径分流与自动选择策略

---

## Phase 5：用户故事 3 - Skill 使用受权限与治理约束（优先级：P3）

**目标**: 实现默认“频道白名单 + DM pairing”与 Skill 级启停治理

**独立验证**: 未授权用户/频道触发 Skill 被拒绝，禁用 Skill 无法执行，授权后可正常执行

- [X] T028 [P] [US3] 扩展技能权限配置结构（enabled/disabled/allowlist/default_mode）于 internal/infrastructure/config/config.go
- [X] T029 [P] [US3] 实现 Skill 权限评估器于 internal/application/intent/permission_checker.go
- [X] T030 [US3] 在路由流程中接入权限拒绝分支与统一错误信息于 internal/application/service/router.go
- [X] T031 [US3] 实现 `skill enable <name>` 命令于 cmd/clawx/skill_cli.go
- [X] T032 [US3] 实现 `skill disable <name>` 命令于 cmd/clawx/skill_cli.go
- [X] T033 [US3] 增加路由审计日志字段（intent.kind/reason/skill/confidence）于 internal/application/service/router.go
- [X] T034 [US3] 补充 US3 集成测试（默认策略、白名单、禁用）于 tests/integration/skill_permission_policy_test.go
- [X] T035 [US3] 实现拒绝错误分类映射（permission_denied/skill_not_found/skill_disabled/skill_invalid/pairing_expired）于 internal/application/intent/permission_checker.go 和 internal/interfaces/chat/error_response.go
- [X] T036 [US3] 实现 DM pairing 生命周期（pending/paired/expired/revoked）于 internal/infrastructure/skills/pairing_store.go 和 internal/application/intent/permission_checker.go
- [X] T037 [US3] 补充 DM pairing 生命周期集成测试于 tests/integration/skill_dm_pairing_lifecycle_test.go

**检查点**: US3 完成后，应可独立验证治理与权限边界

---

## Phase 6：收尾与跨领域事项

**目的**: 完成跨故事一致性、文档与回归校验

- [X] T038 [P] 补充 Intent Router 契约测试于 tests/contract/intent_router_contract_test.go
- [X] T039 [P] 补充 Skill Registry 契约测试于 tests/contract/skill_registry_contract_test.go
- [X] T040 同步更新特性文档中的实现状态与示例于 docs/features/skill_registry_intent_router/implementation.md
- [X] T041 [P] 同步更新快速上手与验收步骤于 specs/003-skill-intent-router/quickstart.md
- [X] T042 运行全量回归并修复问题于 tests/integration/、tests/unit/（执行 `go test ./...`）
- [X] T043 输出阶段交付摘要与风险清单于 specs/003-skill-intent-router/plan.md
- [X] T044 [P] 补充控制命令不回归测试（`/new`、`/resume`、`/list`、`/cancel`）于 tests/integration/control_commands_regression_test.go
- [X] T045 [P] 补充路由判定耗时与刷新生效性能验证于 tests/integration/intent_router_performance_test.go

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-5**: 依赖 Phase 2；按优先级建议顺序 US1 -> US2 -> US3
- **Phase 6**: 依赖已实现的用户故事

### 用户故事依赖

- **US1 (P1)**: 无故事级前置依赖，可在 Foundation 后先落地（MVP）
- **US2 (P2)**: 依赖 US1 已提供可用 Skill 快照数据
- **US3 (P3)**: 依赖 US1/US2 的执行与路由路径可用

### 并行机会

- Phase 1 中 T002 与 T003 可并行
- Phase 2 中 T006 与 T007 可并行
- US1 中 T011 与 T012 可并行
- US2 中 T018、T019、T020 可并行
- US3 中 T026 与 T027 可并行
- US2 中 T026 与 T027 可并行
- US3 中 T036 与 T037 可并行
- Phase 6 中 T038、T039、T041、T044、T045 可并行

---

## 并行执行示例

### 用户故事 1

```bash
任务: "实现 SKILL.md frontmatter 解析与校验于 internal/infrastructure/skills/manifest_parser.go"
任务: "实现多来源扫描（user/workspace/builtin）于 internal/infrastructure/skills/discovery.go"
```

### 用户故事 2

```bash
任务: "实现意图路由优先级流水线于 internal/application/intent/pipeline.go"
任务: "实现显式 /skill <name> 解析于 internal/application/intent/explicit_skill.go"
任务: "实现精确名与别名匹配器于 internal/application/intent/rule_matcher.go"
```

---

## 实施策略

### MVP 优先（仅 US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 立即进行独立验收（Skill 发现、校验、状态展示）

### 增量交付

1. US1 完成后先交付可复用 Skill 能力
2. 再加入 US2，交付稳定分流能力
3. 最后加入 US3，交付治理与权限能力
4. 在 Phase 6 完成契约测试、文档与回归收尾

### 推荐 MVP 范围

- 推荐 MVP：**US1（Phase 3）**
- 该范围能最早验证“Claude Skill 兼容 + 注册可见性”的核心价值

---

## 备注

- 总任务数：45
- US1 任务数：7
- US2 任务数：10
- US3 任务数：10
- 并行机会：10+（见各阶段 `[P]` 标记）
