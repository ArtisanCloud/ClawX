# 架构设计

## 总体流程
1. 消息进入 Channel Adapter（Discord/Telegram）。
2. 标准化请求后交给 Intent Router。
3. Router 输出 `kind=control|skill|task`。
4. Executor 根据 kind 执行并回传结果。
5. 全链路记录路由与执行日志。

## Skill Registry
- 来源优先级：
1. `~/.clawx/skills`
2. `<workspace>/.clawx/skills`
3. 内置 `internal/skills/builtin`
- 目录要求：
1. Skill 根目录必须存在 `SKILL.md`
2. frontmatter 必须有 `name`、`description`
- 冲突规则：
1. `name` 全局唯一
2. 禁止与系统命令名冲突（如 `new/list/resume/cancel/current`）
- 索引缓存：`~/.clawx/state/skills_index.json`

## Intent Router（规则优先 + LLM 兜底）
- 优先级固定：
1. 控制命令（`/new` `/list` `/resume` `/cancel` `/current`）
2. 显式技能命令（`/skill <name> ...`）
3. 文本规则匹配（精确/别名）
4. LLM 兜底意图识别
5. 普通任务
- 输出结构：
- `kind`
- `skill_name`
- `reason`
- `confidence`（LLM 路径）

## 权限模型
- `skills.enabled` 总开关。
- `skills.disabled_names` 黑名单。
- `skills.allowlist.users` 与 `skills.allowlist.channels`。
- 未授权用户触发 Skill 时拒绝并给出明确提示。
