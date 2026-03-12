# WeCom Bot 接入（Phase 5）

## 目标
- 完成 WeCom 回调 URL 验证（`echostr`）。
- 完成签名校验 + 消息解密后接入统一控制命令语义（`/new`、`/resume`、`/switch`、`/current`、`/list`、`/cancel`）。

## 1. 前置准备
1. 在企业微信后台创建应用，记录：
   - `corpId`
   - `agentId`
   - `secret`
   - `token`
   - `encodingAesKey`
2. 准备公网 HTTPS 回调入口。

## 2. 配置 SynapseX
推荐交互式增量配置：

```bash
go run ./cmd/synapsex config channel wecom
```

也可用 `config set`：

```bash
go run ./cmd/synapsex config set channels.wecom.enabled true
go run ./cmd/synapsex config set channels.wecom.mode webhook
go run ./cmd/synapsex config set channels.wecom.corpId <CORP_ID>
go run ./cmd/synapsex config set channels.wecom.agentId <AGENT_ID>
go run ./cmd/synapsex config set channels.wecom.secret <SECRET>
go run ./cmd/synapsex config set channels.wecom.token <TOKEN>
go run ./cmd/synapsex config set channels.wecom.encodingAesKey <ENCODING_AES_KEY>
```

默认单实例路由：`/webhooks/wecom/wecom-default`。

## 3. 启动服务

```bash
go run ./cmd/synapsex serve
```

预期日志：
- `wecom webhook route registered: instance=... path=/webhooks/wecom/...`

## 4. 企业微信后台回调配置
在应用回调配置填写：
- URL: `https://<your-domain>/webhooks/wecom/<instance-id>`
- Token: 与 `channels.wecom.token` 一致
- EncodingAESKey: 与 `channels.wecom.encodingAesKey` 一致

保存时会触发 URL 验证，服务端会返回解密后的 `echostr`。

## 5. 验证步骤
1. 发送 `/current`，首次应提示当前无会话。
2. 连续发送两次 `/new`，记录 `sessA`、`sessB`。
3. 发送 `/list`，预期当前会话为 `sessB`。
4. 发送 `/switch <sessA>` 后 `/current`，预期为 `sessA`。
5. 发送 `/resume <sessB>` 后 `/current`，预期为 `sessB`。
6. 发送 `/cancel`，空闲会话应返回已处理（noop）语义。

## 6. 常见问题
- `403 Forbidden`: `msg_signature`、`token` 或 `encodingAesKey` 不匹配。
- URL 验证失败：回调地址不可达，或路由与实例 ID 不一致。
- 无响应：服务未启动、端口未转发、或反向代理未放行回调路径。
