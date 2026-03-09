# 契约：Intent Router 判定与优先级

## 目的
定义入站消息如何被判定为控制命令、Skill 调用或普通任务，并约束多候选场景的自动选择规则。

## 1. 入站输入结构

Intent Router 接收的最小输入结构：

```text
conversation_id
channel
user_id
text
available_skills
```

可选输入：

```text
session_id
agent_id
metadata
```

## 2. 路由输出结构

Router 必须输出以下统一结构：

```text
kind              # control | skill | task
skill_name        # kind=skill 时必填
reason            # 决策来源
confidence        # 仅 LLM 兜底时存在
```

## 3. 固定优先级（必须）

单条消息按以下顺序判定：

1. 控制命令
2. 显式 `/skill <name>`
3. Skill 精确名匹配
4. Skill 别名匹配
5. LLM 兜底判定
6. 普通任务回退

## 4. 多候选处理（必须）

- 当命中多个候选 Skill 时，系统必须自动选择单一候选继续处理。
- 系统不得要求用户二次选择 Skill。
- 自动选择必须遵循“固定优先级 + 稳定顺序”，确保同样输入得到同样结果。

## 5. 低置信度回退（必须）

- LLM 路径下，低于阈值的候选不得进入 Skill 执行。
- 低置信度时必须回退到 `kind=task`。
- 阈值必须可配置，并定义默认值（默认 `0.72`）。

## 6. 错误语义

- 无效控制命令：返回可理解命令错误，不进入 Skill/Task 路径。
- 请求了不存在或不可用 Skill：返回可理解错误并停止执行。
- 权限不足：返回授权错误，不得降级为隐式 Skill 执行。
- 错误类别至少应区分：`permission_denied`、`skill_not_found`、`skill_disabled`、`skill_invalid`、`pairing_expired`。

## 7. 控制命令兼容性

- 引入 Skill 路由后，现有控制命令语义（`/new`、`/resume`、`/list`、`/cancel`）必须保持不变。
- 控制命令匹配结果必须优先于任何 Skill 路由结果。

## 8. 日志要求

每次判定至少记录：

```text
conversation_id
channel
intent.kind
intent.reason
intent.skill_name
intent.confidence
duration_ms
```
