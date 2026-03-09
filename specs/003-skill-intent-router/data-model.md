# 数据模型：Skill Registry 与意图路由

## 1. Skill 定义（Skill Definition）

### 作用
表示一个可被系统发现并执行的 Skill 基本定义，来源于 `SKILL.md`。

### 字段
- `name`: Skill 规范名称（唯一业务键）。
- `description`: Skill 描述，用于展示与意图匹配。
- `instruction_body`: Skill 指令正文内容。
- `source`: 来源类型（`user` / `workspace` / `builtin`）。
- `base_dir`: Skill 根目录路径。
- `manifest_path`: `SKILL.md` 文件路径。
- `loaded_at`: 最近一次成功加载时间。

### 约束
- `name` 必须非空，且在同一快照中唯一生效。
- 缺少 `name` 或 `description` 时视为无效 Skill。

---

## 2. Skill 目录条目（Skill Catalog Entry）

### 作用
表示注册中心中的可见条目，用于列表展示与路由决策。

### 字段
- `key`: 条目唯一键（由 `source + name + path` 派生）。
- `skill_name`: 关联 Skill 名称。
- `status`: `active` / `invalid` / `disabled` / `shadowed`。
- `shadowed_by`: 当状态为 `shadowed` 时，指向当前激活条目键。
- `errors`: 加载或校验错误摘要列表。
- `updated_at`: 状态更新时间。

### 约束
- 同名 Skill 最多仅一个 `active` 条目。
- `shadowed` 条目必须指向一个存在的 `active` 条目。

---

## 3. Skill 注册快照（Skill Registry Snapshot）

### 作用
表示某时刻的完整 Skill 注册结果，供路由与管理查询。

### 字段
- `version`: 快照版本号（单调递增）。
- `entries`: Skill 目录条目集合。
- `active_names`: 当前可激活 Skill 名称集合。
- `generated_at`: 快照生成时间。

### 约束
- 新快照生成后必须整体替换旧快照（原子切换）。
- 快照生成失败时不得污染当前可用快照。

---

## 4. 路由上下文（Routing Context）

### 作用
承载一次消息路由所需的输入上下文。

### 字段
- `conversation_id`
- `channel`
- `user_id`
- `text`
- `current_session_id`（可选）
- `available_skills`（来自当前快照）

### 约束
- `text` 为空时不触发 Skill 路由。
- `available_skills` 必须来自当前一致性快照。

---

## 5. 意图决策（Intent Decision）

### 作用
表示一次路由判定结果，驱动后续控制流或执行流。

### 字段
- `kind`: `control` / `skill` / `task`
- `skill_name`: 当 `kind=skill` 时必填。
- `reason`: 判定原因（如 `explicit_skill`、`alias_match`、`llm_fallback`）。
- `confidence`: 置信度（仅 LLM 路径）。
- `decided_at`: 判定时间。

### 约束
- `kind=skill` 时必须包含 `skill_name`。
- `kind=control` 时不得附带 `skill_name`。
- 低置信度 LLM 判定必须回退为 `task`。
- 低置信度判断使用可配置阈值，未配置时采用默认值 `0.72`。

---

## 6. Skill 授权策略（Skill Permission Policy）

### 作用
定义 Skill 可用性与访问范围治理规则。

### 字段
- `enabled`: Skill 总开关。
- `disabled_names`: 禁用 Skill 名称集合。
- `allow_users`: 用户白名单。
- `allow_channels`: 频道白名单。
- `default_mode`: 默认模式（`channel_allowlist_dm_pairing`）。

### 约束
- 命中 `disabled_names` 的 Skill 一律不可执行。
- 未命中授权条件时必须拒绝执行并返回可理解错误。

---

## 7. DM Pairing 记录（DM Pairing Record）

### 作用
表示 DM 配对授权生命周期状态，用于控制默认“DM pairing”策略下的 Skill 可用性。

### 字段
- `pairing_id`
- `channel`
- `user_id`
- `state`: `pending` / `paired` / `expired` / `revoked`
- `created_at`
- `updated_at`
- `expires_at`（可选）
- `revoked_reason`（可选）

### 约束
- `expired` 与 `revoked` 状态不得继续授权 Skill 调用。
- `paired` 状态必须可追溯到一次成功配对事件。

---

## 8. 路由审计记录（Intent Audit Record）

### 作用
提供路由可观测性与问题追踪证据。

### 字段
- `conversation_id`
- `session_id`
- `channel`
- `agent_id`
- `intent_kind`
- `intent_reason`
- `intent_skill_name`
- `intent_confidence`
- `duration_ms`
- `timestamp`

### 约束
- 每次有效入站请求必须至少产出一条审计记录。
- 审计记录不得包含敏感凭据或明文密钥。
