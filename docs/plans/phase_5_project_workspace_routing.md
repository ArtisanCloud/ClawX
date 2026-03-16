# Phase 5 - Project Workspace Routing

## 阶段目标
- 在现有 channel/window/session 体系上新增 `project` 维度。
- 支持同一 bot 并发处理多个项目，且代码与会话上下文隔离。
- 固化 `/project` 命令闭环，并明确 `/new` 只创建会话。

## 交付状态（2026-03-13）

### 已完成
- 基础能力：
  - 项目领域模型、仓储接口、文件存储（`projects.json` / `bindings.json` / `proposals.json`）。
  - route key 归一化与路由入口项目判定（binding -> fallback）。
  - 会话键与窗口绑定纳入 `project_id` 作用域。
- US1（显式管理项目）：
  - `/project create/list/use/current` 完成并接入路由控制流。
  - workspace 自动初始化与状态查询可用。
- US2（同 bot 并发隔离）：
  - 多 route key 并发绑定不同项目不串线。
  - `/new` 保持“当前项目内新建会话”语义。
- US3（意图辅助切换）：
  - 建议切换（proposal）生成、确认、过期回收闭环完成。
  - confirm-first 语义已由测试覆盖。
- US4（可观测与恢复）：
  - `/project bind/unbind/audit/delete/repair` 全部可用。
  - broken 状态识别与修复链路可用。
- 收尾与门禁：
  - 契约文档：`project-command-contract.md`、`project-routing-contract.md`。
  - Phase 5 指标门禁测试（SC-001~SC-006）已补齐。
  - 快速验收脚本与发布门禁路径已更新。

### 测试覆盖状态
- Unit: `tests/unit/project_*`
- Integration: `tests/integration/project_*`
- Contract: `tests/contract/project_*`
- Metrics Gate: `tests/integration/project_routing_metrics_report_test.go`

## 当前范围结论
- P5（必须范围）：已完成。
- P5.5（增强范围）：已完成。
- Phase 5 可进入发布候选，发布前执行全量回归与指标门禁。

## 关键规则（已固化）
- `/new` 不得创建或切换项目。
- 项目切换只能显式命令或确认式切换完成。
- route key 需覆盖 `channel/instance/peer/thread`，线程优先。
- 项目目录默认在 `~/.clawx/workspaces/<project_id>`。

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
- 风险：指标样本不足导致门禁不可判定。
  - 缓解：维持 `>=200` 样本下限并输出 markdown 报告。

## 交付物
- 规格与任务：`specs/005-project-workspace-routing/*`
- 契约：`specs/005-project-workspace-routing/contracts/*`
- 阶段计划：`docs/plans/phase_5_project_workspace_routing.md`（本文件）
