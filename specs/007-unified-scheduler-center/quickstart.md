# 快速启动：Unified Scheduler Center

## 目标
完成 007 功能 MVP 验收：
- `/schedule` 控制命令全流程可用
- 自然语言可注册“每周清理图片记录”
- 任务按 `project+agent` 隔离
- 任务执行可观测，服务重启后可恢复

## 开发前准备
1. 分支与规格
- 当前 feature：`007-unified-scheduler-center`
- 阅读：`spec.md`、`plan.md`、`research.md`、`contracts/`

2. 启动与测试命令
- 服务启动：`go run ./cmd/clawx serve`
- 全量测试：`go test ./...`

## 手工验收脚本

### A. 命令全流程（US1）
1. `/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`
2. `/schedule list`
3. `/schedule pause image-cleanup`
4. `/schedule resume image-cleanup`
5. `/schedule run image-cleanup`
6. `/schedule remove image-cleanup`

预期：
- 每步均返回明确状态；`list/status` 可看到状态变化；删除后不再被调度。

### B. 自然语言注册（US2）
1. 发送：`每周清理图片记录`
2. 查询：`查看定时任务`
3. 触发：`立即执行图片清理任务`

预期：
- 创建成功且默认时间为每周日 03:00（未配置项目时区时按 UTC）。
- 默认保留策略为 30 天。
- 执行结果包含清理摘要。

### C. 隔离验证（US3）
1. 在 Project A 创建 `image-cleanup`。
2. 切换到 Project B 查询任务。
3. 在同项目不同 agent 重复查询。

预期：
- 仅能看到当前 `project+agent` 作用域任务。
- 不出现跨作用域可见或可执行。

### D. 可观测与恢复（US4）
1. 人为构造一次失败执行并查询状态。
2. 重启 `clawx serve`。
3. 再次查询任务并等待下一次触发。

预期：
- 可查看失败时间与失败原因。
- 重启后任务定义不丢失，调度继续生效。

## 建议自动化覆盖
- 单元：调度表达解析、状态流转、执行锁行为
- 契约：`/schedule` 命令语义与错误码
- 集成：自然语言映射、任务调度触发、重启恢复、作用域隔离

## 完成检查
- FR-001 ~ FR-019 对应测试通过。
- SC-001 ~ SC-006 的采样口径可执行并可产出报告。
- 关键日志可检索：任务创建、触发、失败、恢复、越权拒绝。
