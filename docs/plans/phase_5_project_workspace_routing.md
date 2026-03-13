# Phase 5 - Project Workspace Routing

## 阶段目标
- 在现有 channel/window/session 体系上新增 `project` 维度。
- 支持同一 bot 并发处理多个项目，且代码与会话上下文隔离。
- 固化 `/project` 命令闭环，并明确 `/new` 只创建会话。

## 范围
### P5（必须）
- 项目注册表（project metadata）
- route key 到项目绑定
- `/project create/list/use/current`
- `project_id` 纳入会话键
- 默认项目 fallback 与审计日志

### P5.5（增强）
- 意图命中跨项目时“建议切换 + 用户确认”
- `/project bind/unbind/audit/delete` 运维命令
- 异常状态修复（broken 项目、脏绑定）

## 关键规则
- `/new` 不得创建或切换项目。
- 项目切换只能显式命令或确认式切换完成。
- route key 需覆盖 `channel/instance/peer/thread`，线程优先。
- 项目目录默认在 `~/.clawx/workspaces/<project_id>`。

## 里程碑

### M1: 显式项目管理（P5 MVP）
- `/project create/list/use/current` 可用。
- `projects.json` 与 `bindings.json` 可稳定读写。
- 路由可按绑定命中项目。

### M2: 并发隔离闭环
- 会话键加入 `project_id`。
- 两个 route key 绑定不同项目后可并行执行且不串线。
- `/new` 在当前项目内新建会话且不改变项目绑定。

### M3: 意图辅助与治理
- 高置信跨项目命中输出 proposal，确认后切换。
- 审计、修复、删除保护命令可用。

## 验收标准（阶段门禁）
- 同一 bot 下至少 2 个项目并发执行无串线。
- `/project use` 生效后首条请求命中目标项目。
- `/new` 语义不回退。
- 意图切换全链路必须是 confirm-first。
- 关键状态变更（switch/bind/fallback）具备可检索审计日志。

## 风险与缓解
- 风险：route key 不稳定导致误绑定。
  - 缓解：统一规范化算法并加单元测试。
- 风险：文件并发写导致 registry/binding 损坏。
  - 缓解：原子写入 + 文件锁 + 损坏检测。
- 风险：意图建议过于频繁干扰用户。
  - 缓解：建议节流 + 最短冷却窗口。

## 交付物
- 规格与任务：`specs/005-project-workspace-routing/*`
- 阶段计划：`docs/plans/phase_5_project_workspace_routing.md`（本文件）
