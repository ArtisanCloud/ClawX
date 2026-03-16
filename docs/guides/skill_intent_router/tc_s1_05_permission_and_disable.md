# TC-S1-05 权限拒绝与 Skill 禁用

## 目标
- 验证未授权请求会被拒绝，禁用 Skill 不可执行。

## 步骤
1. 执行：`go run ./cmd/clawx skill disable echo`
2. 在渠道发送：`skill echo test-disabled`
3. 执行：`go run ./cmd/clawx skill enable echo`
4. 配置一个未授权用户或频道后，再发送：`skill echo test-permission`

## 预期
1. 第 2 步返回“Skill 已禁用”语义错误。
2. 第 4 步返回“权限不足”语义错误。
3. 不会静默失败，不会降级成隐式 Skill 执行。
