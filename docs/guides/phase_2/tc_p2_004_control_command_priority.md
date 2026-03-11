# TC-P2-004 内建控制命令优先级

## 目标
- 验证内建控制命令优先于技能/自然语言路由。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionCommandPriorityBuiltInControlAlwaysWins -count=1
```

## 预期
1. 测试通过。
2. `/new`、`/resume`、`/switch`、`/list`、`/current`、`/cancel` 均路由到 control 分支。
