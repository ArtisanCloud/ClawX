# 契约：多渠道统一控制命令语义

## 目的
定义 Discord、Telegram、Feishu、WeCom 四个渠道下控制命令的统一行为，避免渠道差异导致会话管理不一致。

## 1. 命令集合

必须支持的控制命令：

```text
/new
/resume <session_id>
/list
/current
/switch <session_id>
/cancel
```

## 2. 统一输入

渠道进入控制流前必须归一化为：

```text
channel
instance_id
conversation_id
user_id
window_id
command
args
```

## 3. 统一输出语义

- `/new`: 创建新会话并绑定当前窗口。
- `/resume <session_id>`: 恢复目标会话并绑定当前窗口。
- `/list`: 返回当前窗口可见会话列表并标记当前会话。
- `/current`: 返回当前窗口当前会话信息；若无会话返回引导提示。
- `/switch <session_id>`: 仅切换当前窗口绑定，不触发执行。
- `/cancel`: 取消当前会话正在运行的任务（若存在）。

## 4. 错误语义

至少区分以下类别：

```text
invalid_command
session_not_found
session_not_visible
no_active_session
session_busy
permission_denied
```

要求：
- 错误提示对最终用户可理解。
- 统一错误类别映射，不因渠道变化而变化。

## 5. 一致性要求

- 控制命令优先级必须高于 Skill/Task 路径。
- 同一输入语义在不同渠道必须得到同类输出。
- 渠道特有字段不得改变控制语义，只可扩展 metadata。
