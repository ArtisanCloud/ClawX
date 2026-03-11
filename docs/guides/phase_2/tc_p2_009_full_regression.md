# TC-P2-009 全量回归

## 目标
- 验证第二阶段改动对仓库全量测试无回归。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./... -count=1
```

## 预期
1. `unit`、`integration`、`contract` 全量通过。
