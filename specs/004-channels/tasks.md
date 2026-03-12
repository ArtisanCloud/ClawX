# 任务清单：第四阶段渠道扩展

**输入**: `/home/ubuntu/workspace/SynapseX/specs/004-channels/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本阶段包含显式安全与兼容目标，任务清单包含单元/集成/契约测试任务。

**组织方式**: 任务按用户故事分组，保证每个故事可独立实现和独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/SynapseX/cmd/`、`/home/ubuntu/workspace/SynapseX/internal/`
- 测试路径：`/home/ubuntu/workspace/SynapseX/tests/`
- 文档路径：`/home/ubuntu/workspace/SynapseX/docs/`、`/home/ubuntu/workspace/SynapseX/specs/004-channels/`

## Phase 1：初始化（共享基础）

**目的**: 建立 Phase 4 文档与测试骨架，不改动核心业务路径

- [X] T001 创建渠道扩展集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels_phase4_smoke_test.go
- [X] T002 [P] 创建渠道扩展契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels_contract_test.go
- [X] T003 [P] 创建 Phase 4 验证文档占位于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_4/phase_4_validation.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的渠道运行时与配置基础

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [X] T004 扩展渠道实例配置模型支持 `feishu` 与 `wecom` 于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [X] T005 [P] 扩展配置校验支持多渠道必填项与模式检查于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [X] T006 [P] 增加多渠道配置单元测试于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config_test.go
- [X] T007 实现统一渠道适配器错误重试编排（通道级隔离）于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [X] T008 [P] 增加“单渠道失败不退出主进程”集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channels_runtime_isolation_test.go
- [X] T009 实现 `synapsex config channel <name>` 命令骨架于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config_channel.go
- [X] T010 [P] 增加增量配置回写测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_config_incremental_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - Telegram 双模式稳定运行（优先级：P1） 🎯 MVP

**目标**: Telegram polling/webhook 双模式可用，安全校验生效，渠道故障不拖垮服务

**独立验证**: polling 与 webhook 各自完成 `/new`、普通消息、`/list`，并验证失败重试与主进程存活

### 测试任务（US1）

- [X] T011 [P] [US1] 增加 Telegram webhook 请求校验单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter_test.go
- [X] T012 [P] [US1] 增加 Telegram 双模式路由集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/telegram_dual_mode_test.go
- [X] T013 [P] [US1] 增加 Telegram 适配器重试与隔离测试于 /home/ubuntu/workspace/SynapseX/tests/integration/telegram_runtime_retry_test.go

### 实现任务（US1）

- [X] T014 [US1] 完善 Telegram `webhookPath` 路由挂载与解析于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [X] T015 [US1] 完善 Telegram webhook 鉴权与非法方法拒绝于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter.go
- [X] T016 [US1] 完善 Telegram setWebhook 启动注册与错误重试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter.go
- [X] T017 [US1] 对齐 Telegram 配置向导模式选择与字段提示于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config.go
- [X] T018 [US1] 同步 Telegram 接入文档（polling/webhook 双模式）于 /home/ubuntu/workspace/SynapseX/docs/guides/telegram_bot_setup.md

**检查点**: US1 完成后，应可独立发布 Telegram 双模式能力

---

## Phase 4：用户故事 2 - Feishu 接入与统一命令语义（优先级：P2）

**目标**: Feishu 可作为独立窗口入口，challenge 与验签通过，控制命令语义一致

**独立验证**: Feishu 可完成 challenge、文本消息处理和 `/new` `/list` `/current` 命令链路

### 测试任务（US2）

