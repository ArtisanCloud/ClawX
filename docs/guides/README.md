# Guides

## 作用
- `docs/guides/` 用于放置可直接执行的操作指南。
- 这类文档面向开发、联调、验证和交付检查。
- 与 `docs/plans/` 不同，这里重点回答“怎么做”和“怎么验”。

## 阶段索引
- `phase_1/`: 第一阶段（Foundation / MVP）指南集合
- `skill_intent_router/`: Skill Registry + Intent Router 指南与测试用例集合

## 通用指南
- `agent_setup.md`: Agent 创建、独立 workspace 与渠道路由配置指南
- `storage_local_first.md`: 本地文件优先的持久化策略与阶段使用方式
- `log_tracing.md`: 按 `session_id` 追踪服务日志、会话日志与 Codex 执行日志
- `telegram_bot_setup.md`: Telegram Bot 创建、配置与 ClawX 接入指南
- `discord_bot_setup.md`: Discord Bot 创建、配置与 ClawX 接入指南
- `postgres_setup.md`: PostgreSQL 可选接入（非默认路径）
