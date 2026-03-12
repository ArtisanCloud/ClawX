# Phase 4 - Channels

## 阶段目标
- 在保持现有 Session/Agent 主链路稳定的前提下，扩展渠道能力。
- 完成 Phase 4 基线交付：Telegram 双模式 + Feishu + WeCom。
- 建立与 OpenClaw 渠道能力对齐的波次路线，并同步到技术规范与任务清单。

## 阶段定位
- Phase 1 解决最短执行链路和 Discord/Telegram 基础接入。
- Phase 2 解决多窗口多会话隔离和控制语义。
- Phase 3 解决多 Agent 与意图路由能力。
- Phase 4 聚焦“入口扩展 + 接入稳定性 + 渠道一致性 + 对齐路线”。

## 对齐基线
- OpenClaw 对齐矩阵见：`/home/ubuntu/workspace/SynapseX/specs/004-channels/openclaw-channel-parity.md`
- 当前 SynapseX 已实现：Discord、Telegram（polling/webhook）。
- 当前 SynapseX 未实现但需纳入对齐规划：Slack、WhatsApp、Signal、Google Chat、IRC、Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal、BlueBubbles、iMessage legacy、Tlon、WebChat。

## 核心原则
- `Router -> Session Manager -> Backend Adapter` 主链路不重构。
- 新渠道必须复用统一消息归一化与控制命令语义。
- 安全校验（签名/密钥/重放）必须先于业务处理。
- 单渠道故障不拖垮主进程，保持通道级隔离与重试。
- 先实现可复用适配器模板，再扩展新渠道，避免每个渠道重复造轮子。

## 范围分层
### Wave 1（Phase 4 基线，阻塞发布）
- Telegram
  - 支持 `polling` / `webhook` 两种模式。
  - webhook 自动注册（setWebhook）与回调验签。
- Feishu
  - challenge 验证与签名校验。
  - 文本消息归一化、控制命令与会话流复用。
- WeCom
  - URL 验证、签名校验与解密。
  - 文本消息归一化、控制命令与会话流复用。
- 配置与文档
  - `synapsex config channel <name>` 增量配置。
  - 接入指南与验收用例补齐。

### Wave 2（主流通用渠道，对齐增强）
- Slack、WhatsApp、Signal、Google Chat、IRC。
- 目标：在不改核心链路的前提下复用 Wave 1 适配器模板快速扩展。

### Wave 3（插件类/企业扩展渠道）
- Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal。
- 目标：以插件化或独立适配包降低主仓复杂度。

### Wave 4（可选渠道）
- BlueBubbles、iMessage legacy、Tlon、WebChat。
- 目标：按业务需求驱动，不阻塞主线发布。

## 不做
- 渠道 UI 中台或统一控制台。
- 历史消息回放与离线补偿系统。
- 多租户权限系统重构。
- 与渠道无关的执行后端改造。

## 完成定义
### Phase 4 基线完成（当前发布门槛）
以下条件同时满足，Phase 4 基线才算完成：
- Telegram webhook 模式可在公网 HTTPS 下稳定收发，并通过验签。
- Feishu 与 WeCom 至少各有一个可用实例完成 `/new`、普通消息、`/list` 基础回路。
- 四个渠道（Discord/Telegram/Feishu/WeCom）控制命令语义一致。
- 渠道故障场景下主服务不退出，错误可观测。
- `go test ./...` 通过，新增渠道契约/集成测试覆盖关键路径。

### OpenClaw 对齐完成（长期目标）
- Wave 2 渠道完成后，可宣称“主流渠道基础对齐”。
- Wave 3/4 按业务优先级迭代，不作为 Phase 4 基线阻塞项。

## 风险与缓解
- 公网与证书依赖：通过 quickstart 明确 ngrok/cloudflared/Nginx 方案。
- 渠道验签差异：按渠道独立 contract 文档固化，避免混淆。
- 渠道数量增加导致复杂度上升：统一适配器模板 + 波次交付。
- 配置复杂度上升：优先实现 `config channel <name>` 增量配置。
- 事件重复投递：在适配层增加幂等键与去重窗口。

## 交付物
- 计划与规格
  - `/home/ubuntu/workspace/SynapseX/docs/plans/phase_4_channels.md`
  - `/home/ubuntu/workspace/SynapseX/specs/004-channels/*`
- 对齐文档
  - `/home/ubuntu/workspace/SynapseX/specs/004-channels/openclaw-channel-parity.md`
- 代码（Wave 1）
  - Telegram webhook 模式实现与配置接入
  - Feishu/WeCom 适配层与消息归一化
  - main 启动编排与路由挂载
- 文档（Wave 1）
  - Telegram 接入与 webhook 指南
  - Feishu/WeCom 接入指南

## 验收标准
- Telegram webhook 场景下消息端到端 p95 < 2s（不含 LLM 执行时间）。
- Feishu/WeCom 在 24 小时稳定运行窗口中无进程级崩溃。
- Wave 1 渠道控制命令回归测试通过率 100%。
- 验签失败/重放请求被拒绝并记录可追踪日志。
