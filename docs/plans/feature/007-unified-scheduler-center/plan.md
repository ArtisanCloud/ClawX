# 007 Unified Scheduler Center 计划草案（v0.1）

## 0. 相关文档
- 现有能力基线（Memory Context Layer）：`docs/plans/feature/006-memory-context-layer/plan.md`
- 现有服务托管能力：`/service start|stop|status|logs`

## 1. 背景
当前 ClawX 已具备统一服务托管（`/service`）与自然语言到服务命令映射能力，但缺少“统一定时任务中心”。

结果是：
- Agent 无法把“每周清理图片记录”这类需求注册为平台级计划任务。
- 定时逻辑分散在业务内部 ticker，无法统一查看、暂停、审计与治理。

## 2. 目标
在 ClawX 平台层新增统一调度中心，支持：
- 多 Agent 可注册、查询、暂停、恢复、立即执行任务。
- 任务按 `project_id + agent_id (+ route_key)` 隔离。
- 自然语言触发注册（例如“每周清理图片记录”）。
- 可观测：执行历史、下次执行时间、失败原因、日志关联。

## 3. 范围与非目标
### 3.1 本期范围（MVP）
- 新增 `/schedule` 控制命令。
- 新增调度器应用服务与文件持久化。
- 新增自然语言映射（图片工具清理用例优先）。
- 新增契约测试 + 集成测试。

### 3.2 非目标（本期不做）
- 分布式多节点抢占执行。
- 秒级高精度任务编排。
- 可视化管理后台（后续独立 feature）。

## 4. 对齐策略（落到 ClawX）
### 4.1 数据隔离模型
任务主键建议：`job_id`（唯一）
隔离维度：`project_id + agent_id + route_scope`

建议持久化目录：
```text
~/.clawx/workspaces/<project_id>/.agents/<agent_id>/scheduler/
├── jobs.json              # 任务定义
└── runs/
    └── <job_id>.jsonl     # 执行记录（追加写）
```

`route_scope` 规则（MVP）：
- `global`：项目+agent 级任务（默认）
- `route:<route_key>`：仅绑定某 route 的任务

### 4.2 任务执行模型
- 调度器常驻在 `clawx serve` 进程内。
- 仅负责“触发”，具体动作由任务类型执行器完成。
- MVP 执行器：
  - `image.cleanup`：清理 `.image/collections` 过期数据
  - `command.exec`：执行受限白名单命令（仅内部使用）

## 5. 命令设计（MVP）
新增控制命令：
- `/schedule add <name> --cron "<expr>" --task <task_type> [--arg key=value ...]`
- `/schedule list`
- `/schedule pause <name|job_id>`
- `/schedule resume <name|job_id>`
- `/schedule run <name|job_id>`
- `/schedule remove <name|job_id>`
- `/schedule status <name|job_id>`

错误语义：
- 参数非法：`invalid_schedule_command`
- 任务不存在：`schedule_job_not_found`
- 重名冲突：`schedule_job_conflict`
- 执行失败：`schedule_run_failed`

## 6. 自然语言对齐策略
路由层增加定向映射（先做确定性规则）：
- “每周清理图片记录/图片集合/长图缓存”
  - 映射为：创建 `image.cleanup` 周期任务（默认每周日 03:00）
- “查看定时任务/调度状态”
  - 映射为：`/schedule list`
- “暂停图片清理定时任务”
  - 映射为：`/schedule pause image-cleanup`

说明：
- 自然语言只做“显式意图 -> 标准命令”转换。
- 真正执行统一走 `/schedule` 语义，保证可审计与可回放。

## 7. 实施阶段
### Phase A：领域与持久化（P1）
- 新增 `internal/application/scheduler` 服务与模型。
- 新增文件仓储 `internal/infrastructure/persistence/scheduler_file_store.go`。
- 新增运行记录仓储（jsonl append）。

### Phase B：控制命令与路由接入（P1）
- 新增 `internal/application/command/schedule_command.go`。
- Router 识别 `/schedule`。
- 控制流新增 `handleScheduleControlCommand`。

### Phase C：调度运行器（P2）
- 在 `cmd/clawx/main.go` 启动 scheduler runner。
- runner 周期扫描待执行任务并触发。
- 增加并发保护与停机优雅退出。

### Phase D：自然语言映射与图片清理执行器（P2）
- 在 Router 增加 schedule NL mapping。
- 新增 `image.cleanup` 执行器（按保留周期清理）。
- 支持 dry-run 与真实执行日志。

### Phase E：文档与验收（P2）
- 补充 feature guide 用例：
  - 用户说“每周清理图片记录” -> 系统注册任务 -> 周期执行 -> 可查询状态
- 补充排障文档：任务未触发、权限不足、路径异常。

## 8. 验收标准（MVP）
- 用户可通过 `/schedule` 全流程管理任务（增删改查/暂停恢复/立即执行）。
- 用户可通过自然语言注册“每周清理图片记录”任务。
- 任务严格按 `project + agent` 隔离，互不干扰。
- 调度执行有可追踪记录（开始时间、结束时间、结果、错误）。
- 重启 `clawx serve` 后任务定义可恢复并继续调度。

## 9. 测试计划
- 单元测试：
  - cron 解析与 next run 计算
  - 状态流转（active/pause/resume/remove）
  - 仓储读写与并发安全
- 契约测试：
  - `/schedule` 命令语义与错误码
  - 自然语言映射到标准命令
- 集成测试：
  - 注册周任务并触发执行
  - 多 project/agent 隔离
  - 重启后恢复调度

## 10. 风险与缓解
- 风险：任务失控导致重复执行。
  - 缓解：任务锁 + 最近执行窗口幂等保护。
- 风险：自然语言误判误注册。
  - 缓解：高风险动作要求显式确认；默认仅支持白名单映射。
- 风险：清理策略误删。
  - 缓解：先 dry-run、保留回收站窗口、日志可回溯。

## 11. 建议里程碑
- M1（1-2 天）：Phase A 完成（模型/仓储/基本测试）
- M2（1-2 天）：Phase B 完成（命令与路由）
- M3（2-3 天）：Phase C+D 完成（runner + image.cleanup + NL）
- M4（1 天）：Phase E 文档与回归

## 12. 下一步
建议基于当前分支 `007-unified-scheduler-center` 开始实现 Phase A + Phase B，先交付可管理不可自动触发的骨架；再增量接入 runner 与图片清理执行器。
