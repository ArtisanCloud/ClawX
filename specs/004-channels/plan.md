# 实现计划：第四阶段渠道扩展

**分支**: `004-channels` | **日期**: 2026-03-10 | **规格**: `/home/ubuntu/workspace/SynapseX/specs/004-channels/spec.md`  
**输入**: 功能规格来自 `/home/ubuntu/workspace/SynapseX/specs/004-channels/spec.md`

## 摘要

本阶段在不改动核心执行链路（`Router -> Session Manager -> Backend Adapter`）的前提下，扩展渠道接入能力：补齐 Telegram webhook 生产可用能力，新增 Feishu 与 WeCom 适配层，并统一控制命令语义与增量配置体验。  
同时建立外部基线渠道对齐路线（当前参考 OpenClaw，Wave 1~4），把“未实现渠道”纳入统一技术规范。  
核心目标是“新增入口 + 稳定接入 + 安全校验（含重放防护） + 故障隔离 + 结构化可观测 + 对齐可执行”。

## 技术上下文

**Language/Version**: Go 1.23  
**Primary Dependencies**: Go 标准库、现有 SynapseX DDD 模块、DiscordGo、Telegram Bot SDK（Go）、现有配置与持久化组件  
**Storage**: 本地配置文件（`~/.synapsex/config.json`）+ 现有会话存储  
**Testing**: Go 原生 `testing`（unit/integration/contract）+ `go test ./...`  
**Target Platform**: Linux server（公网 HTTPS 可用场景）  
**Project Type**: 聊天驱动后端服务 / CLI 控制网关  
**Performance Goals**: 渠道路由判定 p95 < 120ms（不含后端执行时间）  
**Constraints**: 单渠道故障不影响主进程；控制命令语义一致；Phase 2 窗口模型不回退；Webhook 必须验签  
**Scale/Scope**: 
- Wave 1：单实例支持 Discord/Telegram/Feishu/WeCom，5-20 并发会话
- Wave 2~4：覆盖外部基线渠道对齐清单（以文档与技术规范先行，代码按波次推进）

## 宪章检查

*门禁：必须在 Phase 0 研究前通过；Phase 1 设计完成后再次检查。*

- Session-First：通过。渠道只提供输入入口，不改变 Session 作为核心执行单位。
- Agent 作为模板：通过。不引入重型 Agent 编排，仅在渠道级透传 agent 选择。
- 直连执行链路：通过。新增渠道仍复用统一 Router 与 Session Manager。
- 稳定边界：通过。渠道适配层负责验签与归一化，业务决策在应用层。
- Docs 驱动范围：通过。范围来源于 `docs/plans/phase_4_channels.md` 与本规格。
- Go + DDD：通过。新增代码落在 `interfaces/infrastructure/application` 分层边界内。

当前无需要豁免的宪章违例。

### Phase 1 设计后复核

- 复核结论：通过。
- `research.md` 已明确 webhook/polling 取舍、验签策略、故障隔离策略与增量配置原则。
- `data-model.md` 与 `contracts/` 未引入跨层职责混乱或不可测需求。

## 项目结构

### 文档（本功能）

```text
/home/ubuntu/workspace/SynapseX/specs/004-channels/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── openclaw-channel-parity.md
├── channels/
│   └── _template.md
├── contracts/
│   ├── channel-control-contract.md
│   ├── telegram-webhook-contract.md
│   ├── feishu-event-contract.md
│   └── wecom-event-contract.md
├── checklists/
│   └── requirements.md
└── tasks.md
```

### 源码（仓库根目录）

```text
/home/ubuntu/workspace/SynapseX/
├── cmd/
│   └── synapsex/
│       ├── main.go
│       ├── config.go
│       └── config_channel.go
├── internal/
│   ├── application/
│   │   └── service/
│   ├── infrastructure/
│   │   └── config/
│   └── interfaces/
│       └── chat/
│           ├── discord/
│           ├── telegram/
│           ├── feishu/      # 新增
│           └── wecom/       # 新增
└── tests/
    ├── integration/
    └── contract/
```

**结构决策**: 沿用单服务 Go + DDD 结构；新增渠道各自一个适配目录，公共逻辑留在统一 `chat` 归一化与 `router`。配置增量能力放在 CLI 配置层，不下沉到业务层。

## 渠道对齐波次计划

对齐矩阵来源：`/home/ubuntu/workspace/SynapseX/specs/004-channels/openclaw-channel-parity.md`

- Wave 1（基线）：Telegram webhook + Feishu + WeCom
- Wave 2（主流通用）：Slack、WhatsApp、Signal、Google Chat、IRC
- Wave 3（插件/企业扩展）：Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal
- Wave 4（可选）：BlueBubbles、iMessage legacy、Tlon、WebChat

波次执行规则：
- 先完成统一适配器模板（输入归一化、控制命令路由、安全校验、错误重试、日志字段）。
- 每新增一个渠道，先补契约与测试，再落适配器代码。
- Wave 2~4 默认不阻塞 Wave 1 发布，但必须保持文档与任务清单同步更新。

## 复杂度跟踪

当前无已批准的复杂度豁免。

## 阶段交付摘要（目标态）

- Telegram：双模式（polling/webhook）稳定可用，支持自动注册与验签。
- Feishu：challenge + 事件验签 + 文本消息归一化 + 控制命令链路可用。
- WeCom：URL 验证 + 签名/解密 + 文本消息归一化 + 控制命令链路可用。
- 安全与可测性：重放请求拒绝、重试结构化日志字段断言、渠道路由 p95 指标可测。
- 配置：`synapsex config channel <name>` 支持单渠道增量配置。
- 测试：新增渠道契约/集成测试，覆盖安全、故障隔离与命令一致性。
- 对齐：未实现渠道全部进入分波次任务清单并具备统一实现模板。

## 风险清单（当前）

- 公网回调依赖 HTTPS 与可达性，内网部署可能需要隧道或反向代理。
- Feishu/WeCom 签名与加解密机制差异较大，容易出现实现混淆。
- 渠道增多后日志体量上升，需要字段规范与限流策略。
- 渠道回调重复投递可能引起重复执行，需幂等键策略配合。
- 多渠道并行扩展导致主仓复杂度上升，需插件化边界与代码所有权约束。
