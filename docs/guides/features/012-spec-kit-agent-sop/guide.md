# Spec Kit 驱动的 Agent 开发 SOP（版本：v1.0）

## 1. 目标
- 用一套固定流程和智能体交互，避免“想到哪做到哪”。
- 所有开发任务都先产出规范，再执行实现，再回写进度。
- 主干固定 `develop`，功能开发使用规范分支。

## 2. 交互铁律
- 用户输入以 `/` 开头：走确定性命令路径。
- 用户输入非 `/`：走自然语言 + LLM 规划路径。
- 实现前必须有：`specify -> plan -> tasks -> analyze`。
- 没有前置规范，智能体只能先补规范，不得直接改代码。

## 3. 标准对话 SOP（你可以直接照发）

### 3.1 创建需求与规范
1. `这是一个新功能：<一句话目标>。请先用 Spec Kit 生成规范，不要直接写代码。`
2. `执行 /speckit.specify，并把结果落到 specs/<feature-id>/spec.md。`
3. `执行 /speckit.plan，并落到 specs/<feature-id>/plan.md。`
4. `执行 /speckit.tasks，并落到 specs/<feature-id>/tasks.md。`
5. `执行 /speckit.analyze，并落到 specs/<feature-id>/analyze.md。`

### 3.2 开发与回归
1. `从 tasks 的第一阶段开始实现，按 phase 推进。`
2. `每完成一个阶段，更新 TASK_PLAN.md 和 TASK_EXECUTION.md。`
3. `只有遇到需要我决策的事项才停下，其它问题自行闭环处理。`

### 3.3 结果回执格式（强制）
- 只返回四类信息：
  - `结论`：完成/部分完成/阻塞
  - `产物`：文件路径清单
  - `证据`：测试命令和关键结果
  - `下一步`：一条可执行动作

## 4. 分支与主干策略
- 基线分支：`develop`
- 功能分支：`spec/<feature-id>-<slug>`（或 `feature/<feature-id>-<slug>`）
- 禁止从 `main/master` 起开发分支。
- Spec Kit 若自动建分支，必须受以下配置约束：
  - `specKit.baseBranch=develop`
  - `specKit.autoBranch=true`
  - `specKit.branchPrefix=spec`

## 5. 文档落盘要求
- 规范文档：`specs/<feature-id>/`
  - `spec.md`
  - `plan.md`
  - `tasks.md`
  - `analyze.md`
- 执行跟踪（agent workspace）：
  - `TASK_PLAN.md`
  - `TASK_EXECUTION.md`
  - `.clawx/workspace-state.json`

## 6. 验收清单
- 已生成并更新 `spec/plan/tasks/analyze` 四件套。
- 实现阶段与 `tasks.md` 一一对应。
- 每次推进都有 `TASK_EXECUTION.md` 记录。
- 回执包含“产物路径 + 下一步”，不再出现空泛模板话术。

## 7. 常用指令模板
- `请按 Spec Kit 流程推进，不要跳步骤。`
- `先补规范，再实现。`
- `继续执行，直到完成当前 phase。`
- `把当前 phase 的产物路径和下一步发我。`

## 8. 变更记录
| 日期 | 修改人 | 变更内容 |
|---|---|---|
| 2026-03-29 | Codex | 新增 Spec Kit Agent 开发 SOP |
