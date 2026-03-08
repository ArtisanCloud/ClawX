# TC-L1-02 健康检查

## 目标
- 确认服务常驻与健康探针。

## 步骤
1. 保持服务运行。
2. 另开终端执行：`curl -s http://127.0.0.1:8080/healthz`

## 预期
1. 返回 JSON。
2. `Status` 为 `healthy` 或 `degraded`。
