# Phase 6 - Agent/Context/Intent 一体化

## 阶段目标
- 把“斜杠指令”和“自然语言”统一到同一控制平面，避免两条链路割裂。
- 把 Agent 路由、上下文装载、意图判定纳入同一可观测闭环。
- 对齐 OpenClaw 的体验方向：自然语言可控 + 命令治理可审计。

## 背景问题（当前痛点）
- `/config plan ...` 走规则链路，普通自然语言走执行链路；同一会话内无法直接“编辑待确认计划”。
- 用户感知上像“上下文断裂”：上一条刚生成计划，下一条自然语言却被当作普通任务执行。
- 现有上下文机制偏执行侧（session/memory/backend thread），缺少“配置意图上下文”这一层。

## 范围
### P6（必须）
- 新增 `Config Intent Bridge`：
  - 普通自然语言先识别是否为“配置意图”。
  - 命中后生成结构化 plan（与 `/config plan` 共用数据结构）。
- 新增 `Plan Patch`：
  - 支持“把 workspace 改成 ...”“timeout 改成 ...”这类自然语言补丁。
  - patch 只作用于当前会话 pending plan。
- 新增统一确认链：
  - 所有配置变更统一进入 `show -> apply/cancel`。
  - `apply` 仍由管理员权限控制。

### P6.5（建议同阶段规划）
- 新增“意图治理层”：
  - 区分 `config_intent / control_intent / task_intent`。
  - 高风险动作（写配置、删资源）必须 confirm-first。
- 新增“上下文压缩策略（控制面）”：
  - 对 pending plan 的变更历史做结构化摘要，不依赖模型自由摘要。
  - 保留可回放的 diff 轨迹与审计字段。

## 不做
- 不做全自动无确认写盘（禁止 silent apply）。
- 不做重型编排中台。
- 不做跨实例分布式一致性（本阶段限定单实例进程内闭环 + 文件持久化）。

## 统一架构策略
1. 双入口：`Slash Parser` 与 `NL Intent Parser` 并行接入。  
2. 单状态：统一写入 `Pending Plan Store`（按 conversation scope 隔离）。  
3. 单出口：统一走 `show/apply/cancel` 与权限门禁。  
4. 单审计：统一输出 `plan_source=slash|nl`、`plan_patch_count`、`applied_by`、`diff`。

## 关键能力设计
### 1) Config Intent Bridge
- 输入：普通消息文本。
- 输出：
  - 命中：`ConfigPlanDelta`（create/update/default/patch）。
  - 未命中：回退原有 task/skill 路由。
- 实现原则：规则优先 + 可选 LLM fallback（低置信度不落盘，只建议）。

### 2) Pending Plan Store
- 作用域：`channel + instance + conversation`。
- 内容：
  - 当前 plan 快照（结构化字段）。
  - patch 历史（append-only，含时间戳和来源）。
  - 最近一次 explain 文本（给 `/config show`）。

### 3) Plan Patch DSL（内部）
- 用户可自然语言表达，内部统一为 patch 操作：
  - `set profile`
  - `set workspace`
  - `set timeout`
  - `set default`
  - `rename id`（可选）
- 失败语义：
  - `no_pending_plan`
  - `invalid_patch`
  - `unsafe_patch_requires_confirm`

### 4) Confirm-First 治理
- 仅 `/config apply` 允许写配置文件。
- 非管理员仍可生成 plan，但不能 apply（可选策略）。
- 审计落地到日志与状态文件，支持事后追踪。

## 验收标准（阶段门禁）
- 在同一会话中，“创建计划 -> 自然语言修改 -> show -> apply”可闭环。
- 自然语言 patch 成功率达到可用阈值（由契约样例集定义）。
- 未命中 config intent 的普通任务不受回归影响。
- 所有配置写盘均可追踪来源与 diff。
- 出现低置信度语义时，系统只给建议，不执行写盘。

## 里程碑
### M1：控制面最小闭环
- 建立 `Config Intent Bridge`（规则优先）。
- 接入统一 `Pending Plan Store` 与 patch 机制。
- 打通 `/config show|apply|cancel` 复用。

### M2：治理与可观测
- 审计字段与日志串联。
- 错误码与用户反馈统一。
- 增加回归测试（指令链 + 语义链 + 混合链）。

### M3：LLM 增强（可选）
- 在规则未命中时启用 LLM fallback。
- 低置信度触发澄清/建议，不直接改 plan。

## 风险与缓解
- 风险：自然语言误判导致错误 patch。
  - 缓解：patch 只进 pending，不直接 apply；低置信度拒绝执行。
- 风险：双入口并发修改同一 plan 造成覆盖。
  - 缓解：版本号 + compare-and-swap；冲突时返回重试提示。
- 风险：用户仍混用普通聊天与配置意图导致歧义。
  - 缓解：命中 config intent 时显式回复“已进入配置计划模式”并展示摘要。

## 交付物
- 阶段计划：`docs/plans/phase_6_agent_context_intent_unification.md`（本文件）
- 规格与任务（下一步）：
  - `specs/009-config-intent-plan/spec.md`
  - `specs/009-config-intent-plan/plan.md`
  - `specs/009-config-intent-plan/tasks.md`

## 对齐状态（2026-03-19）
- 已落地：P6 主体（Config Intent Bridge、Plan Patch、统一确认链、权限与审计基础）。
- 待补齐：P6.5 的“控制面上下文压缩策略”仍需专门实现（结构化摘要、重建机制、show 摘要窗口、一致性门禁）。
- 对齐动作：已在 `specs/009-config-intent-plan/tasks.md` 新增 Phase 9（T067-T078）作为强制补齐项。
