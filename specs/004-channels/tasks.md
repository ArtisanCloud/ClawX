# 任务清单：第四阶段渠道扩展

**输入**: `/home/ubuntu/workspace/SynapseX/specs/004-channels/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本阶段包含显式安全与兼容目标，任务清单包含单元/集成/契约测试任务。

**组织方式**: 任务按用户故事分组，保证每个故事可独立实现和独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/SynapseX/cmd/`、`/home/ubuntu/workspace/SynapseX/internal/`
- 测试路径：`/home/ubuntu/workspace/SynapseX/tests/`
- 文档路径：`/home/ubuntu/workspace/SynapseX/docs/`、`/home/ubuntu/workspace/SynapseX/specs/004-channels/`

## Phase 1：初始化（共享基础）

**目的**: 建立 Phase 4 文档与测试骨架，不改动核心业务路径

- [ ] T001 创建渠道扩展集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels_phase4_smoke_test.go
- [ ] T002 [P] 创建渠道扩展契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels_contract_test.go
- [ ] T003 [P] 创建 Phase 4 验证文档占位于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_4/phase_4_validation.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的渠道运行时与配置基础

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [ ] T004 扩展渠道实例配置模型支持 `feishu` 与 `wecom` 于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [ ] T005 [P] 扩展配置校验支持多渠道必填项与模式检查于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [ ] T006 [P] 增加多渠道配置单元测试于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config_test.go
- [ ] T007 实现统一渠道适配器错误重试编排（通道级隔离）于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T008 [P] 增加“单渠道失败不退出主进程”集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channels_runtime_isolation_test.go
- [ ] T009 实现 `synapsex config channel <name>` 命令骨架于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config_channel.go
- [ ] T010 [P] 增加增量配置回写测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_config_incremental_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - Telegram 双模式稳定运行（优先级：P1） 🎯 MVP

**目标**: Telegram polling/webhook 双模式可用，安全校验生效，渠道故障不拖垮服务

**独立验证**: polling 与 webhook 各自完成 `/new`、普通消息、`/list`，并验证失败重试与主进程存活

### 测试任务（US1）

- [ ] T011 [P] [US1] 增加 Telegram webhook 请求校验单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter_test.go
- [ ] T012 [P] [US1] 增加 Telegram 双模式路由集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/telegram_dual_mode_test.go
- [ ] T013 [P] [US1] 增加 Telegram 适配器重试与隔离测试于 /home/ubuntu/workspace/SynapseX/tests/integration/telegram_runtime_retry_test.go

### 实现任务（US1）

- [ ] T014 [US1] 完善 Telegram `webhookPath` 路由挂载与解析于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T015 [US1] 完善 Telegram webhook 鉴权与非法方法拒绝于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter.go
- [ ] T016 [US1] 完善 Telegram setWebhook 启动注册与错误重试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter.go
- [ ] T017 [US1] 对齐 Telegram 配置向导模式选择与字段提示于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config.go
- [ ] T018 [US1] 同步 Telegram 接入文档（polling/webhook 双模式）于 /home/ubuntu/workspace/SynapseX/docs/guides/telegram_bot_setup.md

**检查点**: US1 完成后，应可独立发布 Telegram 双模式能力

---

## Phase 4：用户故事 2 - Feishu 接入与统一命令语义（优先级：P2）

**目标**: Feishu 可作为独立窗口入口，challenge 与验签通过，控制命令语义一致

**独立验证**: Feishu 可完成 challenge、文本消息处理和 `/new` `/list` `/current` 命令链路

### 测试任务（US2）

- [ ] T019 [P] [US2] 增加 Feishu challenge 与签名校验单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/adapter_test.go
- [ ] T020 [P] [US2] 增加 Feishu 控制命令集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/feishu_control_flow_test.go
- [ ] T021 [P] [US2] 增加 Feishu 消息归一化测试于 /home/ubuntu/workspace/SynapseX/tests/unit/feishu_normalize_test.go

### 实现任务（US2）

- [ ] T022 [US2] 新增 Feishu 适配器核心实现于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/adapter.go
- [ ] T023 [US2] 新增 Feishu webhook handler 与 challenge 处理于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T024 [US2] 将 Feishu 文本事件映射到统一消息模型于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T025 [US2] 扩展渠道路由装配支持 Feishu 实例于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T026 [US2] 更新 Feishu 接入指南于 /home/ubuntu/workspace/SynapseX/docs/guides/feishu_bot_setup.md

**检查点**: US1 与 US2 均应可独立通过验收

---

## Phase 5：用户故事 3 - WeCom 接入与增量配置闭环（优先级：P3）

**目标**: WeCom 接入可用，支持 URL 验证与解密，并完成增量配置闭环

**独立验证**: WeCom 可完成 URL 验证、文本消息控制流；`config channel` 仅改目标渠道配置

