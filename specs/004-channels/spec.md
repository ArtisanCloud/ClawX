# 功能规格说明：第四阶段渠道扩展

**功能分支**: `004-channels`  
**创建时间**: 2026-03-10  
**状态**: 草稿  
**输入**: 用户描述: “按 docs/plans/phase_4_channels.md 落地第四阶段规格，覆盖 Telegram webhook、Feishu、WeCom，并同步外部基线渠道对齐清单，补齐未实现渠道的技术规范与实施路线。”

## Clarifications

### Session 2026-03-10

- Q: Telegram 默认运行模式是什么？ → A: 默认 `polling`，可选 `webhook`。
- Q: 单个渠道故障是否允许导致主进程退出？ → A: 不允许，必须通道级隔离并自动重试。
- Q: 增量配置命令是否纳入本阶段？ → A: 纳入，支持 `clawx config channel <name>`。
- Q: 多渠道是否要求控制命令语义完全一致？ → A: 要求一致，至少覆盖 `/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`。

### Session 2026-03-11

- Q: Phase 4 是否只覆盖 Telegram/Feishu/WeCom？ → A: 不是。它们是 Wave 1 基线，其他外部基线渠道必须纳入同一规格的对齐路线。
- Q: 未实现渠道是否需要进入技术规范？ → A: 需要，至少明确分波次、统一适配模板、契约与测试要求。

## 渠道对齐范围（外部基线映射，2026-03-11）

对齐矩阵见：`/home/ubuntu/workspace/ClawX/specs/004-channels/openclaw-channel-parity.md`

- 已实现：Discord、Telegram（polling/webhook）
- Wave 1（Phase 4 基线）：Feishu、WeCom
- Wave 2（主流通用）：Slack、WhatsApp、Signal、Google Chat、IRC
- Wave 3（插件/企业扩展）：Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal
- Wave 4（可选）：BlueBubbles、iMessage legacy、Tlon、WebChat

## 用户场景与测试（必填）

### 用户故事 1 - Telegram 支持 polling/webhook 双模式并稳定运行（优先级：P1）

作为系统管理员，我希望 Telegram 既支持 polling 也支持 webhook，并且在网络抖动时不会把服务整体拖垮，这样我可以按部署环境选择模式并保持可用性。

**为什么是这个优先级**: Telegram 已是现网入口，模式能力与稳定性直接影响主链路可用性，是本阶段 MVP。

**独立测试方式**: 在同一版本分别以 polling 与 webhook 启动 Telegram，执行控制命令与普通消息，验证消息可达、验签有效、单渠道故障不影响其他渠道。

**验收场景**:

1. **假如** Telegram 配置为 `polling`，**当** 用户发送消息时，**那么** 系统应正常收取并路由，不要求 webhook 依赖。
2. **假如** Telegram 配置为 `webhook` 且验签通过，**当** 回调请求到达时，**那么** 系统应完成消息处理并返回成功响应。
3. **假如** Telegram 网络异常或代理连接重置，**当** 适配器遇到错误时，**那么** 系统应仅重启 Telegram 适配器并保持主进程存活。

---

### 用户故事 2 - Feishu 可作为独立窗口入口接入（优先级：P2）

作为团队用户，我希望能在飞书里直接使用 ClawX，并复用现有会话与控制命令行为，这样无需切换到其他渠道也能完成任务。

**为什么是这个优先级**: 这是新增企业渠道中最常见诉求之一，优先级高于 WeCom 但低于 Telegram 稳定性。

**独立测试方式**: 配置一个 Feishu Bot 实例，完成 challenge 校验、消息收发、控制命令与普通任务路由，确认窗口会话隔离与 Discord/Telegram 一致。

**验收场景**:

1. **假如** Feishu 回调首次验证 challenge，**当** 校验请求到达时，**那么** 系统应按协议返回 challenge。
2. **假如** Feishu 用户发送 `/new` 与普通文本，**当** 系统处理后续消息时，**那么** 应进入与既有渠道一致的控制与执行路径。
3. **假如** Feishu 回调签名无效，**当** 请求进入服务时，**那么** 系统应拒绝请求并记录安全日志。

---

### 用户故事 3 - WeCom 接入并提供统一增量配置体验（优先级：P3）

作为运维负责人，我希望企业微信入口与其他渠道共享同一套命令语义，并能通过增量配置命令单独配置渠道，这样上线和变更更可控。

**为什么是这个优先级**: WeCom 是关键企业渠道，但可以在 Telegram/Feishu 基线稳定后交付。

