# Feishu Bot 接入（Phase 4）

## 目标
- 完成 Feishu webhook challenge 验证。
- 完成事件签名校验后接入统一控制命令语义（`/new`、`/resume`、`/switch`、`/current`、`/list`、`/cancel`）与普通文本链路。

## 1. 前置准备
1. 在飞书开放平台创建自建应用并开通机器人能力。
2. 记录以下配置项：
   - `appId`
   - `appSecret`
   - `verificationToken`
   - `encryptKey`（可选）
3. 准备公网 HTTPS 入口（例如 Nginx + 证书）。

## 2. 配置 ClawX
当前 Phase 4 的 Feishu 增量向导是骨架版本，先使用 `config set` 写入：

```bash
go run ./cmd/clawx config set channels.feishu.enabled true
go run ./cmd/clawx config set channels.feishu.mode webhook
go run ./cmd/clawx config set channels.feishu.appId <APP_ID>
go run ./cmd/clawx config set channels.feishu.appSecret <APP_SECRET>
go run ./cmd/clawx config set channels.feishu.verificationToken <VERIFY_TOKEN>
go run ./cmd/clawx config set channels.feishu.encryptKey <ENCRYPT_KEY>
```

说明：
- 单实例默认 webhook 路由为 `/webhooks/feishu/feishu-default`。
- 多实例路由为 `/webhooks/feishu/<instance-id>`。

## 3. 启动服务

```bash
go run ./cmd/clawx serve
```

预期日志包含：
- `feishu webhook route registered: instance=... path=/webhooks/feishu/...`

## 4. 飞书回调配置
在飞书开放平台事件订阅配置中：
1. 请求地址填写：`https://<your-domain>/webhooks/feishu/<instance-id>`。
2. 按 ClawX 配置填写 `verificationToken` 与签名相关配置。
3. 保存后完成 challenge 校验。

## 5. 验证步骤
1. 发送 `/current`，首次应提示当前无会话。
2. 连续发送两次 `/new`，记录 `sessA`、`sessB`。
3. 发送 `/list`，预期当前会话为 `sessB`。
4. 发送 `/switch <sessA>`，再发送 `/current`，预期为 `sessA`。
5. 发送 `/resume <sessB>`，再发送 `/current`，预期为 `sessB`。
6. 发送 `/cancel`，空闲会话应返回已处理（noop）语义。
7. 发送普通文本，例如 `hello`，预期进入执行链路。

预期：
- 控制命令语义与 Telegram/Discord 保持一致。
- 普通文本进入执行链路。

## 6. 常见问题
- `403 Forbidden`：签名或 verification token 不匹配。
- `400 Bad Request`：事件体结构不合法或缺失文本内容。
- challenge 失败：回调地址不可达，或路由与实例 ID 不一致。
