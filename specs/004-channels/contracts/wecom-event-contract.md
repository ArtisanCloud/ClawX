# 契约：WeCom 回调验证与消息处理

## 目的
定义 WeCom URL 验证、签名校验、消息解密、重放保护与统一路由。

## 1. 路由契约
- Path：`/webhooks/wecom/<instance-id>`
- Method：`GET`（URL 验证）或 `POST`（消息事件）
- Query：`msg_signature`、`timestamp`、`nonce`

## 2. URL 验证契约（GET）
- 输入：`echostr`（加密）
- 校验：签名 + 时间窗 + AES 解密 + CorpID 匹配
- 响应：`200` 且正文为解密后的明文 `echostr`

## 3. 消息事件契约（POST）
- Body：XML，包含 `<Encrypt>`
- 校验顺序：
  1. `msg_signature` 校验
  2. `timestamp` 时间窗校验（15 分钟）
  3. AES-CBC 解密 + PKCS7 去填充
  4. CorpID 匹配
- 仅文本消息进入执行链路，非文本事件返回成功接收。

## 4. 签名与解密
- 签名：`sha1(sort(token,timestamp,nonce,encrypt).join(""))`
- 解密：`EncodingAESKey`（43 字符 base64 key）
- 解密失败、CorpID 不匹配、重放请求均返回拒绝。

## 5. 归一化契约
最小映射：
- `channel=wecom`
- `user_id`: `FromUserName`
- `conversation_id`: direct=`wecom:-:-:<user>`；群聊=`wecom:<chat_id>:-:<user>`
- `window_id`: `compat:<conversation_id>`
- `text`: `Content`

## 6. 响应与错误语义
- URL 验证成功：`200` + 明文 `echostr`
- POST 成功接收：`200` + `success`
- Method 非法：`405`
- 签名/解密/重放失败：`403`
- 载荷解析失败：`400`

## 7. 去重约束
- 幂等键：`channel|instance|event_id`
- event_id 缺失时可回退 hash（timestamp+nonce+encrypt）
