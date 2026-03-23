# Phase 7 - Agent Skill 治理与编排

## 阶段目标
- 建立统一的 Skill Control Plane，支持内建技能与第三方技能（如 ClawHub）治理。
- 把“自然语言控制 Agent/Skill”切到 LLM-first 路径，命令仅作快捷入口与回退。
- 支持技能在多 Agent 间共享、隔离、继承，并具备可审计与可回滚能力。

## 背景问题（当前痛点）
- 配置能力已有基础闭环，但 skill 仍缺少统一注册、版本、风险分级与共享策略。
- 自然语言与命令链路仍有割裂感，用户需要记忆过多 `/config ...` 细节。
- 第三方技能引入后，缺少统一安装策略、供应链安全校验和执行权限边界。

## 范围
### P7（必须）
- 新增 `Skill Registry`：
  - 管理 skill 元数据（id、version、source、capabilities、risk、schema）。
  - 支持 built-in 与 external 双来源统一视图。
- 新增 `Skill Policy`：
  - 安装源白名单、版本锁定、升级策略、禁用策略。
  - 风险分级（low/medium/high）与 confirm-first 规则绑定。
- 新增 `Skill Binding`：
  - `global`（全局共享）、`project`（项目级）、`agent-local`（私有）三层绑定。
  - 明确优先级与覆盖规则：`agent-local > project > global`。
- 新增 `LLM Skill Router`：
  - 自然语言默认走 LLM 意图识别 -> skill 选择 -> 结构化 action。
  - 结构化输出必须过 schema 校验后执行。

### P7.5（建议同阶段规划）
- 新增 `Skill Scheduler`：
  - 支持技能任务定时执行、失败重试、幂等键。
- 新增 `Skill Context Compression`：
  - 将技能调用轨迹压缩为结构化摘要，降低上下文长度并保留决策信息。

## 不做
- 不做全自动无审批安装第三方高风险技能。
- 不做分布式多节点强一致调度（本阶段仍单实例优先）。
- 不做泛化“任意工具自动执行”黑盒路径，必须走 schema 与权限门禁。

## 统一架构策略
1. 双入口：`Command` 与 `Natural Language`。  
2. 单路由：自然语言统一进 `LLM Skill Router`，命令直接映射到同一 action schema。  
3. 单执行：统一进 `Skill Executor`（权限、校验、审计、回滚）。  
4. 单审计：统一记录 skill 选择、参数、风险级别、审批与执行结果。

## 执行契约修正（2026-03-20）

为避免“从模型自然语言回复中正则提取 `/command` 并误执行”的风险，本阶段补充以下硬约束：

1. 自然语言请求仅允许触发 `LLM Planner`，不得直接执行文本命令。
2. 自动执行仅接受结构化 `ControlPlan`（JSON schema 校验通过）。
3. 普通文本中的 `/agent ...`、`/config ...`、`/skill ...` 一律视为说明性文本，不得自动执行。
4. 控制面动作必须由 `ClawX Executor` 执行并落审计，模型输出不作为事实来源。
5. 高风险动作必须进入确认状态机（pending -> confirmed/rejected -> execute/noop）。

### ControlPlan 最小协议

```json
{
  "type": "control_plan",
  "intent": "agent.use",
  "target": {
    "agent_id": "bid-all"
  },
  "mode": "execute",
  "risk": "low",
  "reason": "用户请求切换智能体"
}
```

### 执行门禁

- Schema 不通过：拒绝执行。
- `intent` 不在白名单：拒绝执行。
- 参数不合法或越权：拒绝执行。
- 幂等命中：返回 `status=noop`，不重复执行。
- 执行后必须 `post-verify` 读取真实状态并回写结构化结果。

## 关键机制设计
### 1) Skill 元数据规范（统一 schema）
- 必填：
  - `skill_id`
  - `version`
  - `source`（builtin/clawhub/local/git）
  - `intent_examples`
  - `input_schema`
  - `risk_level`
  - `required_permissions`
  - `supports_agents_scope`（global/project/agent）
- 扩展：
  - `tags`
  - `owner`
  - `checksum/signature`
  - `deprecation_policy`

### 2) 技能共享与隔离
- 共享层次：
  - `global`: 所有 agent 可见。
  - `project`: 仅项目内 agent 可见。
  - `agent-local`: 单 agent 私有。
- 冲突策略：
  - 同名 skill 按 `agent-local > project > global` 解析。
  - 同层冲突按版本策略（pin > latest > compatible）处理。

### 3) LLM-first 路由策略
- 自然语言默认路径：
  - `message + context_summary + skill_catalog` -> LLM -> `SkillAction`。
- `SkillAction` 必须结构化输出：
  - `intent`
  - `skill_id`
  - `arguments`
  - `confidence`
  - `risk_level`
  - `requires_confirmation`
- 低置信度处理：
  - 不执行，只澄清或给候选动作建议。

### 4) 安全与治理
- 高风险技能执行前必须确认：
  - 目录删除、覆盖写入、外部发布、跨工作区迁移。
- 第三方技能治理：
  - 白名单源 + 版本锁定 + 校验摘要。
- 审计要求：
  - 每次执行记录 `who/when/why/what/result`。

### 5) 上下文压缩（skill 维度）
- 压缩目标：
  - 保留最近 N 次关键调用、参数摘要、结果摘要、失败原因。
- 压缩触发：
  - 调用次数阈值、token 预算阈值、会话切换。
- 压缩产物：
  - 结构化 `skill_context_digest`，供下一轮 LLM 路由使用。

## 与 Phase 6 / specs/009 的边界
- `Phase 6 / 009`：配置控制面（plan/patch/apply/cancel）与配置意图统一。
- `Phase 7`：技能控制面（registry/policy/binding/router/executor）与技能意图统一。
- 对接方式：
  - `009` 提供 Agent 配置状态。
  - `Phase 7` 在此基础上决定“该 Agent 当前可用哪些技能，以及如何执行”。

## 验收标准（阶段门禁）
- 用户可仅用自然语言完成：
  - 安装技能、绑定技能、调用技能、查看结果（无需背命令）。
- 所有技能调用都有结构化审计记录。
- 多 Agent 共享与私有覆盖规则可预测且有测试覆盖。
- 高风险技能均触发确认门禁，无法绕过。
- LLM 路由低置信度时不发生误执行。

## 里程碑
### M1：Skill Registry + Binding
- 完成 skill 元数据 schema 与注册中心。
- 完成 global/project/agent 三层绑定与解析优先级。

### M2：LLM Skill Router + Executor
- 打通自然语言 -> skill action 结构化链路。
- 完成权限门禁、风险确认、审计落地。

### M3：Policy + Scheduler + Compression
- 完成第三方技能安装策略与版本治理。
- 完成技能调度与上下文压缩闭环。

## 风险与缓解
- 风险：LLM 误选技能导致错误执行。
  - 缓解：schema 校验 + 风险门禁 + 低置信度不执行。
- 风险：第三方技能供应链风险。
  - 缓解：白名单源、版本锁定、签名/摘要校验。
- 风险：多层绑定规则复杂导致行为不可预测。
  - 缓解：固定优先级、冲突可解释、回归测试覆盖。

## 交付物
- 阶段计划：`docs/plans/phase_7_agent_skill_governance_orchestration.md`（本文件）
- 下一步规格与任务（建议）：
  - `specs/010-skill-governance-orchestration/spec.md`
  - `specs/010-skill-governance-orchestration/plan.md`
  - `specs/010-skill-governance-orchestration/tasks.md`
