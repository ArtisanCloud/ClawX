# 快速启动：Config Intent Plan

## 目标
完成 009 功能 MVP 验收：
- 指令与自然语言统一进入配置计划
- 支持会话内自然语言 patch
- 混合意图先澄清、低置信度仅建议
- `apply` 权限治理生效
- 配置审计可追踪

## 开发前准备
1. 分支与规格
- 当前 feature：`009-config-intent-plan`
- 阅读：`spec.md`、`plan.md`、`research.md`、`contracts/`

2. 启动与测试命令
- 服务启动：`go run ./cmd/clawx serve`
- 全量测试：`go test ./...`

## 手工验收脚本

### A. 双入口统一（US1）
1. 发送：`/config plan 创建 agent bid-all`
2. 发送：`/config show`
3. 发送：`帮我创建一个 review agent`
4. 发送：`/config show`

预期：
- 两种入口都能生成/更新 pending plan。
- `show` 可看到统一结构，不出现“Agent Direct”误链路。

### B. 自然语言 patch（US2）
1. 先确保会话已有 pending plan。
2. 发送：`把 workspace 改成 /home/ubuntu/.clawx/workspaces/bid-all`
3. 发送：`把 timeout 改成 900`
4. 发送：`/config show`

预期：
- 字段变更生效。
- patch 历史可追踪。
- 无 pending plan 时会返回明确错误。

### C. 混合意图澄清与低置信保护（Phase 8）
1. 发送：`请创建 agent bid-all，同时 go test ./...`
2. 回复：`配置`
3. 发送：`agent 怎么配比较好？`

预期：
- 第 1 步先返回澄清提示（回复“配置/任务”）。
- 第 2 步确认后才真正创建 pending plan。
- 第 3 步只返回建议，不自动改 pending plan。

### D. 权限治理（US3）
1. 非管理员发送：`/config apply`
2. 管理员发送：`/config apply`

预期：
- 非管理员被拒绝（不可写盘）。
- 管理员可成功写盘并收到重启提示。

### E. 审计与回退（US4）
1. 触发：创建计划 -> patch -> cancel。
2. 触发：创建计划 -> patch -> apply。
3. 检查结构化日志字段。

预期：
- 创建、补丁、取消、应用、拒绝事件可检索。
- 普通任务消息在未命中配置意图时，仍走原执行链路。

## 建议自动化覆盖
- 单元：配置意图判定、混合意图澄清、低置信度降级、patch 版本一致性
- 契约：配置意图路由契约、计划生命周期契约
- 集成：会话隔离、管理员边界、写盘仅 apply、普通任务回归

## 完成检查
- FR-001 ~ FR-022 对应测试通过。
- SC-001 ~ SC-006 可产出验证样本。
- 关键日志可检索：`plan_created/plan_patched/plan_canceled/plan_applied/plan_apply_rejected`。
