# TC-P2-007 控制命令契约（switch/resume）

## 目标
- 验证 `/switch` 与 `/resume` 在契约层的参数、语义与错误行为。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/contract -run 'TestMultiSessionControlContractSwitchSemantics|TestMultiSessionControlContractParseSwitchAndResume' -count=1
```

## 预期
1. 测试通过。
2. `/switch` 仅切换不执行；缺参或会话不可见返回明确错误。
