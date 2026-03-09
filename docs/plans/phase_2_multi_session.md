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

## 与宪章核心命令集合的关系
- 宪章定义的核心集合（`/new`、`/resume`、`/list`、`/cancel`）保持不变。
- Phase 2 在不破坏核心集合语义的前提下新增 `/current` 与 `/switch` 作为窗口模型增强命令。
- 新增命令仍由 Core Router 优先解析，不进入 Skill 路由覆盖范围。