### 测试任务（US3）

- [ ] T027 [P] [US3] 增加 WeCom URL 验证与签名解密单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/adapter_test.go
- [ ] T028 [P] [US3] 增加 WeCom 控制命令集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/wecom_control_flow_test.go
- [ ] T029 [P] [US3] 增加增量配置“非目标渠道不覆盖”集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_config_incremental_test.go
- [ ] T030 [P] [US3] 增加跨渠道命令语义一致性契约测试于 /home/ubuntu/workspace/SynapseX/tests/contract/channel_control_semantics_contract_test.go

### 实现任务（US3）

- [ ] T031 [US3] 新增 WeCom 适配器核心实现于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/adapter.go
- [ ] T032 [US3] 新增 WeCom URL 验证与回调处理路由于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T033 [US3] 将 WeCom 文本消息映射到统一消息模型于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T034 [US3] 完成 `config channel telegram|feishu|wecom` 增量交互流程于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config_channel.go
- [ ] T035 [US3] 完成配置补丁写入与原子保存于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [ ] T036 [US3] 新增 WeCom 接入指南于 /home/ubuntu/workspace/SynapseX/docs/guides/wecom_bot_setup.md
- [ ] T037 [US3] 拆分并精简 Telegram 配置文档为“创建 bot”与“增量配置”两篇于 /home/ubuntu/workspace/SynapseX/docs/guides/telegram_bot_setup.md 和 /home/ubuntu/workspace/SynapseX/docs/guides/telegram_config_incremental.md

**检查点**: 三个用户故事都可独立验收并演示

---

## Phase 6：收尾与跨领域事项

**目的**: 完成跨渠道一致性、文档闭环与发布门禁检查

- [ ] T038 [P] 补充 Telegram 契约文档细节于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/telegram-webhook-contract.md
- [ ] T039 [P] 补充 Feishu 契约文档细节于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/feishu-event-contract.md
- [ ] T040 [P] 补充 WeCom 契约文档细节于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/wecom-event-contract.md
- [ ] T041 [P] 补充跨渠道控制命令契约说明于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/channel-control-contract.md
- [ ] T042 运行全量回归并修复问题（`go test ./...`）于 /home/ubuntu/workspace/SynapseX/tests/
- [ ] T043 [P] 生成 Phase 4 指标采集测试（SC-001~SC-006）于 /home/ubuntu/workspace/SynapseX/tests/integration/channels_metrics_report_test.go
- [ ] T044 更新 Phase 4 快速验证与人工验收脚本于 /home/ubuntu/workspace/SynapseX/specs/004-channels/quickstart.md
- [ ] T045 回写 Phase 4 交付状态与风险于 /home/ubuntu/workspace/SynapseX/docs/plans/phase_4_channels.md

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-5**: 依赖 Phase 2；建议顺序 US1 -> US2 -> US3
- **Phase 6**: 依赖已实现的用户故事

### 用户故事依赖

- **US1 (P1)**: 无故事级前置依赖，可作为 MVP
- **US2 (P2)**: 依赖 US1 的渠道稳定基线
- **US3 (P3)**: 依赖 US1/US2 的统一语义与配置基础

### 并行机会

- Phase 1：T002 与 T003 可并行
- Phase 2：T005、T006、T008、T010 可并行
- US1：T011、T012、T013 可并行
- US2：T019、T020、T021 可并行
- US3：T027、T028、T029、T030 可并行
- Phase 6：T038、T039、T040、T041、T043 可并行

---

## 并行执行示例

### 用户故事 2

```bash
任务: "增加 Feishu challenge 与签名校验单元测试于 internal/interfaces/chat/feishu/adapter_test.go"
任务: "新增 Feishu 适配器核心实现于 internal/interfaces/chat/feishu/adapter.go"
```

### 用户故事 3

```bash
任务: "新增 WeCom 适配器核心实现于 internal/interfaces/chat/wecom/adapter.go"
任务: "完成 config channel telegram|feishu|wecom 增量交互流程于 cmd/synapsex/config_channel.go"
```

---

## 实施策略

### MVP 优先（仅 US1）

1. 完成 Phase 1（初始化）
2. 完成 Phase 2（基础能力）
3. 完成 Phase 3（US1）
4. 先发布 Telegram 双模式稳定性能力

### 增量交付

1. Telegram 双模式与容错先落地
2. 接入 Feishu 并回归统一命令语义
3. 接入 WeCom 与增量配置闭环
4. 最后补齐契约、文档、指标与发布门禁

### 推荐 MVP 范围

- 推荐 MVP：**US1（Telegram 双模式稳定运行）**

---

## 备注

- 总任务数：45
- US1 任务数：8
- US2 任务数：8
- US3 任务数：11
- 并行机会：10+（见各阶段 `[P]` 标记）
