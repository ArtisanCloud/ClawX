# 契约：Feishu 事件接入与安全校验

## 目的
定义 Feishu challenge、事件验签、文本消息归一化与错误语义。

## 1. 路由契约
- Path：`/webhooks/feishu/<instance-id>`
- Method：`POST`
- Content-Type：JSON

## 2. challenge 契约
- 输入：`type=url_verification` 请求。
- 校验：`token` 必须等于实例 `verificationToken`。
- 响应：`200` 且 JSON `{"challenge":"<value>"}`。
- challenge 请求不得进入业务执行链路。

## 3. 签名契约
- Header：`X-Lark-Request-Timestamp`、`X-Lark-Request-Nonce`、`X-Lark-Signature`
- 计算：`sha256(appSecret, timestamp + nonce + rawBody)`
- 兼容：十六进制签名与 base64 签名
- 不通过时：`403`

## 4. 事件处理契约
- 仅处理 `im.message.receive_v1` 的文本消息。
- 非文本消息返回成功接收（`{"code":0}`）但不进入执行。
- Header token 可用时必须校验 `verificationToken`。

## 5. 归一化契约
最小映射：
- `channel=feishu`
- `conversation_id`: `feishu:<guild_or_dash>:-:<user_id>`
- `window_id`: `compat:<conversation_id>`
- `user_id`: sender open_id/user_id/union_id
- `text`: 解析 `content.text`

## 6. 响应与错误语义
- challenge 成功：`200` + `{"challenge":"..."}`
- 普通事件成功：`200` + `{"code":0}`
- Method 非法：`405`
- 签名/token 非法：`403`
- 解析失败：`400`
- 事件接收后下游执行失败：返回 `200`，错误写入日志。

## 7. 去重约束
- 幂等键：`channel|instance|event_id`。
- 重复事件在幂等窗口内应标记 duplicate 并跳过执行。
