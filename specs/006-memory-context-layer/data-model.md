# 数据模型：Memory Context Layer

## 1. MemoryScopeKey（记忆作用域键）

### 作用
定义一次加载/写回操作的隔离边界。

### 字段
- `agent_id`
- `project_id`
- `route_key`
- `session_id`
- `chat_mode`：`main` | `shared`

### 约束
- `agent_id`、`project_id`、`route_key` 不能为空。
- `chat_mode=shared` 时，禁止访问长期私有层。

## 2. MemoryTemplateManifest（模板清单）

### 作用
描述 workspace 记忆骨架版本和必需文件。

### 字段
- `template_version`
- `required_files[]`（如 `SOUL.md`、`USER.md`、`TOOLS.md`、`AGENTS.md`）
- `optional_files[]`（如 `MEMORY.md`）
- `updated_at`

### 约束
- `template_version` 必须可比较（语义化或递增整数）。
- `required_files` 缺失时必须触发 repair 或自愈。

## 3. MemoryProfile（记忆加载配置）

### 作用
定义某作用域的加载顺序、预算与 ACL。

### 字段
- `scope_key`（关联 `MemoryScopeKey`）
- `load_order[]`（层级列表）
- `token_budget`
- `acl_mode`：`strict` | `degraded`
- `allow_main_private`

### 约束
- `token_budget > 0`。
- `chat_mode=shared` 时 `allow_main_private=false`。

## 4. MemoryLoadItem（加载项）

### 作用
表示一次加载流程中的单个文件候选。

### 字段
- `layer`：`agent_private` | `project_shared` | `main_private`
- `path`
- `size_bytes`
- `priority`
- `decision`：`loaded` | `skipped_acl` | `skipped_budget` | `error`
- `reason`

### 约束
- `decision` 与 `reason` 必须成对记录。
- `main_private` 仅允许在 `chat_mode=main` 且 ACL 通过时 `loaded`。

## 5. MemoryAuditRecord（记忆审计记录）

### 作用
记录加载/拒绝/错误元信息，用于排障与合规。

### 字段
- `timestamp`
- `scope_key`
- `loaded_files[]`
- `denied_files[]`
- `error_files[]`
- `acl_mode`
- `degraded`

### 约束
- 审计中不得写入敏感正文。
- 每次首轮加载至少产生 1 条审计记录。

## 6. MemoryJournalEntry（记忆日记条目）

### 作用
承载 `/memory note` 写回的原子条目。

### 字段
- `entry_id`
- `created_at`
- `author`（user/system）
- `scope`（agent-private 或 project-shared）
- `content_summary`
- `raw_ref`（原文位置引用）

### 约束
- 默认 `scope=agent-private`。
- `content_summary` 必须可审计且长度受限。

## 7. MemoryDigestJob（长期记忆汇总任务）

### 作用
管理 `/memory digest` 的执行与状态。

### 字段
- `job_id`
- `scope_key`
- `trigger_mode`：`manual` | `auto`
- `status`：`pending` | `running` | `completed` | `failed` | `rejected_acl`
- `started_at`
- `ended_at`
- `output_file`

### 状态流转
- `pending -> running -> completed`
- `pending -> rejected_acl`
- `running -> failed`

### 约束
- `trigger_mode=auto` 需显式配置开启。
- `chat_mode=shared` 时若目标为长期私有层，必须 `rejected_acl`。

## 实体关系
- `MemoryScopeKey` 1:N `MemoryAuditRecord`
- `MemoryScopeKey` 1:N `MemoryJournalEntry`
- `MemoryScopeKey` 1:N `MemoryDigestJob`
- `MemoryProfile` 1:N `MemoryLoadItem`
- `MemoryTemplateManifest` 作用于 `project_id` 与 `.agents/<agent_id>` 初始化/修复
