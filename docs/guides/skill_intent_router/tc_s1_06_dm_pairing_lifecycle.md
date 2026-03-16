# TC-S1-06 DM pairing 生命周期

## 目标
- 验证 `pending/paired/expired/revoked` 生命周期行为与拒绝语义。

## 步骤
1. 确认服务启动后生成 pairing 存储文件（位于 `~/.clawx/state/`）。
2. 让目标用户进入 `paired` 状态后，在 DM 触发 Skill。
3. 将该 pairing 标记为 `expired` 或 `revoked`。
4. 再次在 DM 触发同一 Skill。

## 预期
1. `paired` 状态可执行。
2. `expired/revoked` 状态被拒绝。
3. 返回错误语义为 `pairing_expired`（或同义用户提示）。
