# TC-L1-03 main agent 首次对话

## 目标
- 在 `healthz` 通过后，验证 `main agent` 可在 Discord 私聊里完成一次完整对话链路。

## 前置
1. 已完成 `tc_l1_02_health_check.md`。
2. 服务仍在运行。
3. Discord 网关日志已出现 `discord gateway ready`。

## 步骤
1. 在 Discord 私聊 Bot 发送：`/new`
2. 继续发送：`请先执行 pwd，并返回当前工作目录绝对路径`
3. 按第 2 步返回的目录继续发送二选一指令：
   - 若目录是项目根（例如 `/home/ubuntu/workspace/SynapseX`）：`请读取 docs/guides/phase_1/README.md 第一行并原样返回`
   - 若目录不是项目根：`请列出当前目录前 5 个文件名`
4. 观察服务端日志是否出现 `discord execute begin` 与 `discord execute done`

## 预期
1. 第 1 条消息返回新会话创建成功（包含 `session_id`）。
2. 第 2 条消息返回可识别的绝对路径（表示执行已进入 agent workspace）。
3. 第 3 步消息收到正常文本回包，不出现 `The application did not respond`。
4. 日志里出现 `discord execute begin: ... agent=main ... profile_kind=codex-cli ... profile_command=codex ...`。
5. 日志里出现 `discord execute done: ... backend_session_id=<uuid> ... state=success ...`。

## 失败排查
1. 查看服务日志是否有 `discord inbound message`。
2. 查看服务日志是否有 `discord route begin`。
3. 查看是否出现 `discord execute failed`，若有，按错误文本处理（网络/鉴权/CLI 不可用）。
4. 如发送失败，检查是否出现 `send discord message:` 的网络错误。
5. 若你希望固定验证项目内文件读取，请先把 `main` workspace 设为项目根再重启服务。
