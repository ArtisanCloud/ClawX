# 契约：Schedule 运行时与可观测

## 目的
定义调度触发、并发控制、恢复与审计行为。

## 1. 调度触发契约
- 仅 `status=active` 的任务参与调度。
- 达到 `next_run_at` 后，任务应在可接受窗口内启动。
- 触发后必须重算下一次执行时间。

## 2. 并发与幂等契约
- 同一 `job_id` 同时最多一个活动执行实例。
- 若任务正在执行，再次触发应返回 `schedule_job_running`。
- 单任务失败不得阻断其他任务调度。

## 3. 恢复契约
- 服务启动后必须加载既有任务定义。
- 已删除任务不得恢复到调度集合。
- 恢复后任务继续按规则调度，不需要用户重新创建。

## 4. 作用域隔离契约
- 调度、查询、执行均受 `project+agent` 作用域约束。
- 跨 project 或跨 agent 的任务访问必须拒绝。
- route 级任务（若启用）不得被非目标 route 错误触发。

## 5. 可观测契约
每次执行至少记录：
- `job_id`
- `trigger_mode`（`scheduled`/`manual`）
- `started_at`
- `ended_at`
- `result`
- `error_summary`（失败时）
- `output_summary`

## 6. 图片清理任务契约
- 清理范围限定为当前项目空间内图片集合目录。
- 默认仅清理超过 30 天的记录。
- 清理结果必须输出：扫描数量、删除数量、保留数量、失败数量。

## 7. 测试映射
- 命令契约：`tests/contract/schedule_command_contract_test.go`
- 运行时契约：`tests/integration/schedule_runtime_flow_test.go`
- 隔离契约：`tests/integration/schedule_scope_isolation_test.go`
- 自然语言映射：`tests/integration/schedule_nl_mapping_test.go`
