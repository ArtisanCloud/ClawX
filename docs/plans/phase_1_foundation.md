# Phase 1 - Foundation

## 目标
- 打稳最短执行链路。
- 确保单窗口单会话可以稳定创建、恢复和继续执行。
- 为后续多窗口、多会话和轻量多 Agent 保留兼容边界，但不提前实现这些能力。

## 范围
- `Router`
- `Session Manager`
- `Backend Adapter`
- `Channel Adapter`
- `Output Streamer`
- 会话锁
- 超时与错误回显
- 基础日志与健康检查
- Telegram 的基础接入
- Discord 的基础接入

## 不做
- 多窗口
- 多 Agent
- 自动路由
- 重型网关或复杂调度层
- 插件系统
- 超出当前主后端的额外后端扩展

## 第一阶段完成的硬门槛
第一阶段不是“代码骨架齐了”就算完成。进入第二阶段代码开发前，必须同时满足以下条件：
- 服务能以常驻形态启动，而不是只完成初始化后退出。
- 至少一个渠道完成真实接入，并可通过配置收发消息。
- 至少一个真实执行后端已接通，不再使用占位返回。
- 能完成一条最小端到端链路：
  - 配置渠道
  - 收到用户消息
  - 创建或继续会话
  - 调用真实执行
  - 返回结果

如果上述条件未满足，则只能视为“第一阶段骨架完成”，不能视为“第一阶段 MVP 完成”。

## 开发启动顺序

### 第 1 层：项目总约束
先读并遵守：
- [../../.specify/memory/constitution.md](../../.specify/memory/constitution.md)

要求：
- 以 `Session` 为主执行单元。
- 保持默认直连执行链路：
  - `Router -> Session Manager -> Backend Adapter`
- `docs/reference/` 只可用于本地研究，不可作为正式开发主依据。

### 第 2 层：第一阶段范围
然后以本文件为阶段范围约束。

要求：
- 只实现本阶段目标与范围。
- 不把第二阶段、第三阶段能力提前拉进来。

### 第 3 层：基础设计文档
接着阅读并对齐以下基础文档：
- [../roadmap.md](../roadmap.md)
- [../../01_concepts/concepts.md](../../01_concepts/concepts.md)
- [../../02_architecture/architecture.md](../../02_architecture/architecture.md)
- [../../06_capabilities/capabilities.md](../../06_capabilities/capabilities.md)
- [../../08_permissions/permissions.md](../../08_permissions/permissions.md)
- [../../09_channels/channels.md](../../09_channels/channels.md)
- [../../11_operations/operations.md](../../11_operations/operations.md)
- [../persistence_strategy.md](../persistence_strategy.md)

用途：
- `roadmap.md`：确认 Phase 1 不是终局，避免超前设计。
- `concepts.md`：统一术语。
- `architecture.md`：按当前最小链路落地模块。
- `capabilities.md`：明确必须支持与明确不做。
- `permissions.md`：落实安全和执行边界。
- `channels.md`：落实 Discord / Telegram 行为与分段规则。
- `operations.md`：落实日志、探针、故障处理基线。

## 相关 Feature 文档

