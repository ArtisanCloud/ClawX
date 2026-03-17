# 数据模型：Unified Scheduler Center

## 1. ScheduleScope（任务作用域）

### 作用
定义任务的可见性与执行边界。

### 字段
- `project_id`
- `agent_id`
- `route_scope`：`global` | `route:<route_key>`

### 约束
- 默认 `route_scope=global`。
- `project_id`、`agent_id` 必须非空。
- 跨作用域访问必须拒绝。

## 2. ScheduleJob（定时任务）

### 作用
描述可被周期触发的任务定义。

### 字段
- `job_id`
- `name`
- `scope`（关联 `ScheduleScope`）
- `schedule_expr`
- `task_type`（如 `image.cleanup`）
- `task_args`（键值）
- `status`：`active` | `paused` | `removed`
- `next_run_at`
- `last_run_at`
- `created_at`
- `updated_at`

### 约束
- 同一 `scope` 下 `name` 必须唯一。
- `status=removed` 的任务不得再被调度触发。
- 未指定时间的“每周清理图片记录”应固化为每周日 03:00。

## 3. ScheduleRunRecord（任务执行记录）

### 作用
记录单次任务执行结果与诊断信息。

### 字段
- `run_id`
- `job_id`
- `started_at`
- `ended_at`
- `result`：`success` | `failed` | `skipped`
- `error_code`（可空）
- `error_message`（可空）
- `summary`
- `trigger_mode`：`scheduled` | `manual`

### 约束
- `started_at <= ended_at`（若已结束）。
- `result=failed` 时 `error_message` 必须非空。

## 4. ScheduleExecutionLock（任务执行锁）

### 作用
防止同一任务并发重复执行。

### 字段
- `job_id`
- `lock_token`
- `acquired_at`
- `expires_at`

### 约束
- 同一 `job_id` 在同一时间最多存在一个有效锁。
- 超时锁需可被回收并记录异常。

## 5. ScheduleIntentTemplate（自然语言模板）

### 作用
把自然语言映射到标准调度动作。

### 字段
- `template_id`
- `match_rules[]`
- `mapped_command`
- `default_args`
- `enabled`

### 约束
- 模板必须可追踪到对应标准命令。
- 误匹配风险高的模板默认关闭。

## 6. ImageCleanupPolicy（图片清理策略）

### 作用
定义图片集合的清理边界。

### 字段
- `retention_days`（默认 30）
- `target_path_pattern`
- `dry_run`
- `updated_at`

### 约束
- `retention_days > 0`。
- 默认清理仅删除超过保留期的记录。

## 状态流转

### ScheduleJob
- `active -> paused -> active`
- `active -> removed`
- `paused -> removed`

### ScheduleRunRecord
- `scheduled/manual -> success`
- `scheduled/manual -> failed`
- `scheduled/manual -> skipped`

## 实体关系
- `ScheduleScope` 1:N `ScheduleJob`
- `ScheduleJob` 1:N `ScheduleRunRecord`
- `ScheduleJob` 1:1 `ScheduleExecutionLock`（运行时）
- `ScheduleIntentTemplate` 用于生成/更新 `ScheduleJob`
- `ImageCleanupPolicy` 被 `task_type=image.cleanup` 的 `ScheduleJob` 引用
