# 数据模型：Config Intent Plan

## 1. ConfigIntentDecision（配置意图决策）

### 作用
描述单条输入消息是否进入配置控制面，以及对应置信度与建议动作。

### 字段
- `conversation_scope`：会话作用域键
- `decision_kind`：`config` | `task` | `clarify`
- `confidence`：0.00 ~ 1.00
- `reason`：判定原因（规则命中、低置信度、冲突澄清等）
- `suggested_action`：建议动作（可空）

### 约束
- `decision_kind=clarify` 时必须带澄清提示语义。
- `confidence` 必须可审计、可追踪。

## 2. PendingConfigPlan（待确认配置计划）

### 作用
承载会话内待确认的配置变更快照，是 `/config show/apply/cancel` 的统一对象。

### 字段
- `plan_id`
- `conversation_scope`
- `kind`：`upsert-agent` | `set-default-agent`
- `source`：`slash` | `nl`
- `created_by`
- `created_at`
- `summary`
- `agent_options`（可空）
- `default_agent_id`（可空）
- `version`

### 约束
- 同一 `conversation_scope` 同时最多一个活动计划。
- `version` 必须单调递增，用于并发 patch 保护。

## 3. ConfigPlanPatch（计划补丁）

### 作用
记录对 pending plan 的单次字段变更，支撑编辑历史与回放。

### 字段
- `patch_id`
- `plan_id`
- `applied_by`
- `source`：`slash` | `nl`
- `operation`：`set_profile` | `set_workspace` | `set_timeout` | `set_default` | `set_agent_id`
- `before_value`
- `after_value`
- `applied_at`

### 约束
- 每条 patch 必须可映射到一个明确字段变更。
- patch 应按时间顺序可回放，且与 `plan.version` 一致增长。

## 4. ConfigApplyAttempt（配置应用尝试）

### 作用
记录一次 apply 行为及其权限校验结果。

### 字段
- `attempt_id`
- `plan_id`
- `requested_by`
- `is_admin`
- `result`：`applied` | `rejected_permission` | `rejected_invalid_plan` | `failed_write`
- `message`
- `attempted_at`

### 约束
- `is_admin=false` 时 `result` 不得为 `applied`。
- 成功应用后应终结对应 pending plan 生命周期。

## 5. ConfigAuditEvent（配置审计事件）

### 作用
对配置计划生命周期关键动作做统一审计。

### 字段
- `event_id`
- `conversation_scope`
- `plan_id`
- `event_type`：`plan_created` | `plan_patched` | `plan_shown` | `plan_canceled` | `plan_applied` | `plan_apply_rejected`
- `actor`
- `source`
- `diff_summary`
- `timestamp`

### 约束
- 计划创建、补丁、取消、应用、拒绝必须有对应审计事件。
- `diff_summary` 在 `patched/applied` 场景必须非空。

## 状态流转

### PendingConfigPlan
- `created -> patched -> patched ... -> applied`
- `created -> patched -> canceled`
- `created -> canceled`

### ConfigApplyAttempt
- `requested -> applied`
- `requested -> rejected_permission`
- `requested -> rejected_invalid_plan`
- `requested -> failed_write`

## 实体关系
- `PendingConfigPlan` 1:N `ConfigPlanPatch`
- `PendingConfigPlan` 1:N `ConfigApplyAttempt`
- `PendingConfigPlan` 1:N `ConfigAuditEvent`
- `ConfigIntentDecision` 驱动 `PendingConfigPlan` 的创建或补丁行为
