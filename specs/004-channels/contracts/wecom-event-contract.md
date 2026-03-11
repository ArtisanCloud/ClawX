# 契约：WeCom 回调验证与消息处理

## 目的
定义 WeCom 回调 URL 验证、签名校验、消息解密与归一化要求。

## 1. URL 验证

首次接入必须支持 URL 验证流程：
- 校验 `msg_signature`、`timestamp`、`nonce`。
- 解密 `echostr` 并返回明文。
- 失败时返回拒绝响应。

## 2. 事件消息处理

要求支持：
- XML/JSON 载荷解析（按接入协议）
- 签名校验
- 加密消息解密（AES key/CorpID）

拒绝条件：
- 签名不合法
- 解密失败
- 必填字段缺失

## 3. 文本消息归一化

最小映射：

```text
conversation_id <- ExternalUserID or ChatId fallback
user_id         <- FromUserName
window_id       <- wecom:<conversation_id>:<user_id>
text            <- Content
channel         <- wecom
instance_id     <- configured id
```

## 4. 去重要求

- 事件唯一标识可用时必须构造 `channel|instance|event_id` 幂等键。
- 幂等窗口内重复事件必须被标记为 duplicate 并跳过执行。

## 5. 运行隔离

- WeCom 适配器错误只影响本适配器。
- 主进程与其他渠道适配器必须继续运行。
