# TC-P2-003 兼容路径（无 window_id）回退

## 目标
- 验证未显式提供 `window_id` 时，系统仍按 compat 路径可继续会话。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionCompatFallbackWithoutExplicitWindowID -count=1
```

## 预期
1. 测试通过。
2. 兼容窗口键稳定，第二次请求继续命中第一次会话。