- [X] T019 [P] [US2] 增加 Feishu challenge 与签名校验单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/adapter_test.go
- [X] T020 [P] [US2] 增加 Feishu 控制命令全集集成测试（`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/SynapseX/tests/integration/feishu_control_flow_test.go
- [X] T021 [P] [US2] 增加 Feishu 消息归一化测试于 /home/ubuntu/workspace/SynapseX/tests/unit/feishu_normalize_test.go

### 实现任务（US2）

- [X] T022 [US2] 新增 Feishu 适配器核心实现于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/adapter.go
- [X] T023 [US2] 新增 Feishu webhook handler 与 challenge 处理于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [X] T024 [US2] 将 Feishu 文本事件映射到统一消息模型于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [X] T025 [US2] 扩展渠道路由装配支持 Feishu 实例于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [X] T026 [US2] 更新 Feishu 接入指南于 /home/ubuntu/workspace/SynapseX/docs/guides/feishu_bot_setup.md

**检查点**: US1 与 US2 均应可独立通过验收

---

## Phase 5：用户故事 3 - WeCom 接入与增量配置闭环（优先级：P3）

**目标**: WeCom 接入可用，支持 URL 验证与解密，并完成增量配置闭环

**独立验证**: WeCom 可完成 URL 验证、文本消息控制流；`config channel` 仅改目标渠道配置

### 测试任务（US3）

- [ ] T027 [P] [US3] 增加 WeCom URL 验证与签名解密单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/adapter_test.go
- [ ] T028 [P] [US3] 增加 WeCom 控制命令全集集成测试（`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/SynapseX/tests/integration/wecom_control_flow_test.go
- [ ] T029 [P] [US3] 增加增量配置“非目标渠道不覆盖”集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_config_incremental_test.go
- [ ] T030 [P] [US3] 增加跨渠道命令语义一致性契约测试（对齐 `/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/SynapseX/tests/contract/channel_control_semantics_contract_test.go

### 实现任务（US3）

- [ ] T031 [US3] 新增 WeCom 适配器核心实现于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/adapter.go
- [ ] T032 [US3] 新增 WeCom URL 验证与回调处理路由于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T033 [US3] 将 WeCom 文本消息映射到统一消息模型于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/normalize.go
- [ ] T034 [US3] 完成 `config channel telegram|feishu|wecom` 增量交互流程于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/config_channel.go
- [ ] T035 [US3] 完成配置补丁写入与原子保存于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [ ] T036 [US3] 新增 WeCom 接入指南于 /home/ubuntu/workspace/SynapseX/docs/guides/wecom_bot_setup.md
- [ ] T037 [US3] 精简 Telegram 增量配置文档于 /home/ubuntu/workspace/SynapseX/docs/guides/telegram_config_incremental.md

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

## Phase 7：渠道对齐波次（外部基线，Wave 2~4）

**目的**: 将所有“未实现渠道”补齐到开发文档与技术规范，形成可执行 backlog

### Phase 7A：契约与测试门禁（必须先完成）

**顺序门禁**:
- 先完成渠道契约文件（T047~T066）与测试三联骨架（T067~T123），再开始渠道实现卡（T128~T146）。
- Wave 2/3/4 任何渠道若缺少“adapter unit / integration / contract”三联任务，不得进入实现。

- [ ] T046 [US4] 新增渠道实现模板文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/_template.md
- [ ] T047 [P] [US4] 新增 Slack 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/slack-event-contract.md
- [ ] T048 [P] [US4] 新增 WhatsApp 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/whatsapp-event-contract.md
- [ ] T049 [P] [US4] 新增 Signal 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/signal-bridge-contract.md
- [ ] T050 [P] [US4] 新增 Google Chat 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/googlechat-event-contract.md
- [ ] T051 [P] [US4] 新增 IRC 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/irc-gateway-contract.md
- [ ] T052 [P] [US4] 新增 Matrix 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/matrix-event-contract.md
- [ ] T053 [P] [US4] 新增 Mattermost 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/mattermost-event-contract.md
- [ ] T054 [P] [US4] 新增 Microsoft Teams 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/msteams-event-contract.md
- [ ] T055 [P] [US4] 新增 Nextcloud Talk 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/nextcloud-talk-contract.md
- [ ] T056 [P] [US4] 新增 LINE 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/line-event-contract.md
- [ ] T057 [P] [US4] 新增 Nostr 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/nostr-relay-contract.md
- [ ] T058 [P] [US4] 新增 Synology Chat 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/synology-chat-contract.md
- [ ] T059 [P] [US4] 新增 Twitch 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/twitch-eventsub-contract.md
- [ ] T060 [P] [US4] 新增 Zalo 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/zalo-event-contract.md
- [ ] T061 [P] [US4] 新增 Zalo Personal 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/zalouser-bridge-contract.md
- [ ] T062 [P] [US4] 新增 BlueBubbles 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/bluebubbles-bridge-contract.md
- [ ] T063 [P] [US4] 新增 iMessage legacy 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/imessage-legacy-contract.md
- [ ] T064 [P] [US4] 新增 Tlon 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/tlon-bridge-contract.md
- [ ] T065 [P] [US4] 新增 WebChat 契约文档于 /home/ubuntu/workspace/SynapseX/specs/004-channels/contracts/webchat-gateway-contract.md
- [ ] T066 [P] [US4] 更新 Wave 2~4 渠道对齐矩阵于 /home/ubuntu/workspace/SynapseX/specs/004-channels/openclaw-channel-parity.md
- [ ] T067 [P] [US4] 新增 Wave 2 Slack adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave2/slack_adapter_test.go
- [ ] T068 [P] [US4] 新增 Wave 2 WhatsApp adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave2/whatsapp_adapter_test.go
- [ ] T069 [P] [US4] 新增 Wave 2 Signal adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave2/signal_adapter_test.go
- [ ] T070 [P] [US4] 新增 Wave 2 Google Chat adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave2/googlechat_adapter_test.go
- [ ] T071 [P] [US4] 新增 Wave 2 IRC adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave2/irc_adapter_test.go
- [ ] T072 [P] [US4] 新增 Wave 2 Slack 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave2/slack_control_flow_test.go
- [ ] T073 [P] [US4] 新增 Wave 2 WhatsApp 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave2/whatsapp_control_flow_test.go
- [ ] T074 [P] [US4] 新增 Wave 2 Signal 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave2/signal_control_flow_test.go
- [ ] T075 [P] [US4] 新增 Wave 2 Google Chat 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave2/googlechat_control_flow_test.go
- [ ] T076 [P] [US4] 新增 Wave 2 IRC 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave2/irc_control_flow_test.go
- [ ] T077 [P] [US4] 新增 Wave 2 Slack 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave2/slack_contract_test.go
- [ ] T078 [P] [US4] 新增 Wave 2 WhatsApp 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave2/whatsapp_contract_test.go
- [ ] T079 [P] [US4] 新增 Wave 2 Signal 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave2/signal_contract_test.go
- [ ] T080 [P] [US4] 新增 Wave 2 Google Chat 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave2/googlechat_contract_test.go
- [ ] T081 [P] [US4] 新增 Wave 2 IRC 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave2/irc_contract_test.go
- [ ] T082 [P] [US4] 新增 Wave 3 Matrix adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/matrix_adapter_test.go
- [ ] T083 [P] [US4] 新增 Wave 3 Mattermost adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/mattermost_adapter_test.go
- [ ] T084 [P] [US4] 新增 Wave 3 Microsoft Teams adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/msteams_adapter_test.go
- [ ] T085 [P] [US4] 新增 Wave 3 Nextcloud Talk adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/nextcloud_talk_adapter_test.go
- [ ] T086 [P] [US4] 新增 Wave 3 LINE adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/line_adapter_test.go
- [ ] T087 [P] [US4] 新增 Wave 3 Nostr adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/nostr_adapter_test.go
- [ ] T088 [P] [US4] 新增 Wave 3 Synology Chat adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/synology_chat_adapter_test.go
- [ ] T089 [P] [US4] 新增 Wave 3 Twitch adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/twitch_adapter_test.go
- [ ] T090 [P] [US4] 新增 Wave 3 Zalo adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/zalo_adapter_test.go
- [ ] T091 [P] [US4] 新增 Wave 3 Zalo Personal adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave3/zalouser_adapter_test.go
- [ ] T092 [P] [US4] 新增 Wave 3 Matrix 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/matrix_control_flow_test.go
- [ ] T093 [P] [US4] 新增 Wave 3 Mattermost 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/mattermost_control_flow_test.go
- [ ] T094 [P] [US4] 新增 Wave 3 Microsoft Teams 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/msteams_control_flow_test.go
- [ ] T095 [P] [US4] 新增 Wave 3 Nextcloud Talk 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/nextcloud_talk_control_flow_test.go
- [ ] T096 [P] [US4] 新增 Wave 3 LINE 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/line_control_flow_test.go
- [ ] T097 [P] [US4] 新增 Wave 3 Nostr 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/nostr_control_flow_test.go
- [ ] T098 [P] [US4] 新增 Wave 3 Synology Chat 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/synology_chat_control_flow_test.go
- [ ] T099 [P] [US4] 新增 Wave 3 Twitch 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/twitch_control_flow_test.go
- [ ] T100 [P] [US4] 新增 Wave 3 Zalo 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/zalo_control_flow_test.go
- [ ] T101 [P] [US4] 新增 Wave 3 Zalo Personal 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave3/zalouser_control_flow_test.go
- [ ] T102 [P] [US4] 新增 Wave 3 Matrix 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/matrix_contract_test.go
- [ ] T103 [P] [US4] 新增 Wave 3 Mattermost 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/mattermost_contract_test.go
- [ ] T104 [P] [US4] 新增 Wave 3 Microsoft Teams 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/msteams_contract_test.go
- [ ] T105 [P] [US4] 新增 Wave 3 Nextcloud Talk 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/nextcloud_talk_contract_test.go
- [ ] T106 [P] [US4] 新增 Wave 3 LINE 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/line_contract_test.go
- [ ] T107 [P] [US4] 新增 Wave 3 Nostr 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/nostr_contract_test.go
- [ ] T108 [P] [US4] 新增 Wave 3 Synology Chat 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/synology_chat_contract_test.go
- [ ] T109 [P] [US4] 新增 Wave 3 Twitch 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/twitch_contract_test.go
- [ ] T110 [P] [US4] 新增 Wave 3 Zalo 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/zalo_contract_test.go
- [ ] T111 [P] [US4] 新增 Wave 3 Zalo Personal 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave3/zalouser_contract_test.go
- [ ] T112 [P] [US4] 新增 Wave 4 BlueBubbles adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave4/bluebubbles_adapter_test.go
- [ ] T113 [P] [US4] 新增 Wave 4 iMessage legacy adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave4/imessage_legacy_adapter_test.go
- [ ] T114 [P] [US4] 新增 Wave 4 Tlon adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave4/tlon_adapter_test.go
- [ ] T115 [P] [US4] 新增 Wave 4 WebChat adapter 单元测试骨架于 /home/ubuntu/workspace/SynapseX/tests/unit/channels/wave4/webchat_adapter_test.go
- [ ] T116 [P] [US4] 新增 Wave 4 BlueBubbles 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave4/bluebubbles_control_flow_test.go
- [ ] T117 [P] [US4] 新增 Wave 4 iMessage legacy 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave4/imessage_legacy_control_flow_test.go
- [ ] T118 [P] [US4] 新增 Wave 4 Tlon 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave4/tlon_control_flow_test.go
- [ ] T119 [P] [US4] 新增 Wave 4 WebChat 控制流集成测试骨架于 /home/ubuntu/workspace/SynapseX/tests/integration/channels/wave4/webchat_control_flow_test.go
- [ ] T120 [P] [US4] 新增 Wave 4 BlueBubbles 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave4/bluebubbles_contract_test.go
- [ ] T121 [P] [US4] 新增 Wave 4 iMessage legacy 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave4/imessage_legacy_contract_test.go
- [ ] T122 [P] [US4] 新增 Wave 4 Tlon 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave4/tlon_contract_test.go
- [ ] T123 [P] [US4] 新增 Wave 4 WebChat 语义契约测试骨架于 /home/ubuntu/workspace/SynapseX/tests/contract/channels/wave4/webchat_contract_test.go
- [ ] T124 [P] [US4] 增加 FR-021/FR-022 判定记录与治理检查清单于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_4/phase_4_spec_governance_checklist.md
- [ ] T125 [P] [US4] 增加 FR-026 自动化门禁工作流于 /home/ubuntu/workspace/SynapseX/.github/workflows/spec-governance.yml
- [ ] T126 [P] [US4] 增加 FR-026 自动化门禁脚本于 /home/ubuntu/workspace/SynapseX/scripts/ci/check_spec_governance.sh
- [ ] T127 [US4] 汇总外部基线 Wave 2~4 验收脚本与门禁于 /home/ubuntu/workspace/SynapseX/docs/guides/phase_4/phase_4_channel_parity_validation.md

### Phase 7B：渠道实现卡（在 7A 全量完成后执行）

- [ ] T128 [P] [US4] 新增 Slack 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/slack.md
- [ ] T129 [P] [US4] 新增 WhatsApp 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/whatsapp.md
- [ ] T130 [P] [US4] 新增 Signal 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/signal.md
- [ ] T131 [P] [US4] 新增 Google Chat 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/googlechat.md
- [ ] T132 [P] [US4] 新增 IRC 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/irc.md
- [ ] T133 [P] [US4] 新增 Matrix 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/matrix.md
- [ ] T134 [P] [US4] 新增 Mattermost 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/mattermost.md
- [ ] T135 [P] [US4] 新增 Microsoft Teams 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/msteams.md
- [ ] T136 [P] [US4] 新增 Nextcloud Talk 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/nextcloud-talk.md
- [ ] T137 [P] [US4] 新增 LINE 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/line.md
- [ ] T138 [P] [US4] 新增 Nostr 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/nostr.md
- [ ] T139 [P] [US4] 新增 Synology Chat 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/synology-chat.md
- [ ] T140 [P] [US4] 新增 Twitch 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/twitch.md
- [ ] T141 [P] [US4] 新增 Zalo 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/zalo.md
- [ ] T142 [P] [US4] 新增 Zalo Personal 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/zalouser.md
- [ ] T143 [P] [US4] 新增 BlueBubbles 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/bluebubbles.md
- [ ] T144 [P] [US4] 新增 iMessage legacy 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/imessage-legacy.md
- [ ] T145 [P] [US4] 新增 Tlon 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/tlon.md
- [ ] T146 [P] [US4] 新增 WebChat 渠道实现卡于 /home/ubuntu/workspace/SynapseX/specs/004-channels/channels/webchat.md
- [ ] T147 [P] [US1] 增加 Phase 2 窗口语义回归契约测试于 /home/ubuntu/workspace/SynapseX/tests/contract/window_session_semantics_contract_test.go
- [ ] T148 [P] [US1] 增加跨渠道窗口隔离回归集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_window_isolation_regression_test.go
- [ ] T149 [US3] 增加审计日志字段写入实现（`channel`、`instance`、`event_id`、`intent.kind`、`duration_ms`）于 /home/ubuntu/workspace/SynapseX/cmd/synapsex/main.go
- [ ] T150 [P] [US3] 增加审计日志字段断言测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_audit_fields_test.go
- [ ] T151 [P] [US4] 增加 Backend Adapter 边界守护契约测试于 /home/ubuntu/workspace/SynapseX/tests/contract/backend_adapter_boundary_contract_test.go
- [ ] T152 [US4] 增加 Wave 2~4 配置模型扩展实现（`enabled/defaultAgent/instances`）于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config.go
- [ ] T153 [P] [US4] 增加 FR-025 配置模型扩展单元测试于 /home/ubuntu/workspace/SynapseX/internal/infrastructure/config/config_test.go
- [ ] T154 [P] [US4] 增加 FR-025 配置模型扩展集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_config_incremental_test.go

**检查点**: Phase 7 完成后，未实现渠道必须 100% 在技术规范中具备独立契约、独立测试三联、独立实现卡

---

## Phase 8：安全与可测性补强（跨渠道）

**目的**: 补齐重放防护、日志字段断言与路由性能可测性，消除发布前高风险缺口

- [ ] T155 [P] [US1] 增加 Telegram 重放请求防护单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/replay_test.go
- [ ] T156 [US1] 实现 Telegram 重放请求防护于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/telegram/adapter.go
- [ ] T157 [P] [US2] 增加 Feishu 重放请求防护单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/replay_test.go
- [ ] T158 [US2] 实现 Feishu 重放请求防护于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/feishu/adapter.go
- [ ] T159 [P] [US3] 增加 WeCom 重放请求防护单元测试于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/replay_test.go
- [ ] T160 [US3] 实现 WeCom 重放请求防护于 /home/ubuntu/workspace/SynapseX/internal/interfaces/chat/wecom/adapter.go
- [ ] T161 [P] [US3] 增加跨渠道重放拒绝集成测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_replay_protection_test.go
- [ ] T162 [P] [US1] 增加指数退避结构化日志字段断言测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_runtime_retry_log_fields_test.go
- [ ] T163 [US1] 实现渠道路由耗时采集与聚合于 /home/ubuntu/workspace/SynapseX/internal/application/service/channel_metrics.go
- [ ] T164 [P] [US1] 增加渠道路由 p95 指标报告测试于 /home/ubuntu/workspace/SynapseX/tests/integration/channel_route_latency_metrics_test.go

---

## 依赖关系与执行顺序

### 阶段依赖

- **Phase 1**: 无依赖，可立即开始
- **Phase 2**: 依赖 Phase 1，阻塞全部用户故事
- **Phase 3-5**: 依赖 Phase 2；建议顺序 US1 -> US2 -> US3
- **Phase 6**: 依赖已实现的用户故事
- **Phase 7**: 依赖 Phase 6，作为 Wave 2~4 文档与规范前置（7A 完成后才能进入 7B）
- **Phase 8**: 依赖 Phase 3~7，作为发布前安全与可测性门禁

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
- Phase 7A：T047~T123 可并行（先于 7B）
- Phase 7B：T128~T146 可并行（仅在 7A 全量完成后）
- Phase 8：T155、T157、T159、T161、T162、T164 可并行

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
5. 按 Wave 2~4 扩展其余外部基线对齐渠道

### 推荐 MVP 范围

- 推荐 MVP：**US1（Telegram 双模式稳定运行）**

---

## 备注

- 总任务数：164
- US1 任务数：15
- US2 任务数：10
- US3 任务数：16
- US4 任务数：105
- 并行机会：100+（见各阶段 `[P]` 标记）