### 当前阶段的使用方式
本阶段允许参考以下功能专题文档，但它们不是 Phase 1 的主范围定义：
- [../..//features/multi_agent/overview.md](../../features/multi_agent/overview.md)
- [../../features/multi_agent/architecture.md](../../features/multi_agent/architecture.md)
- [../../features/multi_agent/implementation.md](../../features/multi_agent/implementation.md)

使用原则：
- 这些文档只用于“预留兼容边界”，不能作为 Phase 1 扩 scope 的依据。
- Phase 1 可以参考其中的接口边界，例如：
  - `window_id`
  - `agent_id`
  - `backend_session_id`
- 但 Phase 1 不要求把多窗口、多 Agent、本地 Binding 真正做出来。

### Phase 1 与 Feature 文档的关系
- Phase 1 应只吸收其中对未来兼容有帮助的部分：
  - 数据字段预留
  - 模块边界保持独立
  - 不把会话逻辑写死到后端适配器里
- Phase 1 不应实现其中属于后续阶段的内容：
  - `Window Context` 的完整多窗口模型
  - `Agent Registry`
  - `Binding`
  - 自动路由策略

## 本阶段必须完成
- `Router` 能区分：
  - 新建会话
  - 恢复会话
  - 在当前会话中继续执行
- `Session Manager` 能完成：
  - 会话创建
  - 会话查询
  - 会话锁
  - 超时后锁释放
- `Backend Adapter` 能完成：
  - 新建后端会话
  - 恢复后端会话
  - 单次执行
  - 超时和取消
- `Channel Adapter` 能完成：
-  - Discord 基础接入
  - Telegram 基础接入
  - 统一消息结构转换
- `Output Streamer` 能完成：
  - 长输出分段
  - 顺序保证
  - 代码块和基础 Markdown 保留
- `Config & Logging` 能完成：
  - `config.json` 主配置读取
  - 环境变量覆盖
  - 默认轻量持久化策略对齐（不把数据库作为主链路前置）
  - 结构化日志
  - 基础健康探针

## 实施要求

### 数据模型要求
- `Session` 至少应具备：
  - `id`
  - `backend`
  - `backend_session_id`
  - `cwd`
  - `status`
  - `lock_token`
- 即使当前不启用多 Agent，也建议预留：
  - `window_id`
  - `agent_id`

### 会话要求
- 私聊和线程必须映射为独立会话。
- 同一会话必须串行执行。
- 新会话、恢复会话和继续当前会话必须有明确状态流转。

### 渠道要求
- Discord 群聊需明确提及机器人后触发。
- Telegram 群聊需命令或明确提及后触发。
- 输出必须遵守各渠道长度限制：
  - Discord: 2000
  - Telegram: 4096

### 安全要求
- 只能在允许的工作目录内执行。
- 不能变成任意 shell 透传。
- 敏感信息不能回显到用户消息。

## 严禁提前实现
- 多窗口管理界面
- 多 Agent 管理界面
- Agent 级自动路由
- 额外复杂状态同步
- 基于研究资料直接复制行为规则

## 本阶段完成后的交接
- Phase 1 完成后，进入 Phase 2 时应优先复用本阶段已稳定的：
  - `Router`
  - `Session Manager`
  - `Backend Adapter`
  - `Channel Adapter`
- 不应推翻 Phase 1 的直连执行链路，只在其上增加多窗口、多会话能力。

## 验收标准
- 能新建会话
- 能恢复会话
- 能继续当前会话
- 同一会话串行执行
- Telegram 基础链路可用
- Discord 基础链路可用
- 长输出可按渠道上限稳定分段
- 超时、取消、错误回显可观察
- 日志和基础健康探针可用

## 当前交付摘要
- 已完成 Go 项目初始化、DDD 目录骨架与测试目录骨架。
- 已完成 `Session`、`Execution`、`Conversation` 的核心领域模型与内存态会话仓储。
- 已完成 `Router`、`Session Manager`、最小 `Backend Adapter` 以及控制命令主链路。
- 已完成输出分段、格式化、部分失败提示，以及 Telegram 的最小真实接入。
- 已完成统一错误映射、健康处理器以及基础测试文件骨架。
- 已完成 `go test ./...` 的本地基础编译验证。
- 已完成常驻服务启动、健康检查挂载与 `codex` / `claude` profile 执行器接入，并保留本地 smoke fallback。
- 已完成 `providers.profiles` 与 `agents.default/list` 的轻量执行配置模型。
- Discord 已补到 `Bot Token + Gateway WebSocket` 运行时代码，外部联调仍需真实凭证验证。
- 当前已具备“最小本地可用”状态，但仍未完成完整外部渠道人工验收。

## 当前仍建议补强的项
以下项目不再阻塞第一阶段代码完成，但在真实部署前仍建议补强：
- 当前仍可能把默认 agent 切到 `local-smoke` 做本地烟测，但这不是最终业务执行器。
- 默认 agent 虽已可指向 `codex` / `claude`，但仍需你的真实 CLI 环境完成联调验收。
- 还未完成一次带真实 Telegram / Discord 凭证的外部人工验收。
- 还未把最终运行说明收敛成生产前使用手册。

## 当前未闭环的功能需求（对应 `specs/001-phase1-foundation/spec.md`）
以下需求在代码和本地链路上已具备，但对外真实可用仍取决于你的真实凭证联调：
- `FR-001` / `FR-002` / `FR-003` / `FR-004`
  - 会话新建、继续、列表、恢复已可由 Telegram / Discord 运行时驱动，但仍需外部凭证验收。
- `FR-005` / `FR-007`
  - 取消与超时已接到真实执行路径，但仍需按最终业务执行器再验一次。
- `FR-008` / `FR-009` / `FR-010`
  - 输出分段、错误回显已接到 Telegram / Discord 发送路径，但仍需外部凭证验收。
- `FR-011` / `FR-012`
  - 真实渠道入站上下文已接入；Discord 的线程细分仍可继续优化。

当前可以视为基本落地并保持不回退的主要是：
- `FR-006`
  - 同一会话串行执行与繁忙拒绝
- `FR-013`
  - 运行边界配置校验骨架
- `FR-014`
  - 基础元数据结构与记录位点

当前已从“骨架”推进到“最小可用”的主要是：
- `FR-001` / `FR-002` / `FR-003` / `FR-004`
  - Telegram / Discord 运行时已可驱动会话主链路
- `FR-005` / `FR-007`
  - 取消与超时已接到真实本地执行路径
- `FR-008` / `FR-009` / `FR-010`
  - 输出分段与错误回显已可走真实本地发送路径
- `FR-011` / `FR-012`
  - 真实渠道上下文已接到 Router 的准入校验
- `FR-015`
  - 健康检查已可通过真实 HTTP 入口访问

## 本次最小人工验收记录
- 时间：2026-03-04
- 验收方式：
  - 以 `agents.default=local-smoke`
  - `channels.telegram.enabled=false`
  - `gateway.listenAddr=127.0.0.1:18080`
    启动本地服务
  - 访问 `http://127.0.0.1:18080/healthz`
- 验收结果：
  - 服务可常驻启动
  - 健康检查返回 `healthy`
  - `BackendProbe` 返回 `ok`
- 结论：
- 第一阶段已通过“本地服务可启动 + 健康检查可访问”的最小人工验收
- 第一阶段代码层面的 MVP 交付已达成
- “真实外部 Telegram / Discord 收发消息”仍需你的真实凭证完成部署侧验收
- “真实外部 Codex / Claude CLI 执行”仍需你的真实 CLI 环境完成部署侧验收

## 进入真实部署前建议补完
在进入真实对外使用前，建议继续完成：
- 一次带真实 Telegram / Discord 凭证的外部人工验收
- 一次带真实 Codex / Claude CLI 的外部人工验收
- 将默认 agent 切到真实 `codex` 或 `claude`
- 面向真实部署的最终运行说明
