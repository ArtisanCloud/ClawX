# Phase 2 - Multi Session

## 目标
- 让“多窗口 = 多 Session”成为默认交互模型。

## 范围
- `Window Context`
- 一个窗口绑定一个当前 Session
- Session 列表
- Session 切换
- 最近使用会话

## 不做
- 复杂 Agent 管理
- 入口级自动路由

## 验收标准
- 用户可同时打开多个窗口
- 每个窗口可持有不同 Session
- 切换窗口不会串线
