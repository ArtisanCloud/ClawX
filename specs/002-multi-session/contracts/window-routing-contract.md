# 契约：窗口优先路由

## 目的
定义第二阶段中 `window_id` 如何参与请求路由，以及缺省兼容行为。

## 1. 统一入站消息字段

入站消息在进入 Router 前必须具备：

```text
conversation_id
window_id
user_id
text
reply_to (可选)
attachments (可选)
channel
context_flags
```

### 字段约束
- `window_id` 支持调用方显式传入。
- 若未传入，归一化层必须生成：`compat:<conversation_id>`。
- 归一化层不得直接持有窗口状态，仅负责提供稳定键。
- 当 `conversation_id` 无法构建时，归一化层必须返回可见错误，不得构造空窗口键。

## 2. 路由优先级

Router 必须按以下顺序决策目标会话：
1. 控制命令显式指定的会话
2. `window_id` 当前绑定会话
3. `conversation_id` 最近会话（兼容回退）
4. 新建会话

## 3. 绑定刷新触发

以下动作成功后必须刷新窗口绑定：
- `new`
- `resume`
- `switch`
- 普通消息执行成功

## 4. 渠道映射约束

- Telegram：
  - 调用方可透传 `window_id`；缺省时由归一化层生成 `compat:<conversation_id>`。
  - `conversation_id` 由 `channel/guild/thread/user` 统一构造。
- Discord：
  - 文本消息与 Slash Command 统一走同一归一化逻辑。
  - Slash Command 与普通消息必须共享同一 `window_id` 判定规则。
- 渠道适配层只负责输入归一化，不得自行缓存窗口绑定状态。

## 5. 失败行为

- 若窗口绑定指向不存在会话：返回可见错误并回退到兼容流程（按策略可新建）。
- 若目标会话不可见或不属于当前上下文：返回拒绝错误，不得静默改绑。
- 若消息上下文不完整（如缺失可构建 `conversation_id` 的关键字段）：返回可见错误并终止路由。

## 6. 可观测性要求

- 路由日志必须至少包含：`conversation_id`、`window_id`、`decision.kind`、`session_id`（若存在）。
- 回归验证需覆盖：
  - 显式窗口路由
  - compat 回退路由
  - 控制命令优先于技能/自然语言路径
