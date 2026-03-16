# 契约：项目命令语义

## 目的
定义 `/project` 控制命令的语法、状态变更和错误边界，保证多渠道行为一致。

## 1. 命令集合

- `/project create <project_id> [name...]`
- `/project list`
- `/project use <project_id>`
- `/project current`
- `/project bind <route_key> <project_id>`
- `/project unbind <route_key>`
- `/project audit`
- `/project delete <project_id> [--force]`
- `/project repair <project_id>`
- `/project suggest <project_id> [confidence] [reason...]`
- `/project confirm <proposal_id>`

## 2. 语法契约

- `project_id` 归一化后仅允许：`a-z`、`0-9`、`-`、`_`。
- `route_key` 必须为非空字符串，建议使用归一化消息中的 `route_key`。
- `proposal_id` 必须为非空字符串。
- `confidence` 范围为 `[0,1]`，超出范围按边界值截断。

## 3. 行为契约

- `create`
  - 创建项目元数据并确保 workspace 目录存在。
  - 不改动任意 route 绑定。
- `list`
  - 返回全部项目及状态（`active/inactive/broken`）与 workspace 路径。
- `use`
  - 将“当前 route_key”绑定到目标项目。
  - 后续 `/new` 与执行请求都应落在该项目作用域。
- `current`
  - 返回当前 route_key 命中的项目、状态、workspace、命中模式（`binding/fallback`）。
- `bind/unbind`
  - `bind` 对任意 route_key 建立或覆盖绑定。
  - `unbind` 删除 route_key 绑定，后续回退到默认项目。
- `audit`
  - 汇总项目数量、绑定数量、`broken` 项目数量、异常绑定数量与异常明细。
- `delete`
  - 默认执行安全删除：
    - 若存在活动绑定，拒绝删除；
    - 若存在活动会话，拒绝删除；
    - 不允许删除默认项目。
  - `--force` 可跳过绑定检查并清理该项目绑定。
- `repair`
  - 当项目 workspace 缺失导致 `broken` 时，重建目录并恢复为 `active`。
- `suggest/confirm`
  - `suggest` 只生成 proposal，不切换绑定。
  - 同一 `route_key` 在冷却窗口内重复触发建议时，应命中限流策略，避免提示轰炸。
  - `confirm` 仅对有效且未过期 proposal 生效，成功后更新 route 绑定。

## 4. 错误契约

以下场景必须返回可追踪错误，不得静默吞错：

- 参数缺失或非法（命令解析失败）。
- 目标项目不存在或处于不可用状态。
- proposal 不存在、已过期、或状态非法。
- 删除命令命中保护条件（活动绑定/活动会话/默认项目）。
- 持久化读写失败（registry/binding/proposal）。

## 5. 一致性要求

- 控制命令优先级高于技能与自然语言路由。
- 所有渠道（Telegram/Discord/Feishu/WeCom）执行语义一致。
- 输出文案允许渠道适配差异，但状态变更必须一致。

## 6. 控制命令回归矩阵（项目作用域）

- `/new`：仅新建当前项目会话，不改 route 绑定。
- `/resume`：仅恢复当前项目作用域会话。
- `/switch`：仅切换当前项目作用域会话。
- `/list`：仅列出当前项目作用域会话。
- `/current`：仅返回当前项目作用域会话。
- `/cancel`：仅取消当前项目作用域会话。
