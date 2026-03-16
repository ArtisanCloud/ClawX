# TC-P2-002 同窗口 list/current/switch

## 目标
- 验证同窗口创建多个会话后，`/list`、`/current`、`/switch` 语义正确。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionControlFlowListCurrentSwitch -count=1
```

## 预期
1. 测试通过。
2. 切换前后当前会话标记正确，切换后继续输入命中新目标会话。
