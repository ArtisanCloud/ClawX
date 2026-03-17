# 契约：Memory 加载与审计

## 目的
定义“首轮执行前记忆加载”的输入、判定顺序、输出与审计要求。

## 1. 输入契约
加载器输入必须至少包含：
- `agent_id`
- `project_id`
- `route_key`
- `session_id`
- `chat_mode`（`main` 或 `shared`）
- `request_id`

缺失关键字段时必须返回降级模式并记录错误。

### 1.1 来源约束
- `agent_id/project_id/route_key` 来源必须与当前 session/project 路由决议一致。
- `chat_mode` 必须由 ACL 分类器输出，不允许由渠道层直接注入最终判定值。

## 2. 加载顺序契约（MVP）
1. `agent_private` 层：`~/.clawx/workspaces/<project_id>/.agents/<agent_id>/`
2. `project_shared` 层：`~/.clawx/workspaces/<project_id>/`
3. `main_private` 层：`MEMORY.md`（仅主会话且 ACL 通过）

每层内部遵循固定顺序：`IDENTITY/SOUL -> USER -> TOOLS -> AGENTS -> daily memory`。

### 2.1 可观察加载决策
- 每个候选文件必须落入以下决策之一：
  - `loaded`
  - `skipped_acl`
  - `skipped_budget`
  - `error`
- 对于 `skipped_acl/skipped_budget/error`，必须写入非空 reason。

## 3. ACL 契约
- `shared` 会话必须跳过 `main_private`。
- 任何路径逃逸（`..`、符号链接越界）必须拒绝并记审计。
- 不允许读取 `.agents/<other_agent_id>/`。

### 3.1 冲突降级
- 当 direct route identity 与 user identity 冲突时，必须进入 `acl_mode=degraded`。
- `degraded` 模式下必须执行最小权限：
  - 强制 `chat_mode=shared`
  - 强制 `allow_main_private=false`

## 4. 预算与截断契约
- 超预算时优先保留 `agent_private`。
- 裁剪必须记录 `skipped_budget` 条目及文件名。
- 预算策略变化必须可通过配置审计到版本。

### 4.1 预算统计输出
- 每轮加载后必须可计算：
  - `loaded_files` 数量
  - `denied_files` 数量（含 ACL 与预算拒绝）
  - `error_summary`（聚合错误）

## 5. 输出契约
加载器输出：
- `prompt_context`（注入后端的拼接结果）
- `loaded_files[]`
- `denied_files[]`
- `degraded`（布尔）
- `error_summary`（可空）

当 `degraded=true` 时，主流程继续执行，不得阻断任务。

## 6. 审计契约
每次首轮加载至少记录：
- `project_id`
- `agent_id`
- `route_key`
- `memory_scope`
- `memory_acl_mode`
- `memory_loaded_files`
- `memory_denied_files`

日志不得包含记忆正文。

### 6.1 审计字段检索
- 运行日志至少支持关键字检索：
  - `memory_load_audit:`
  - `memory_loaded_files=`
  - `memory_denied_files=`
  - `error_summary=`

### 6.2 对应测试
- ACL 与降级：`tests/integration/memory_shared_acl_test.go`、`tests/integration/memory_acl_degrade_test.go`
- 字段检索：`tests/integration/memory_audit_fields_search_test.go`
