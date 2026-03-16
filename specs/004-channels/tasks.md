# 任务清单：第四阶段渠道扩展

**输入**: `/home/ubuntu/workspace/ClawX/specs/004-channels/` 下的设计文档  
**前置条件**: `plan.md`（必需）、`spec.md`（必需）、`research.md`、`data-model.md`、`contracts/`、`quickstart.md`

**测试**: 本阶段包含显式安全与兼容目标，任务清单包含单元/集成/契约测试任务。

**组织方式**: 任务按用户故事分组，保证每个故事可独立实现和独立验收。

## 格式：`[ID] [P?] [Story] 描述`

- **[P]**: 可并行执行（不同文件、无未完成依赖）
- **[Story]**: 所属用户故事（`US1`、`US2`、`US3`、`US4`）
- 每条任务包含明确文件路径

## 路径约定

- 代码路径：`/home/ubuntu/workspace/ClawX/cmd/`、`/home/ubuntu/workspace/ClawX/internal/`
- 测试路径：`/home/ubuntu/workspace/ClawX/tests/`
- 文档路径：`/home/ubuntu/workspace/ClawX/docs/`、`/home/ubuntu/workspace/ClawX/specs/004-channels/`

## Phase 1：初始化（共享基础）

**目的**: 建立 Phase 4 文档与测试骨架，不改动核心业务路径

- [X] T001 创建渠道扩展集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels_phase4_smoke_test.go
- [X] T002 [P] 创建渠道扩展契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels_contract_test.go
- [X] T003 [P] 创建 Phase 4 验证文档占位于 /home/ubuntu/workspace/ClawX/docs/guides/phase_4/phase_4_validation.md

---

## Phase 2：基础能力（阻塞前置）

**目的**: 完成所有用户故事共享的渠道运行时与配置基础

**⚠️ 关键说明**: 本阶段完成前不得进入用户故事实现

- [X] T004 扩展渠道实例配置模型支持 `feishu` 与 `wecom` 于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T005 [P] 扩展配置校验支持多渠道必填项与模式检查于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T006 [P] 增加多渠道配置单元测试于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config_test.go
- [X] T007 实现统一渠道适配器错误重试编排（通道级隔离）于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T008 [P] 增加“单渠道失败不退出主进程”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/channels_runtime_isolation_test.go
- [X] T009 实现 `clawx config channel <name>` 命令骨架于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_channel.go
- [X] T010 [P] 增加增量配置回写测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channel_config_incremental_test.go

**检查点**: 基础能力完成后，用户故事实现可以开始

---

## Phase 3：用户故事 1 - Telegram 双模式稳定运行（优先级：P1） 🎯 MVP

**目标**: Telegram polling/webhook 双模式可用，安全校验生效，渠道故障不拖垮服务

**独立验证**: polling 与 webhook 各自完成 `/new`、普通消息、`/list`，并验证失败重试与主进程存活

### 测试任务（US1）

- [X] T011 [P] [US1] 增加 Telegram webhook 请求校验单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/telegram/adapter_test.go
- [X] T012 [P] [US1] 增加 Telegram 双模式路由集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/telegram_dual_mode_test.go
- [X] T013 [P] [US1] 增加 Telegram 适配器重试与隔离测试于 /home/ubuntu/workspace/ClawX/tests/integration/telegram_runtime_retry_test.go

### 实现任务（US1）

- [X] T014 [US1] 完善 Telegram `webhookPath` 路由挂载与解析于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T015 [US1] 完善 Telegram webhook 鉴权与非法方法拒绝于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/telegram/adapter.go
- [X] T016 [US1] 完善 Telegram setWebhook 启动注册与错误重试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/telegram/adapter.go
- [X] T017 [US1] 对齐 Telegram 配置向导模式选择与字段提示于 /home/ubuntu/workspace/ClawX/cmd/clawx/config.go
- [X] T018 [US1] 同步 Telegram 接入文档（polling/webhook 双模式）于 /home/ubuntu/workspace/ClawX/docs/guides/telegram_bot_setup.md