**独立测试方式**: 配置 WeCom 回调并完成 URL 验证、消息签名/解密、命令路由；再用 `clawx config channel wecom`、`clawx config channel telegram` 验证增量配置不会覆盖其他渠道。

**验收场景**:

1. **假如** WeCom 配置完成并通过 URL 验证，**当** 用户发送控制命令时，**那么** 系统应按统一控制语义处理并返回结果。
2. **假如** 运维只修改 Telegram 渠道配置，**当** 执行 `clawx config channel telegram` 后，**那么** 其他渠道配置不得被清空。
3. **假如** WeCom 回调出现解密失败或重放请求，**当** 系统接收该请求时，**那么** 系统应拒绝并产生日志，不影响其他渠道执行。

---

### 用户故事 4 - 补齐未实现外部基线渠道的技术规范与分波次执行（优先级：P4）

作为架构负责人，我希望把外部基线已支持但 ClawX 未实现的渠道全部纳入统一对齐规范，这样后续扩展不会反复重做架构或安全策略。

**为什么是这个优先级**: 这不阻塞 Wave 1 上线，但它直接决定后续 Wave 2~4 的交付速度和一致性。

**独立测试方式**: 检查 `spec/plan/tasks/contracts/data-model` 是否包含 Wave 2~4 渠道的实现模板、契约落点和回归要求。

**验收场景**:

1. **假如** 需要新增 Slack 或 WhatsApp，**当** 开发者阅读规格与任务清单时，**那么** 应能直接找到该渠道的实现模板与测试门禁。
2. **假如** 需要接入 Matrix 或 Teams 等插件型渠道，**当** 开发者执行设计评审时，**那么** 应有明确的插件化落位与主仓边界说明。
3. **假如** 计划声明“对齐外部基线渠道能力”，**当** 进行文档审计时，**那么** 未实现渠道必须出现在对齐矩阵与任务波次中，而不是口头描述。

---

### 边界场景

- Telegram webhook 与健康检查共用 HTTP 服务时，如何避免端口冲突与路径冲突。
- Feishu/WeCom 重复投递同一事件时，如何避免重复执行。
- 渠道消息缺失用户 ID 或会话上下文字段时，如何生成稳定兼容窗口。
- 某渠道连续失败并指数退避时，如何保证其他渠道响应不受影响。
- 渠道配置被部分更新时，如何确保未涉及渠道配置保持原值。
- Wave 2/3/4 渠道接入时，如何避免复制粘贴式实现导致安全与命令语义漂移。
- 插件型渠道（Matrix/Mattermost/Teams 等）如何在不污染主仓的情况下共享统一契约。

## 需求（必填）

### 功能需求

