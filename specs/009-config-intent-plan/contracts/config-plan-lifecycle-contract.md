# 契约：配置计划生命周期

## 目的
定义 pending plan 的创建、展示、补丁、取消、应用以及权限治理边界。

## 1. 生命周期状态
- `created`
- `patched`（可重复）
- `canceled`
- `applied`

## 2. 核心命令语义
- `/config plan ...`：创建或覆盖当前会话 pending plan。
- `/config show`：展示当前会话 pending plan 摘要。
- `/config cancel`：取消当前会话 pending plan。
- `/config apply`：校验权限并应用计划，成功后结束生命周期。

## 3. 会话隔离契约
- pending plan 必须按会话作用域隔离。
- 当前实现以 `conversation` 作为计划存储键；上游应保证该键可唯一表示 `channel + instance + conversation` 作用域。
- 不同作用域之间不得读写同一 pending plan。

## 4. 补丁契约
- 自然语言或命令式补丁均可编辑当前会话 pending plan。
- 每次补丁必须至少记录：变更字段、操作者、时间、摘要。
- 补丁历史在 `apply/cancel` 前完整保留（版本递增）。

## 5. 权限契约
- 非管理员可：
  - 创建计划
  - 编辑计划
  - 展示计划
  - 取消计划
- 非管理员不可：
  - 应用计划（`/config apply`）

## 6. 应用契约
- 仅 `/config apply` 可以触发配置落盘。
- 未 apply 前，配置文件不得变更。
- apply 结果必须返回明确状态：
  - `applied`
  - `rejected_permission`
  - `rejected_invalid_plan`
  - `failed_write`

## 7. 审计契约
- 必须记录以下事件：
  - `plan_created`
  - `plan_patched`
  - `plan_canceled`
  - `plan_applied`
  - `plan_apply_rejected`
- 审计字段至少包含：
  - `conversation_scope`
  - `version`（计划版本）
  - `actor`
  - `source`
  - `timestamp`
  - `diff_summary`（适用于 patch/apply）
