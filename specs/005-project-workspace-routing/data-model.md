# 数据模型：项目空间隔离与路由

## 1. 项目（Project）

### 作用
表示一个独立项目空间，承载 workspace 与运行状态。

### 字段
- `project_id`: 项目唯一标识（建议 kebab-case）
- `name`: 展示名称
- `workspace_path`: 项目工作目录
- `status`: `active` / `inactive` / `broken`
- `default_agent`: 默认 agent（初期可选）
- `created_at`
- `updated_at`

### 约束
- `project_id` 全局唯一。
- `workspace_path` 必须存在或可创建。
- `status=broken` 时禁止自动执行任务。

---

## 2. 项目注册表（Project Registry）

### 作用
维护所有项目元数据与默认项目指针。

### 字段
- `version`
- `default_project_id`
- `projects`: `project_id -> Project`
- `updated_at`

### 约束
- `default_project_id` 必须存在于 `projects`。
- 文件写入需原子替换。

---

## 3. 路由键（Route Key）

### 作用
标识一条消息来源上下文，用于项目绑定与会话定位。

### 字段
- `channel`
- `instance`
- `peer_kind`: `direct` / `channel` / `group`
- `peer_id`
- `thread_id`（可选）
- `normalized_key`（规范化字符串）

### 约束
- `normalized_key` 必须稳定可重算。
- 同一来源上下文每次入站生成相同 key。

---

## 4. 项目绑定（Project Binding）

### 作用
维护 `route_key -> project_id` 映射。

### 字段
- `route_key`
- `project_id`
- `updated_at`
- `updated_by`
- `source`: `manual` / `auto_confirmed` / `fallback_seed`

### 约束
- 一个 `route_key` 任意时刻只绑定一个 `project_id`。
- 目标 `project_id` 必须存在且非 `broken`。

---

## 5. 项目会话键（Project Session Key）

### 作用
为会话系统增加项目维度，防止跨项目冲突。

### 字段
- `agent_id`
- `project_id`
- `channel`
- `chat_scope`
- `scope_id`
- `session_id`

### 约束
- 键中必须包含 `project_id`。
- 同一 `session_id` 不可跨项目复用。

---

## 6. 切换建议（Project Switch Proposal）

### 作用
承载意图识别产生的待确认项目切换。

### 字段
- `proposal_id`
- `route_key`
- `from_project_id`
- `to_project_id`
- `confidence`
- `reason`
- `created_at`
- `expires_at`
- `status`: `pending` / `accepted` / `rejected` / `expired`

### 约束
- `pending` 记录必须有 `expires_at`。
- 超时后只能变为 `expired`，不得再被接受。

---

## 7. 项目审计事件（Project Audit Event）

### 作用
追踪关键项目路由行为，用于排障与合规。

### 字段
- `timestamp`
- `event_type`: `project_switch` / `binding_update` / `routing_fallback` / `proposal_created`
- `route_key`
- `project_id`
- `operator`
- `metadata`

### 约束
- 每次切换、绑定、fallback 必须至少记录 1 条审计事件。