- **FR-001**: 系统必须支持 Telegram `polling` 与 `webhook` 两种模式，且可通过配置切换。
- **FR-002**: 当 Telegram 为 `webhook` 模式时，系统必须支持自动注册 webhook（setWebhook）并处理回调请求。
- **FR-003**: Telegram webhook 回调必须支持鉴权（secret token）与非法方法拒绝。
- **FR-004**: 任一渠道适配器运行失败时，系统必须仅重启该适配器并保持其他渠道继续运行，不得导致主进程退出。
- **FR-005**: 系统必须支持 Feishu 事件回调 challenge 验证。
- **FR-006**: 系统必须支持 Feishu 回调签名校验，验签失败必须拒绝。
- **FR-007**: 系统必须支持 WeCom 回调 URL 验证（echostr）流程。
- **FR-008**: 系统必须支持 WeCom 回调签名校验与消息解密，失败必须拒绝。
- **FR-009**: Feishu 与 WeCom 文本消息必须归一化到统一消息模型，并复用既有 Session/Router 链路。
- **FR-010**: Discord、Telegram、Feishu、WeCom 的控制命令语义必须一致，且至少覆盖 `/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`。
- **FR-011**: 渠道适配器重启必须使用指数退避（初始 2s、最大 60s），并输出结构化日志字段（`channel`、`instance`、`retry_count`、`last_error`）。
- **FR-012**: 系统必须提供 `clawx config channel <name>` 增量配置入口。
- **FR-013**: 增量配置必须仅更新目标渠道字段，不得覆盖其他渠道已有配置。
- **FR-014**: 渠道配置必须支持实例化（至少 `id`、`enabled`、`mode`、`token/secret`、`agent`）。
- **FR-015**: 系统必须保留 Phase 2 多窗口多会话语义，不得因新增渠道回退。
- **FR-016**: 系统必须记录渠道维度审计字段（`channel`、`instance`、`event_id`、`intent.kind`、`duration_ms`）。
- **FR-017**: 对验签失败、重放、解密失败等安全异常，系统必须返回明确拒绝且可追踪日志。
- **FR-018**: 新渠道接入不得要求改造 Backend Adapter 接口。
- **FR-019**: 系统必须维护渠道对齐矩阵，至少覆盖“已实现/未实现/波次/来源基线”字段。
- **FR-020**: Wave 2 渠道（Slack/WhatsApp/Signal/Google Chat/IRC）必须复用统一适配模板，并继承控制命令契约。
- **FR-021**: Wave 3 渠道（Matrix/Mattermost/Teams/Nextcloud Talk/LINE/Nostr/Synology Chat/Twitch/Zalo/Zalo Personal）在满足任一条件时必须采用插件优先策略：`需要私有化/内网部署`、`需要企业身份系统集成`、`预计维护成本 > 2 人日/月`。
- **FR-022**: Wave 4 渠道（BlueBubbles/iMessage legacy/Tlon/WebChat）仅在满足至少一项业务门槛时接入：`至少 1 个付费客户明确需求并确认上线窗口`、`内部月活预测 >= 50`、`存在明确合规/法务要求`；未满足则保持 backlog，不阻塞 Wave 1 发布。
- **FR-023**: 每个新增渠道在进入实现前，必须新增对应契约文档（事件协议/鉴权/幂等/错误语义）或复用已有模板并显式声明差异。
- **FR-024**: 每个新增渠道必须至少具备 1 个适配器单测、1 个控制命令集成测试、1 个跨渠道语义契约测试。
- **FR-025**: 渠道配置模型必须支持新增渠道的最小字段集（`enabled/defaultAgent/instances`）且兼容增量配置命令。
- **FR-026**: 技术规范更新顺序必须一致：`openclaw-channel-parity.md -> spec.md -> plan.md -> tasks.md`，并在 PR 中标注波次范围。

### 关键实体

- **渠道实例（Channel Instance）**: 某渠道的一组运行配置与运行态标识。
- **入站事件（Inbound Event）**: 渠道回调或拉取到的原始消息载荷，包含签名与事件 ID。
- **标准消息（Normalized Message）**: 被统一成 `channel/instance/conversation/user/window/text` 的内部输入。
- **渠道运行状态（Channel Runtime State）**: 某实例的运行状态与重试信息（running/backoff/stopped）。
- **增量配置补丁（Channel Config Patch）**: 仅针对某渠道的配置变更集。

### 假设

- Phase 1~3 主链路稳定可用，Phase 4 不重构核心执行流程。
- 当前优先接入文本消息，不在本阶段处理复杂卡片或富媒体交互。
- 渠道鉴权所需密钥由部署方以配置文件或环境变量提供。
- 本阶段不引入统一渠道管理 UI，仅提供 CLI 与文档流程。
- Wave 2/3/4 可拆分为后续子阶段实现，但本规格先定义统一边界与门禁。

## 成功标准（必填）

### 可度量结果

- **SC-001**: Telegram 在 `polling` 与 `webhook` 模式下均可完成 `/new` 与普通消息链路，成功率 >= 99%（不含 LLM 失败）。
- **SC-002**: 任一单渠道出现连续错误时，主进程存活率为 100%，且其他渠道请求成功率 >= 99%。
- **SC-003**: Feishu 与 WeCom 基础控制命令回归通过率 100%（`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`）。
- **SC-004**: 验签失败/解密失败/重放请求拦截率 100%，并产出可检索日志。
- **SC-005**: `clawx config channel <name>` 增量配置场景下，非目标渠道配置保留率 100%。
- **SC-006**: 渠道路由判定（不含后端执行）p95 < 120ms。
- **SC-007**: 对齐矩阵中的未实现渠道 100% 出现在任务波次（Wave 2~4）中，不得遗漏。
- **SC-008**: 新增任一渠道的技术方案评审输入文档完整率 100%（至少含契约、数据模型映射、测试计划）。

### 统计口径

- 统计窗口：7 天滚动窗口，按候选版本测试批次统计。
- 样本量下限：每项成功标准不少于 200 次有效请求。
- 采集来源：集成测试日志、渠道回调日志、人工验收记录。
- 判定规则：任一 SC 未达标不得进入发布候选。
