# Guides

## 作用
- `docs/guides/` 用于放置可直接执行的操作指南。
- 这类文档面向开发、联调、验证和交付检查。
- 与 `docs/plans/` 不同，这里重点回答“怎么做”和“怎么验”。

## 阶段索引
- `phase_1/`: 第一阶段（Foundation / MVP）指南集合
- `skill_intent_router/`: Skill Registry + Intent Router 指南与测试用例集合
- `use_cases/`: 渠道窗口任务型用例（能做/怎么做/缺什么）

## 通用指南
- `agent_setup.md`: Agent 创建、独立 workspace 与渠道路由配置指南
- `storage_local_first.md`: 本地文件优先的持久化策略与阶段使用方式
- `log_tracing.md`: 按 `session_id` 追踪服务日志、会话日志与 Codex 执行日志
- `features/012-spec-kit-agent-sop/guide.md`: Spec Kit 驱动的 Agent 开发 SOP（规范生成、实现门禁、回执标准）
- `features/013-lead-worker-runtime-orchestrator/guide.md`: Lead/Worker 运行时编排指南（启动、派工、恢复、回执规范）
- `telegram_bot_setup.md`: Telegram Bot 创建、配置与 ClawX 接入指南
- `discord_bot_setup.md`: Discord Bot 创建、配置与 ClawX 接入指南
- `postgres_setup.md`: PostgreSQL 可选接入（非默认路径）
