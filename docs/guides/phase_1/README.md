# Phase 1 Guides

## 先看这个
- `phase_1_mvp_guide.md`
- 这是第一阶段主线文档，目标是先跑通：`main agent + Discord 私聊 + Codex`。

## 推荐执行顺序
1. `tc_l0_01_build_and_tests.md`
2. `tc_l1_01_bootstrap_config.md`
3. `tc_l1_02_health_check.md`
4. `tc_l1_03_main_agent_first_chat.md`
5. `tc_l1_04_main_agent_session_controls.md`
6. `tc_l1_05_main_agent_dev_task.md`
7. `tc_l3_01_chat_control_commands.md`
8. `tc_l3_02_chat_agent_switch_and_dev_prompt.md`（先只用 `main`，不强制切换 agent）

## 进阶（主线跑通后再做）
1. `tc_l2_01_agent_add_default_workspace.md`
2. `tc_l2_02_agent_switch_default.md`
3. `tc_l4_01_agent_workspace_routing_pwd.md`
4. `tc_l5_01_chat_config_plan_apply.md`
5. `tc_l5_02_chat_config_admin_whitelist.md`

## 上层通用文档
- `../storage_local_first.md`: 本地文件优先的持久化策略与阶段目标。
- `../log_tracing.md`: 通过 `session_id` 追踪 runtime/session/codex 全链路日志。
- `../discord_bot_setup.md`: Discord Bot 创建和接入。
- `../telegram_bot_setup.md`: Telegram Bot 创建和接入。
- `../agent_setup.md`: 多 agent 与 workspace 路由配置。
- `../postgres_setup.md`: PostgreSQL 可选接入（非主线）。
