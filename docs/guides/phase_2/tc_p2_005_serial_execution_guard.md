# TC-P2-005 同一会话串行执行约束

## 目标
- 验证同一 `session` 的并发执行请求会被串行/拒绝，不会并发落后端。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionSerialExecutionRejectConcurrentSameSession -count=1
```

## 预期
1. 测试通过。
2. 第二个并发请求被拒绝，后端执行计数保持为 1。
