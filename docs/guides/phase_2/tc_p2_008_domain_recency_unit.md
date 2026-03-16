# TC-P2-008 领域绑定与 recency 单元验证

## 目标
- 验证窗口绑定独立性、conversation 回退与最近使用时间刷新。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/unit -run 'TestMultiSessionDomainWindowIndependentBinding|TestMultiSessionDomainContinueFallsBackToConversationAndCreatesBinding|TestMultiSessionRecencyRefreshForNewResumeSwitchAndExecuteSuccess' -count=1
```

## 预期
1. 测试通过。
2. 不同窗口绑定互不影响，回退路径正确，recency 在规定动作后刷新。