**检查点**: US1 完成后，应可独立发布 Telegram 双模式能力

---

## Phase 4：用户故事 2 - Feishu 接入与统一命令语义（优先级：P2）

**目标**: Feishu 可作为独立窗口入口，challenge 与验签通过，控制命令语义一致

**独立验证**: Feishu 可完成 challenge、文本消息处理和 `/new` `/list` `/current` 命令链路

### 测试任务（US2）

- [X] T019 [P] [US2] 增加 Feishu challenge 与签名校验单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/feishu/adapter_test.go
- [X] T020 [P] [US2] 增加 Feishu 控制命令全集集成测试（`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/ClawX/tests/integration/feishu_control_flow_test.go
- [X] T021 [P] [US2] 增加 Feishu 消息归一化测试于 /home/ubuntu/workspace/ClawX/tests/unit/feishu_normalize_test.go

### 实现任务（US2）

- [X] T022 [US2] 新增 Feishu 适配器核心实现于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/feishu/adapter.go
- [X] T023 [US2] 新增 Feishu webhook handler 与 challenge 处理于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T024 [US2] 将 Feishu 文本事件映射到统一消息模型于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/normalize.go
- [X] T025 [US2] 扩展渠道路由装配支持 Feishu 实例于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T026 [US2] 更新 Feishu 接入指南于 /home/ubuntu/workspace/ClawX/docs/guides/feishu_bot_setup.md

**检查点**: US1 与 US2 均应可独立通过验收

---

## Phase 5：用户故事 3 - WeCom 接入与增量配置闭环（优先级：P3）

**目标**: WeCom 接入可用，支持 URL 验证与解密，并完成增量配置闭环

**独立验证**: WeCom 可完成 URL 验证、文本消息控制流；`config channel` 仅改目标渠道配置

### 测试任务（US3）

- [X] T027 [P] [US3] 增加 WeCom URL 验证与签名解密单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/wecom/adapter_test.go
- [X] T028 [P] [US3] 增加 WeCom 控制命令全集集成测试（`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/ClawX/tests/integration/wecom_control_flow_test.go
- [X] T029 [P] [US3] 增加增量配置“非目标渠道不覆盖”集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_config_incremental_test.go
- [X] T030 [P] [US3] 增加跨渠道命令语义一致性契约测试（对齐 `/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）于 /home/ubuntu/workspace/ClawX/tests/contract/channel_control_semantics_contract_test.go

### 实现任务（US3）

- [X] T031 [US3] 新增 WeCom 适配器核心实现于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/wecom/adapter.go
- [X] T032 [US3] 新增 WeCom URL 验证与回调处理路由于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T033 [US3] 将 WeCom 文本消息映射到统一消息模型于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/normalize.go
- [X] T034 [US3] 完成 `config channel telegram|feishu|wecom` 增量交互流程于 /home/ubuntu/workspace/ClawX/cmd/clawx/config_channel.go
- [X] T035 [US3] 完成配置补丁写入与原子保存于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T036 [US3] 新增 WeCom 接入指南于 /home/ubuntu/workspace/ClawX/docs/guides/wecom_bot_setup.md
- [X] T037 [US3] 精简 Telegram 增量配置文档于 /home/ubuntu/workspace/ClawX/docs/guides/telegram_config_incremental.md

**检查点**: 三个用户故事都可独立验收并演示

---

## Phase 6：收尾与跨领域事项

**目的**: 完成跨渠道一致性、文档闭环与发布门禁检查

- [X] T038 [P] 补充 Telegram 契约文档细节于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/telegram-webhook-contract.md
- [X] T039 [P] 补充 Feishu 契约文档细节于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/feishu-event-contract.md
- [X] T040 [P] 补充 WeCom 契约文档细节于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/wecom-event-contract.md
- [X] T041 [P] 补充跨渠道控制命令契约说明于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/channel-control-contract.md
- [X] T042 运行全量回归并修复问题（`go test ./...`）于 /home/ubuntu/workspace/ClawX/tests/
- [X] T043 [P] 生成 Phase 4 指标采集测试（SC-001~SC-006）于 /home/ubuntu/workspace/ClawX/tests/integration/channels_metrics_report_test.go
- [X] T044 更新 Phase 4 快速验证与人工验收脚本于 /home/ubuntu/workspace/ClawX/specs/004-channels/quickstart.md
- [X] T045 回写 Phase 4 交付状态与风险于 /home/ubuntu/workspace/ClawX/docs/plans/phase_4_channels.md

---

## Phase 7：渠道对齐波次（外部基线，Wave 2~4）

**目的**: 将所有“未实现渠道”补齐到开发文档与技术规范，形成可执行 backlog

### Phase 7A：契约与测试门禁（必须先完成）

**顺序门禁**:
- 先完成渠道契约文件（T047~T066）与测试三联骨架（T067~T123），再开始渠道实现卡（T128~T146）。
- Wave 2/3/4 任何渠道若缺少“adapter unit / integration / contract”三联任务，不得进入实现。

- [X] T046 [US4] 新增渠道实现模板文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/_template.md
- [X] T047 [P] [US4] 新增 Slack 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/slack-event-contract.md
- [X] T048 [P] [US4] 新增 WhatsApp 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/whatsapp-event-contract.md
- [X] T049 [P] [US4] 新增 Signal 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/signal-bridge-contract.md
- [X] T050 [P] [US4] 新增 Google Chat 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/googlechat-event-contract.md
- [X] T051 [P] [US4] 新增 IRC 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/irc-gateway-contract.md
- [X] T052 [P] [US4] 新增 Matrix 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/matrix-event-contract.md
- [X] T053 [P] [US4] 新增 Mattermost 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/mattermost-event-contract.md
- [X] T054 [P] [US4] 新增 Microsoft Teams 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/msteams-event-contract.md
- [X] T055 [P] [US4] 新增 Nextcloud Talk 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/nextcloud-talk-contract.md
- [X] T056 [P] [US4] 新增 LINE 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/line-event-contract.md
- [X] T057 [P] [US4] 新增 Nostr 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/nostr-relay-contract.md
- [X] T058 [P] [US4] 新增 Synology Chat 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/synology-chat-contract.md
- [X] T059 [P] [US4] 新增 Twitch 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/twitch-eventsub-contract.md
- [X] T060 [P] [US4] 新增 Zalo 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/zalo-event-contract.md
- [X] T061 [P] [US4] 新增 Zalo Personal 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/zalouser-bridge-contract.md
- [X] T062 [P] [US4] 新增 BlueBubbles 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/bluebubbles-bridge-contract.md
- [X] T063 [P] [US4] 新增 iMessage legacy 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/imessage-legacy-contract.md
- [X] T064 [P] [US4] 新增 Tlon 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/tlon-bridge-contract.md
- [X] T065 [P] [US4] 新增 WebChat 契约文档于 /home/ubuntu/workspace/ClawX/specs/004-channels/contracts/webchat-gateway-contract.md
- [X] T066 [P] [US4] 更新 Wave 2~4 渠道对齐矩阵于 /home/ubuntu/workspace/ClawX/specs/004-channels/openclaw-channel-parity.md
- [X] T067 [P] [US4] 新增 Wave 2 Slack adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave2/slack_adapter_test.go
- [X] T068 [P] [US4] 新增 Wave 2 WhatsApp adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave2/whatsapp_adapter_test.go
- [X] T069 [P] [US4] 新增 Wave 2 Signal adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave2/signal_adapter_test.go
- [X] T070 [P] [US4] 新增 Wave 2 Google Chat adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave2/googlechat_adapter_test.go
- [X] T071 [P] [US4] 新增 Wave 2 IRC adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave2/irc_adapter_test.go
- [X] T072 [P] [US4] 新增 Wave 2 Slack 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave2/slack_control_flow_test.go
- [X] T073 [P] [US4] 新增 Wave 2 WhatsApp 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave2/whatsapp_control_flow_test.go
- [X] T074 [P] [US4] 新增 Wave 2 Signal 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave2/signal_control_flow_test.go
- [X] T075 [P] [US4] 新增 Wave 2 Google Chat 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave2/googlechat_control_flow_test.go
- [X] T076 [P] [US4] 新增 Wave 2 IRC 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave2/irc_control_flow_test.go
- [X] T077 [P] [US4] 新增 Wave 2 Slack 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave2/slack_contract_test.go
- [X] T078 [P] [US4] 新增 Wave 2 WhatsApp 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave2/whatsapp_contract_test.go
- [X] T079 [P] [US4] 新增 Wave 2 Signal 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave2/signal_contract_test.go
- [X] T080 [P] [US4] 新增 Wave 2 Google Chat 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave2/googlechat_contract_test.go
- [X] T081 [P] [US4] 新增 Wave 2 IRC 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave2/irc_contract_test.go
- [X] T082 [P] [US4] 新增 Wave 3 Matrix adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/matrix_adapter_test.go
- [X] T083 [P] [US4] 新增 Wave 3 Mattermost adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/mattermost_adapter_test.go
- [X] T084 [P] [US4] 新增 Wave 3 Microsoft Teams adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/msteams_adapter_test.go
- [X] T085 [P] [US4] 新增 Wave 3 Nextcloud Talk adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/nextcloud_talk_adapter_test.go
- [X] T086 [P] [US4] 新增 Wave 3 LINE adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/line_adapter_test.go
- [X] T087 [P] [US4] 新增 Wave 3 Nostr adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/nostr_adapter_test.go
- [X] T088 [P] [US4] 新增 Wave 3 Synology Chat adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/synology_chat_adapter_test.go
- [X] T089 [P] [US4] 新增 Wave 3 Twitch adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/twitch_adapter_test.go
- [X] T090 [P] [US4] 新增 Wave 3 Zalo adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/zalo_adapter_test.go
- [X] T091 [P] [US4] 新增 Wave 3 Zalo Personal adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave3/zalouser_adapter_test.go
- [X] T092 [P] [US4] 新增 Wave 3 Matrix 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/matrix_control_flow_test.go
- [X] T093 [P] [US4] 新增 Wave 3 Mattermost 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/mattermost_control_flow_test.go
- [X] T094 [P] [US4] 新增 Wave 3 Microsoft Teams 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/msteams_control_flow_test.go
- [X] T095 [P] [US4] 新增 Wave 3 Nextcloud Talk 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/nextcloud_talk_control_flow_test.go
- [X] T096 [P] [US4] 新增 Wave 3 LINE 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/line_control_flow_test.go
- [X] T097 [P] [US4] 新增 Wave 3 Nostr 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/nostr_control_flow_test.go
- [X] T098 [P] [US4] 新增 Wave 3 Synology Chat 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/synology_chat_control_flow_test.go
- [X] T099 [P] [US4] 新增 Wave 3 Twitch 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/twitch_control_flow_test.go
- [X] T100 [P] [US4] 新增 Wave 3 Zalo 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/zalo_control_flow_test.go
- [X] T101 [P] [US4] 新增 Wave 3 Zalo Personal 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave3/zalouser_control_flow_test.go
- [X] T102 [P] [US4] 新增 Wave 3 Matrix 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/matrix_contract_test.go
- [X] T103 [P] [US4] 新增 Wave 3 Mattermost 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/mattermost_contract_test.go
- [X] T104 [P] [US4] 新增 Wave 3 Microsoft Teams 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/msteams_contract_test.go
- [X] T105 [P] [US4] 新增 Wave 3 Nextcloud Talk 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/nextcloud_talk_contract_test.go
- [X] T106 [P] [US4] 新增 Wave 3 LINE 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/line_contract_test.go
- [X] T107 [P] [US4] 新增 Wave 3 Nostr 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/nostr_contract_test.go
- [X] T108 [P] [US4] 新增 Wave 3 Synology Chat 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/synology_chat_contract_test.go
- [X] T109 [P] [US4] 新增 Wave 3 Twitch 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/twitch_contract_test.go
- [X] T110 [P] [US4] 新增 Wave 3 Zalo 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/zalo_contract_test.go
- [X] T111 [P] [US4] 新增 Wave 3 Zalo Personal 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave3/zalouser_contract_test.go
- [X] T112 [P] [US4] 新增 Wave 4 BlueBubbles adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave4/bluebubbles_adapter_test.go
- [X] T113 [P] [US4] 新增 Wave 4 iMessage legacy adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave4/imessage_legacy_adapter_test.go
- [X] T114 [P] [US4] 新增 Wave 4 Tlon adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave4/tlon_adapter_test.go
- [X] T115 [P] [US4] 新增 Wave 4 WebChat adapter 单元测试骨架于 /home/ubuntu/workspace/ClawX/tests/unit/channels/wave4/webchat_adapter_test.go
- [X] T116 [P] [US4] 新增 Wave 4 BlueBubbles 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave4/bluebubbles_control_flow_test.go
- [X] T117 [P] [US4] 新增 Wave 4 iMessage legacy 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave4/imessage_legacy_control_flow_test.go
- [X] T118 [P] [US4] 新增 Wave 4 Tlon 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave4/tlon_control_flow_test.go
- [X] T119 [P] [US4] 新增 Wave 4 WebChat 控制流集成测试骨架于 /home/ubuntu/workspace/ClawX/tests/integration/channels/wave4/webchat_control_flow_test.go
- [X] T120 [P] [US4] 新增 Wave 4 BlueBubbles 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave4/bluebubbles_contract_test.go
- [X] T121 [P] [US4] 新增 Wave 4 iMessage legacy 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave4/imessage_legacy_contract_test.go
- [X] T122 [P] [US4] 新增 Wave 4 Tlon 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave4/tlon_contract_test.go
- [X] T123 [P] [US4] 新增 Wave 4 WebChat 语义契约测试骨架于 /home/ubuntu/workspace/ClawX/tests/contract/channels/wave4/webchat_contract_test.go
- [X] T124 [P] [US4] 增加 FR-021/FR-022 判定记录与治理检查清单于 /home/ubuntu/workspace/ClawX/docs/guides/phase_4/phase_4_spec_governance_checklist.md
- [X] T125 [P] [US4] 增加 FR-026 自动化门禁工作流于 /home/ubuntu/workspace/ClawX/.github/workflows/spec-governance.yml
- [X] T126 [P] [US4] 增加 FR-026 自动化门禁脚本于 /home/ubuntu/workspace/ClawX/scripts/ci/check_spec_governance.sh
- [X] T127 [US4] 汇总外部基线 Wave 2~4 验收脚本与门禁于 /home/ubuntu/workspace/ClawX/docs/guides/phase_4/phase_4_channel_parity_validation.md

### Phase 7B：渠道实现卡（在 7A 全量完成后执行）

- [X] T128 [P] [US4] 新增 Slack 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/slack.md
- [X] T129 [P] [US4] 新增 WhatsApp 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/whatsapp.md
- [X] T130 [P] [US4] 新增 Signal 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/signal.md
- [X] T131 [P] [US4] 新增 Google Chat 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/googlechat.md
- [X] T132 [P] [US4] 新增 IRC 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/irc.md
- [X] T133 [P] [US4] 新增 Matrix 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/matrix.md
- [X] T134 [P] [US4] 新增 Mattermost 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/mattermost.md
- [X] T135 [P] [US4] 新增 Microsoft Teams 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/msteams.md
- [X] T136 [P] [US4] 新增 Nextcloud Talk 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/nextcloud-talk.md
- [X] T137 [P] [US4] 新增 LINE 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/line.md
- [X] T138 [P] [US4] 新增 Nostr 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/nostr.md
- [X] T139 [P] [US4] 新增 Synology Chat 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/synology-chat.md
- [X] T140 [P] [US4] 新增 Twitch 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/twitch.md
- [X] T141 [P] [US4] 新增 Zalo 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/zalo.md
- [X] T142 [P] [US4] 新增 Zalo Personal 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/zalouser.md
- [X] T143 [P] [US4] 新增 BlueBubbles 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/bluebubbles.md
- [X] T144 [P] [US4] 新增 iMessage legacy 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/imessage-legacy.md
- [X] T145 [P] [US4] 新增 Tlon 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/tlon.md
- [X] T146 [P] [US4] 新增 WebChat 渠道实现卡于 /home/ubuntu/workspace/ClawX/specs/004-channels/channels/webchat.md
- [X] T147 [P] [US1] 增加 Phase 2 窗口语义回归契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/window_session_semantics_contract_test.go
- [X] T148 [P] [US1] 增加跨渠道窗口隔离回归集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_window_isolation_regression_test.go
- [X] T149 [US3] 增加审计日志字段写入实现（`channel`、`instance`、`event_id`、`intent.kind`、`duration_ms`）于 /home/ubuntu/workspace/ClawX/cmd/clawx/main.go
- [X] T150 [P] [US3] 增加审计日志字段断言测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_audit_fields_test.go
- [X] T151 [P] [US4] 增加 Backend Adapter 边界守护契约测试于 /home/ubuntu/workspace/ClawX/tests/contract/backend_adapter_boundary_contract_test.go
- [X] T152 [US4] 增加 Wave 2~4 配置模型扩展实现（`enabled/defaultAgent/instances`）于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config.go
- [X] T153 [P] [US4] 增加 FR-025 配置模型扩展单元测试于 /home/ubuntu/workspace/ClawX/internal/infrastructure/config/config_test.go
- [X] T154 [P] [US4] 增加 FR-025 配置模型扩展集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_config_incremental_test.go

**检查点**: Phase 7 完成后，未实现渠道必须 100% 在技术规范中具备独立契约、独立测试三联、独立实现卡

---

## Phase 8：安全与可测性补强（跨渠道）

**目的**: 补齐重放防护、日志字段断言与路由性能可测性，消除发布前高风险缺口

- [X] T155 [P] [US1] 增加 Telegram 重放请求防护单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/telegram/replay_test.go
- [X] T156 [US1] 实现 Telegram 重放请求防护于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/telegram/adapter.go
- [X] T157 [P] [US2] 增加 Feishu 重放请求防护单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/feishu/replay_test.go
- [X] T158 [US2] 实现 Feishu 重放请求防护于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/feishu/adapter.go
- [X] T159 [P] [US3] 增加 WeCom 重放请求防护单元测试于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/wecom/replay_test.go
- [X] T160 [US3] 实现 WeCom 重放请求防护于 /home/ubuntu/workspace/ClawX/internal/interfaces/chat/wecom/adapter.go
- [X] T161 [P] [US3] 增加跨渠道重放拒绝集成测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_replay_protection_test.go
- [X] T162 [P] [US1] 增加指数退避结构化日志字段断言测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_runtime_retry_log_fields_test.go
- [X] T163 [US1] 实现渠道路由耗时采集与聚合于 /home/ubuntu/workspace/ClawX/internal/application/service/channel_metrics.go
- [X] T164 [P] [US1] 增加渠道路由 p95 指标报告测试于 /home/ubuntu/workspace/ClawX/tests/integration/channel_route_latency_metrics_test.go

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
任务: "完成 config channel telegram|feishu|wecom 增量交互流程于 cmd/clawx/config_channel.go"
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
