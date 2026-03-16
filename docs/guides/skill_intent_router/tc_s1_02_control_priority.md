# TC-S1-02 控制命令优先级不回归

## 目标
- 验证引入 Skill 路由后，控制命令语义保持不变。

## 步骤
1. 启动服务：`go run ./cmd/clawx`
2. 在渠道发送：`/new`
3. 发送：`/list`
4. 发送：`/cancel`
5. 再测裸命令：`new`、`list`、`cancel`

## 预期
1. 全部命中控制路径。
2. 不出现被 Skill 路由抢占的情况。
3. 日志中可见 `intent.kind=control`（或路由判定为 control）。
