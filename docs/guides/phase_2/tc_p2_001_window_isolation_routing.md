# TC-P2-001 多窗口隔离路由

## 目标
- 验证窗口 A/B 各自继续原会话，不发生串线。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionWindowRoutingWindowIsolationParallelContinue -count=1
```

## 预期
1. 测试通过。
2. 断言窗口 A 继续 A 会话、窗口 B 继续 B 会话，且两者不同。
