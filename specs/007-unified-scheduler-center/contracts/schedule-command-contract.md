# 契约：Schedule 控制命令

## 目的
定义 `/schedule` 命令语法、状态行为、错误语义与渠道一致性。

## 1. 命令集合
- `/schedule add <name> --cron "<expr>" --task <task_type> [--arg key=value ...]`
- `/schedule list`
- `/schedule status <name|job_id>`
- `/schedule pause <name|job_id>`
- `/schedule resume <name|job_id>`
- `/schedule run <name|job_id>`
- `/schedule remove <name|job_id>`

### 1.1 解析优先级
- `/schedule` 属于内建控制命令，优先级高于自然语言执行与技能路由。
- 兼容写法：`/schedule ...` 与 `schedule ...` 在语义上等价（渠道可决定是否自动补 `/`）。

## 2. 语法契约
- `add` 必须包含 `name`、`--cron`、`--task`。
- `--arg` 支持重复提供，格式必须为 `key=value`。
- `list` 不接受额外参数。
- `status/pause/resume/run/remove` 必须且仅接受 1 个目标标识。

### 2.1 语法示例
- 合法：`/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`
- 合法：`/schedule list`
- 合法：`/schedule pause image-cleanup`
- 非法：`/schedule add image-cleanup --task image.cleanup`
- 非法：`/schedule run`
- 非法：`/schedule remove image-cleanup now`

## 3. 行为契约

### `/schedule add`
- 在当前 `project+agent` 作用域创建任务。
- 同作用域同名任务冲突时返回冲突错误。
- 成功返回至少包含：`job_id`、`name`、`status`、`next_run_at`。

### `/schedule list`
- 仅返回当前作用域任务。
- 每条任务至少包含：`name`、`status`、`next_run_at`、`last_run_result`。

### `/schedule status`
- 返回目标任务详情与最近执行记录。
- 任务不存在时返回明确错误。

### `/schedule pause` 与 `/schedule resume`
- `pause` 后不得再自动触发。
- `resume` 后应恢复调度并更新下一次执行时间。

### `/schedule run`
- 立即执行目标任务一次。
- 若该任务已在执行中，返回并发冲突错误。
- 手工执行结果必须写入执行记录。

### `/schedule remove`
- 删除后任务不可再被自动触发。
- 删除动作应保留历史执行记录用于审计。

## 4. 默认业务规则契约
- 自然语言“每周清理图片记录”在未指定时间时，创建每周日 03:00 任务。
- 若未显式配置项目时区，周期表达按 UTC 解释；若配置项目时区，则按项目时区解释并在响应中返回时区标识。
- 图片清理默认保留 30 天。
- 任务单次失败后仅标记失败，不自动暂停或删除，等待下周期执行。

## 5. 错误契约
推荐错误码：
- `invalid_schedule_command`: 命令或参数非法
- `schedule_job_not_found`: 任务不存在
- `schedule_job_conflict`: 同名冲突
- `schedule_job_running`: 任务正在执行
- `schedule_run_failed`: 执行失败
- `rejected_scope`: 作用域越权

### 5.1 可行动反馈约束
- 所有错误响应必须至少包含：`error_code`、`reason`、`next_action`。
- `next_action` 必须给出可执行建议（例如修正参数、查看任务列表、切换项目或 agent）。

## 6. 一致性要求
- 各渠道回复样式可不同，但命令语义与状态流转必须一致。
- 命令执行必须可追踪到结构化执行记录。
