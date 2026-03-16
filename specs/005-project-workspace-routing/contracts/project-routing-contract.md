# 契约：项目路由与会话隔离

## 目的
定义 `route_key -> project -> session` 的判定顺序与隔离规则，确保同一 bot 并发多项目不串线。

## 1. 路由键契约

消息归一化后必须生成稳定 `route_key`：

```text
<channel>:<instance>:<peer_kind>:<peer_id>[:thread:<thread_id>]
```

约束：

- `thread_id` 可用时必须纳入 `route_key`。
- 同一来源上下文重复请求必须得到相同 `route_key`。
- `route_key` 缺失时，兼容回退为 `compat:<conversation_id>`。

## 2. 项目判定顺序

对每条入站消息，项目判定必须按顺序执行：

1. 查询 `bindings.json`：若 `route_key` 有绑定，命中绑定项目。
2. 若无绑定，回退到默认项目（缺省 `main`）。
3. 若命中项目状态为 `broken`，应返回错误并提供修复路径。

## 3. 会话作用域契约

- 会话 ID 必须包含 `project_id` 维度：`sess-<project_id>-<nanos>`。
- window 绑定键必须包含项目作用域：
  - `<window_id>|project:<project_id>`
- `/new` 仅在“当前项目作用域窗口”创建会话，不得改写 route 绑定。
- `continue/resume/switch/list/current/cancel` 仅操作当前项目作用域会话。

## 4. 控制命令与项目关系

- `/project use`：更新当前 route 绑定，不创建会话。
- `/project bind/unbind`：运维修复 route 绑定关系。
- `/project current`：返回当前 route 命中项目与模式（binding/fallback）。
- `/project suggest`：仅创建 proposal，必须显式确认后才切换。
- `/project confirm`：确认后更新绑定并写入审计日志。

## 5. 删除与恢复契约

- `/project delete` 默认必须通过安全检查：
  - 不允许删除默认项目；
  - 不允许删除有活动绑定项目；
  - 不允许删除有活动会话项目。
- `/project repair` 必须可从缺失 workspace 恢复 `broken` 项目到 `active`。

## 6. 可观测性契约

执行与路由日志至少包含：

- `conversation_id`
- `route_key`
- `project_id`
- `project_mode`（`binding` 或 `fallback`）
- `session_id`（若存在）

审计汇总必须可导出：

- 项目总数、活跃/损坏项目数
- 绑定总数、异常绑定数
- 异常明细（route_key/project_id/reason）
