# Skill Registry + Intent Router（Phase Next）

## 目标
- 在 ClawX 中引入可扩展 Skill 注册中心与意图路由。
- 100% 兼容 Claude Code Skill 结构（`SKILL.md` + frontmatter）。
- 与现有多会话、Discord/Telegram、`main` agent 机制无缝协作。

## 范围
- Skill 扫描、校验、索引、启停。
- 路由链路：控制命令（`/...`）-> 非命令自然语言（LLM 规划）-> 结构化 action/control plan -> 执行器。
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
4. 灰度开启 LLM 规划并调优阈值（保持执行端结构化门禁）。

## 执行安全边界

- 自动执行仅消费结构化输出（SkillAction/ControlPlan）。
- 普通文本中的 `/command` 不得作为自动执行输入。
- 控制面动作由 ClawX 执行器统一落审计与 post-verify。

## LLM 调用分层（新增规范）
- 自然语言路径采用分层调用，避免单次大上下文请求：
1. `Intent Planner`：识别意图、目标 Agent、风险等级、执行线路。
2. `Route-specific Planner`：仅加载目标线路相关技能清单与上下文。
3. `Structured Executor`：输出 `control_plan` / `requirement_sync` 等结构化计划。
- 执行器负责校验、执行、审计；模型仅负责“提案”。

## OpenAI Prompt Caching（新增规范）
- 对自然语言 Planner 请求，必须接入 Prompt Caching。
- 必须显式设计并传递 `prompt_cache_key`，保证“固定系统前缀 + 阶段标识”稳定。
- `prompt_cache_retention` 默认 `in_memory`，可按场景评估 `24h`。
- 必须记录 `usage.prompt_tokens_details.cached_tokens` 作为命中指标，并纳入运行日志/看板。
- 不得把 Prompt Caching 作为状态一致性机制；业务状态仍以会话存储与审计日志为准。

## 参考链接
- https://platform.openai.com/docs/guides/prompt-caching
- https://openai.com/index/api-prompt-caching/
