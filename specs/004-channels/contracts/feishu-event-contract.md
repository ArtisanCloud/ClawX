# 契约：Feishu 事件接入与安全校验

## 目的
定义 Feishu 回调协议、challenge 验证、安全校验与消息归一化要求。

## 1. challenge 验证

当收到 Feishu challenge 请求时：
- 必须识别 challenge 类型请求。
- 必须返回协议要求的 challenge 值。
- 不进入业务路由。

## 2. 安全校验

必须支持：
- 时间戳与签名校验（按配置 app secret）
- 非法签名拒绝

建议支持：
- 时间窗校验，拒绝过期请求

## 3. 文本消息归一化

最小映射：

```text
conversation_id <- chat_id or open_chat_id
user_id         <- sender_id
window_id       <- feishu:<conversation_id>:<user_id>
text            <- text content
channel         <- feishu
instance_id     <- configured id
```

## 4. 去重与重放

- 事件 ID 可用时必须参与幂等键生成。
- 同一事件重复投递不得重复执行业务。

## 5. 错误处理

- 验签失败：HTTP 拒绝 + 安全日志
- 解析失败：HTTP 拒绝 + 结构化日志
- 下游执行失败：返回成功接收（避免无限重投）并记录执行错误
