# 契约：Memory 控制命令

## 目的
定义 `/memory` 命令的语法、权限边界和状态行为，保证各渠道一致。

## 1. 命令集合
- `/memory note <text>`
- `/memory note --shared <text>`（显式共享写入）
- `/memory digest`
- `/memory audit`

### 1.1 解析优先级
- `/memory` 属于内建控制命令，解析优先级高于自然语言执行与 skill 路由。
- 兼容写法：`/memory ...` 与 `memory ...` 在语义上等价（渠道可决定是否自动补 `/`）。

## 2. 语法契约
- `<text>` 必须为非空文本，空白文本应返回参数错误。
- 命令解析失败必须返回可追踪错误，不得静默忽略。
- `note` 仅允许 `--shared` 作为 flag；未知 flag 必须报参数错误。
- `digest` 与 `audit` 不接受额外参数。

### 2.1 语法示例
- 合法：`/memory note 修复图片拼接参数`
- 合法：`/memory note --shared 已同步部署窗口`
- 合法：`/memory digest`
- 合法：`/memory audit`
- 非法：`/memory note`
- 非法：`/memory digest now`
- 非法：`/memory note --unknown abc`

## 3. 行为契约

### `/memory note <text>`
- 默认写入当前 `agent_id + project_id` 私有日记：`.agents/<agent_id>/memory/YYYY-MM-DD.md`。
- 如显式共享写入能力未启用，禁止写入项目共享层。
- 成功响应必须包含写入目标层级与时间戳。
- 成功响应格式（逻辑字段）：
  - `scope=agent-private`
  - `at=<RFC3339 timestamp>`

### `/memory note --shared <text>`
- 仅在 ACL 允许时写入项目共享日记：`memory/YYYY-MM-DD.md`。
- 若会话或策略不允许共享写入，必须返回 `rejected_acl`。
- 成功响应必须标记 `scope=project-shared`。
- ACL 允许条件（最小集合）：
  - `chat_mode=main`
  - owner allowlist 命中
  - 非降级模式（`acl_mode != degraded`）

### `/memory digest`
- 手工触发永远可用。
- 自动 digest 仅在配置开启时执行。
- 若目标是长期私有层，必须经过主会话 ACL 校验。
- 若存在 `pending/running` digest 任务，必须返回 `digest_in_progress`。
- 成功响应至少包含：
  - `job=<digest_job_id>`
  - `status=completed`
  - `output=<path>`
  - `auto_digest=<true|false>`

### `/memory audit`
- 输出当前作用域模板完整性、ACL 拒绝统计、预算裁剪统计、最近错误摘要。
- 输出仅包含元信息，不包含记忆正文。
- 成功响应至少包含：
  - `template_version`
  - `required`
  - `missing`
  - `acl_denied`
  - `budget_skipped`
  - `recent_errors`

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

### 5.1 推荐错误码
- `invalid_argument`: 参数无效（如空 note 文本）
- `rejected_acl`: ACL 拒绝
- `digest_in_progress`: digest 并发冲突
- `audit_corrupted`: 审计数据不可读/损坏

## 6. 一致性要求
- 渠道差异仅限回复样式，不得改变命令语义。
- 记忆命令优先级高于自然语言执行路径。

## 7. 契约测试映射
- `/memory note`：`tests/contract/memory_note_contract_test.go`
- `/memory digest`：`tests/contract/memory_digest_contract_test.go`
- `/memory audit`：`tests/contract/memory_audit_contract_test.go`
- `/memory note --shared`：`tests/contract/memory_note_shared_contract_test.go`
