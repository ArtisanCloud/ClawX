# 持久化策略使用指南（本地文件优先）

## 适用范围
- 用于第一阶段和第二阶段开发联调。
- 目标是最小依赖启动，不把数据库当作默认前置。

## 默认策略
1. 配置、会话、工作目录都放在 `~/.clawx`。
2. 会话主存储使用本地文件（当前默认实现）：
- `agents/<agent_id>/sessions/sessions.json`
- `agents/<agent_id>/sessions/<session_id>.jsonl`
3. 数据库能力仅作为可选扩展，不作为主线验收条件。

## Phase 1 当前建议
1. 首次引导时，数据库选项选择 `否`。
2. 先验证主链路：
- 渠道收消息
- `/new` 创建会话
- 文本指令触发执行
- 输出回传渠道
3. 建议补做重启恢复验证：
- 重启前执行 `/new` 并记录 `session_id`。
- 重启后执行 `/list` 和 `/resume <session_id>`。

## Phase 2 开发目标
1. 完善文件持久化仓储：
- 服务重启后可 `/list` 查看历史会话。
- `/resume <session_id>` 可恢复历史会话。
2. 将窗口绑定（`window_id -> current_session_id`）从内存提升为文件持久化。

## 文档入口
- 方案：`docs/plans/persistence_strategy.md`
- 第一阶段主线：`docs/guides/phase_1/phase_1_mvp_guide.md`
- 第二阶段计划：`docs/plans/phase_2_multi_session.md`
