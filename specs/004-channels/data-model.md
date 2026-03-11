# 数据模型：第四阶段渠道扩展

## 1. 渠道实例（Channel Instance）

### 作用
描述一个渠道运行单元及其静态配置。

### 字段
- `channel`: `discord` / `telegram` / `feishu` / `wecom`
- `instance_id`: 实例唯一标识
- `enabled`: 是否启用
- `mode`: 渠道模式（Telegram 为 `polling|webhook`，其他渠道可留空或固定）
- `agent`: 绑定 agent 名称
- `credentials`: 渠道密钥集合（token/app_secret 等）
- `endpoint`: webhook 回调路径或地址配置
- `updated_at`

### 约束
- `channel + instance_id` 必须唯一。
- `enabled=true` 的实例必须满足各渠道最小凭据要求。

---

## 2. 入站事件（Inbound Event）

### 作用
承载渠道原始请求或拉取消息，用于验签、去重与归一化。

### 字段
- `channel`
- `instance_id`
- `event_id`
- `event_type`
- `received_at`
- `raw_payload`
- `signature`
- `request_meta`（method/path/headers/query）

### 约束
- `event_id` 缺失时应生成兼容键（例如 hash）。
- 验签失败事件不得进入业务路由。

---

## 3. 标准消息（Normalized Message）

### 作用
统一各渠道输入格式，供 Router 与 Session Manager 使用。

### 字段
- `channel`
- `instance_id`
- `conversation_id`
- `user_id`
- `window_id`
- `text`
- `timestamp`
- `metadata`

### 约束
- `text` 为空时不触发执行流。
- `window_id` 不可用时按渠道生成稳定兼容值。

---

## 4. 渠道运行状态（Channel Runtime State）

### 作用
跟踪渠道实例运行与退避重试状态。

### 字段
- `channel`
- `instance_id`
- `state`: `running` / `backoff` / `stopped`
- `retry_count`
- `last_error`
- `last_error_at`
- `next_retry_at`

### 约束
- `backoff` 状态必须包含 `next_retry_at`。
- `retry_count` 随连续失败单调递增，成功后归零。

---

## 5. 渠道事件幂等记录（Channel Event Dedup Record）

### 作用
避免重复投递导致重复执行。

### 字段
- `dedup_key`（`channel|instance_id|event_id`）
- `first_seen_at`
- `expires_at`
- `status`: `accepted` / `duplicate`

### 约束
- 同一 `dedup_key` 在有效期内最多一次 `accepted`。
- 过期后可重新接收同键事件。

---

## 6. 增量配置补丁（Channel Config Patch）

### 作用
表达一次 `config channel <name>` 的最小变更集。

### 字段
- `target_channel`
- `target_instance_id`
- `patch_fields`
- `applied_at`
- `applied_by`

### 约束
- `patch_fields` 只允许修改目标渠道命名空间内字段。
- 应用补丁后必须保留其他渠道配置原值。
