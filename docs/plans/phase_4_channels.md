# Phase 4 - Channels

## 阶段目标
- 在保持现有 Session/Agent 主链路稳定的前提下，扩展渠道能力。
- 将 Telegram 从单一 polling 升级为 polling/webhook 双模式可选。
- 新增 Feishu（飞书）与 WeCom（企业微信）接入能力，纳入统一 Channel Adapter 模型。

## 阶段定位
- Phase 1 解决了最短执行链路和基础渠道接入。
- Phase 2 解决了多窗口多会话隔离和控制语义。
- Phase 3 解决了多 Agent 与意图路由能力。
- Phase 4 关注“入口扩展 + 接入稳定性 + 渠道一致性”，不改写核心会话与执行模型。

## 核心原则
- `Router -> Session Manager -> Backend Adapter` 主链路不重构。
- 新渠道必须复用统一消息归一化与控制命令语义。
- 安全校验（签名/密钥/重放）必须先于业务处理。
- 渠道故障不拖垮主进程，保持通道级隔离与重试。

## 范围
- Telegram
  - 支持 `polling` / `webhook` 两种模式。
  - webhook 自动注册（setWebhook）与回调验签。
- Feishu
  - 事件接入（包含 challenge 验证与签名校验）。
  - 文本消息归一化、控制命令与会话流复用。
- WeCom
  - 回调验签/解密（URL 验证 + 事件消息）。
  - 文本消息归一化、控制命令与会话流复用。
- 配置与文档
  - 渠道配置项扩展、增量配置命令对齐。
  - 接入指南与验收用例补齐。

## 不做
- 渠道 UI 中台或统一控制台。
- 历史消息回放与离线补偿系统。
- 多租户权限系统重构。
- 与渠道无关的执行后端改造。

## 完成定义
以下条件同时满足，Phase 4 才算完成：
- Telegram webhook 模式可在公网 HTTPS 下稳定收发，并通过验签。
- Feishu 与 WeCom 至少各有一个可用实例完成 `/new`、普通消息、`/list` 基础回路。
- 三个渠道（Discord/Telegram/Feishu/WeCom）控制命令语义一致。
- 渠道故障场景下主服务不退出，错误可观测。
- `go test ./...` 通过，新增渠道契约/集成测试覆盖关键路径。

## 风险与缓解
- 公网与证书依赖：通过 quickstart 明确 ngrok/cloudflared 方案。
- 渠道验签差异：按渠道独立 contract 文档固化，避免混淆。
- 配置复杂度上升：优先实现 `config channel <name>` 增量配置。
- 事件重复投递：在适配层增加幂等键与去重窗口。

## 交付物
- 计划与规格
  - `docs/plans/phase_4_channels.md`
  - `specs/004-channels/*`
- 代码
  - Telegram webhook 模式实现与配置接入
  - Feishu/WeCom 适配层与消息归一化
  - main 启动编排与路由挂载
- 文档
  - `docs/guides/telegram_bot_setup.md` webhook 章节
  - 新增 Feishu/WeCom 接入指南

## 验收标准
- Telegram webhook 场景下消息端到端 p95 < 2s（不含 LLM 执行时间）。
- Feishu/WeCom 在 24 小时稳定运行窗口中无进程级崩溃。
- 渠道控制命令回归测试通过率 100%。
- 验签失败/重放请求被拒绝并记录可追踪日志。
