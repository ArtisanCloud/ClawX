# 快速验证：Skill Registry 与意图路由

## 1. 前置条件

- 已在分支 `003-skill-intent-router`。
- 本地 Go 环境可用（Go 1.23）。
- 已完成基础服务配置（Discord/Telegram 至少一个可用）。

## 2. 准备一个最小 Skill

在用户技能目录创建：

```text
~/.synapsex/skills/echo/SKILL.md
```

示例内容：

```md
---
name: echo
description: 回显输入内容
---

你是一个回显技能。请原样输出用户输入。
```

## 3. 启动服务

```bash
go run ./cmd/synapsex
```

确认服务正常启动，并可看到健康日志或渠道适配器启动日志。

## 4. 校验 Skill 注册

目标结果：
- `echo` 出现在 Skill 列表中
- 状态为 `active`

可执行：

```bash
go run ./cmd/synapsex skill list
go run ./cmd/synapsex skill reload
```

若状态异常：
- `invalid`：检查 `SKILL.md` frontmatter 的 `name/description`
- `shadowed`：检查是否有同名上位来源 Skill

## 5. 校验路由优先级

在渠道中按顺序发送：

1. 控制命令（如 `/list`）  
   预期：命中控制路径，不进入 Skill。
2. 显式技能命令（如 `/skill echo hello`）  
   预期：命中 `echo` Skill。
3. 普通任务文本（无明确 Skill 名）  
   预期：未命中规则时进入 LLM 兜底或普通任务回退。

## 6. 校验“多候选自动选择”

构造可同时命中多个 Skill 的输入。  
预期：
- 系统自动选择单一候选执行。
- 不出现“请用户二次选择 Skill”的交互。

## 7. 校验权限默认策略

在未显式放开的用户或频道触发 Skill：  
预期：
- 请求被拒绝
- 返回可理解授权提示

## 8. 校验阈值回退与 DM pairing 生命周期

1. 将 LLM 兜底阈值设置为较高值，发送模糊意图文本  
   预期：请求回退到普通任务路径。
2. 让 DM pairing 进入失效状态后再次触发 Skill  
   预期：返回 `pairing_expired`（或同义错误），并提示重新配对。

## 9. 日志检查

确认每次请求都可看到路由审计字段：

```text
intent.kind
intent.reason
intent.skill_name
intent.confidence
```

## 10. 回归检查

执行：

```bash
go test ./...
```

预期：
- 现有 Session 控制能力与渠道基础行为无回归。
- `/new`、`/resume`、`/list`、`/cancel` 语义不回归。

## 11. 已覆盖测试文件（实现态）

- `tests/integration/skill_registry_load_test.go`
- `tests/integration/intent_router_flow_test.go`
- `tests/integration/intent_router_threshold_test.go`
- `tests/integration/skill_permission_policy_test.go`
- `tests/integration/skill_dm_pairing_lifecycle_test.go`
- `tests/integration/control_commands_regression_test.go`
- `tests/integration/intent_router_performance_test.go`
- `tests/contract/intent_router_contract_test.go`
- `tests/contract/skill_registry_contract_test.go`
