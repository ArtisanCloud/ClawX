# 契约：Memory 控制命令

## 目的
定义 `/memory` 命令的语法、权限边界和状态行为，保证各渠道一致。

## 1. 命令集合
- `/memory note <text>`
- `/memory note --shared <text>`（显式共享写入）
- `/memory digest`
- `/memory audit`

## 2. 语法契约
- `<text>` 必须为非空文本，空白文本应返回参数错误。
- 命令解析失败必须返回可追踪错误，不得静默忽略。

## 3. 行为契约

### `/memory note <text>`
- 默认写入当前 `agent_id + project_id` 私有日记：`.agents/<agent_id>/memory/YYYY-MM-DD.md`。
- 如显式共享写入能力未启用，禁止写入项目共享层。
- 成功响应必须包含写入目标层级与时间戳。

### `/memory note --shared <text>`
- 仅在 ACL 允许时写入项目共享日记：`memory/YYYY-MM-DD.md`。
- 若会话或策略不允许共享写入，必须返回 `rejected_acl`。
- 成功响应必须标记 `scope=project-shared`。

### `/memory digest`
- 手工触发永远可用。
- 自动 digest 仅在配置开启时执行。
- 若目标是长期私有层，必须经过主会话 ACL 校验。

### `/memory audit`
- 输出当前作用域模板完整性、ACL 拒绝统计、预算裁剪统计、最近错误摘要。
- 输出仅包含元信息，不包含记忆正文。

## 4. ACL 契约
- `chat_mode=shared`：禁止读取/写入长期私有层（如 `MEMORY.md`）。
- `chat_mode=main`：允许长期私有层，但需满足 owner allowlist。
- 跨项目、跨 agent 的私有路径访问必须拒绝。

## 5. 错误契约
以下场景必须返回错误码与简短原因：
- ACL 拒绝（`rejected_acl`）
- 目标路径不可写或缺失且自愈失败
- digest 进行中重复触发
- 审计数据损坏或不可读取

## 6. 一致性要求
- 渠道差异仅限回复样式，不得改变命令语义。
- 记忆命令优先级高于自然语言执行路径。
