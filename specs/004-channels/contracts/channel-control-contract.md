# 契约：多渠道统一控制命令语义

## 目的
定义 Discord、Telegram、Feishu、WeCom 的控制命令输入/输出和错误语义，保证跨渠道一致。

## 1. 命令集合
必须支持：
- `/new`
- `/resume <session_id>`
- `/switch <session_id>`
- `/list`
- `/current`
- `/cancel`

兼容要求：
- 对 Telegram/Discord，裸命令（如 `new`）应与带斜杠语义一致。
- 对 Feishu/WeCom，按文本命令处理，不引入渠道特有命令分叉。

## 2. 路由优先级契约
- 控制命令优先级必须高于 Skill/Task 意图路由。
- `/switch` 与 `/resume` 必须要求显式 `session_id`。
- `/switch` 仅切换窗口绑定，不触发执行。

## 3. 统一输入模型
控制流处理前，必须提供以下字段：
- `channel`
- `instance_id`
- `conversation_id`
- `window_id`
- `user_id`
- `command`（原始文本）

## 4. 统一输出语义
- `/new`：创建并绑定新会话。
- `/resume`：恢复指定会话并绑定窗口。
- `/switch`：切换当前窗口会话指针。
- `/list`：返回窗口可见会话列表并标记当前会话。
- `/current`：返回当前窗口当前会话；无会话返回引导。
- `/cancel`：取消当前会话执行；空闲会话返回 noop 语义。

## 5. 错误类别契约
至少覆盖：
- `invalid_command`
- `session_not_found`
- `session_not_visible`
- `no_active_session`
- `session_busy`
- `permission_denied`

要求：
- 用户侧文案可理解。
- 同类错误在四渠道输出同类语义，不因渠道变化。

## 6. 可测试性契约
跨渠道一致性回归至少验证：
- 6 个控制命令的成功路径。
- 缺参与会话不存在错误路径。
- 控制命令不触发后端执行（特别是 `/switch`、`/list`、`/current`）。
