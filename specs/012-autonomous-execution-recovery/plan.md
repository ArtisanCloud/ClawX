# Implementation Plan: 012-autonomous-execution-recovery

**Branch**: `012-autonomous-execution-recovery` | **Date**: 2026-03-23 | **Spec**: [/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/spec.md](/home/ubuntu/workspace/ClawX/specs/012-autonomous-execution-recovery/spec.md)
**Input**: Feature specification from `/specs/012-autonomous-execution-recovery/spec.md`

## Summary

在现有结构化执行链路上新增“失败分类 -> 自动恢复 -> 最小打扰升级提问 -> 审计回放”自治闭环，提升 Agent 执行自主性并降低用户被动决策负担。

## Technical Context

**Language/Version**: Go 1.23  
**Primary Dependencies**: `cmd/clawx/main.go`、`internal/application/skillorchestrator`、现有 trace/audit logger  
**Storage**: `~/.clawx/logs/trace.jsonl` + 新增 `~/.clawx/logs/autonomy.jsonl`  
**Testing**: `go test ./...` + contract/integration 回归  
**Constraints**:
- 非 `/` 保持 LLM-first；`/command` 保持命令路径。
- 不允许绕过 `control_plan` / `requirement_sync` 校验门禁。
- 自治恢复策略必须可限制重试次数与退出条件。

## Constitution Check

| Gate | 检查项 | 结果 |
|------|--------|------|
| LLM-first | 非命令路径不引入规则化意图判定 | PASS |
| Safety | 结构化执行与 allowlist 边界保持不变 | PASS |
| Auditability | 自治链路全程可追溯 | PASS |
| Scope Isolation | 与 011 能力边界清晰 | PASS |

## Project Structure

```text
specs/012-autonomous-execution-recovery/
├── spec.md
├── plan.md
└── tasks.md
```

涉及代码目录（规划）：

```text
cmd/clawx/
  main.go
internal/application/autonomy/
  classifier.go
  recovery_registry.go
  executor.go
  escalation_policy.go
internal/infrastructure/logging/
  autonomy_log.go
tests/
  contract/
  integration/
```

## Phase Design

1. **Phase A - Failure Classification**
- 统一错误归因接口与分类枚举。
- 在执行失败分支接入分类器并写 trace。

2. **Phase B - Recovery Registry**
- 定义恢复策略注册中心（按 FailureClass 匹配）。
- 支持重试、退避、回退与终止条件。

3. **Phase C - Escalation Policy**
- 定义最小打扰升级策略（何时问用户、问什么）。
- 输出统一问题模板：已尝试 + 失败摘要 + 推荐动作。

4. **Phase D - Audit & Replay**
- 新增自治日志 `autonomy.jsonl`。
- 提供基础报表/筛选能力（按 agent/channel/conversation）。

5. **Phase E - UX Conventions**
- 统一任务回执规范：接收/进度/完成/失败。
- 默认简报输出，详细内容按需展开。

## Risks

- 风险：恢复策略过强导致错误重试放大。  
  缓解：全策略强制最大重试与熔断条件。
- 风险：升级提问过频影响体验。  
  缓解：策略层定义打扰预算和场景白名单。
- 风险：自治层与 011 编排耦合过深。  
  缓解：通过标准输入输出契约解耦（Plan/Result）。
