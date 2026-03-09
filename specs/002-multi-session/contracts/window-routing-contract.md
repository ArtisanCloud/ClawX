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

## 4. 失败行为

- 若窗口绑定指向不存在会话：返回可见错误并回退到兼容流程（按策略可新建）。
- 若目标会话不可见或不属于当前上下文：返回拒绝错误，不得静默改绑。
