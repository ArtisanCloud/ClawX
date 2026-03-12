# Skill Registry + Intent Router（Phase Next）

## 目标
- 在 ClawX 中引入可扩展 Skill 注册中心与意图路由。
- 100% 兼容 Claude Code Skill 结构（`SKILL.md` + frontmatter）。
- 与现有多会话、Discord/Telegram、`main` agent 机制无缝协作。

## 范围
- Skill 扫描、校验、索引、启停。
- 路由链路：控制命令 -> 显式 Skill -> 规则匹配 -> LLM 兜底 -> 普通任务。
- Skill 权限：按 channel/user 白名单 + skill 开关。
- 运行时日志与诊断字段标准化。

## 非目标
- Web 管理界面。
- 在线 Skill 市场。
- 多 Skill 编排（本阶段仅单次命中一个 Skill）。

## 里程碑
1. Registry 最小可用（`list/reload/enable/disable`）。
2. Router 接入现有消息入口并稳定分流。
3. Claude Skill 兼容验收通过（本地导入）。
4. 灰度开启 LLM 兜底并调优阈值。
