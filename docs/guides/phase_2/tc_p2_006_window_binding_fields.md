# TC-P2-006 窗口绑定字段持久化与查询

## 目标
- 验证窗口绑定关键字段持久化和查询一致：
  - `window_id`
  - `current_session_id`
  - `conversation_id`
  - `updated_at`
  - `last_used_at`

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 分支为 `002-multi-session`。

## 步骤
1. 执行：

```bash
go test ./tests/integration -run TestMultiSessionBindingFieldsPersistenceAndQuery -count=1
```

## 预期
1. 测试通过。
2. 窗口绑定记录可查询，时间戳在继续执行后前进。
