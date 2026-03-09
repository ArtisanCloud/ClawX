# 契约：Skill Registry 发现、状态与管理命令

## 目的
定义 Skill 的发现来源、合法性校验、冲突处理与管理命令行为，保证运行时可预测与可观测。

## 1. Skill 来源优先级

注册中心按如下顺序加载来源：

1. `~/.synapsex/skills`
2. `<workspace>/.synapsex/skills`
3. 内置技能来源

同名冲突时，上位来源胜出。

## 2. Skill 合法性要求

每个 Skill 必须满足：

- 根目录存在 `SKILL.md`
- frontmatter 包含 `name`
- frontmatter 包含 `description`

不满足者标记为 `invalid`，不得进入可执行集合。

## 3. 条目状态

Skill 条目状态集合：

- `active`
- `invalid`
- `disabled`
- `shadowed`

约束：

- 同名仅允许一个 `active`
- `shadowed` 必须可追溯到激活条目

## 4. 同名冲突处理

- 同名时按来源优先级保留一个 `active`。
- 其余条目标记 `shadowed` 并产生日志告警。
- 系统不得静默覆盖而无可观测记录。

## 5. 管理命令契约

### `skill list`
- 返回当前目录条目及状态。
- 至少包含：`name`、`status`、`source`。

### `skill reload`
- 触发重新扫描并原子替换快照。
- 对刷新后的新请求立即生效。

### `skill enable <name>`
- 将目标 Skill 从禁用状态恢复为可用（若其校验通过且未被 shadowed）。

### `skill disable <name>`
- 将目标 Skill 标记为不可执行。
- 被禁用 Skill 请求必须返回显式提示。

## 6. 权限契约

默认模式为“频道白名单 + DM pairing”。

- 未授权用户或频道请求 Skill 时必须拒绝。
- 拒绝必须返回用户可理解错误。
- 禁止未授权请求隐式降级成 Skill 执行。

## 7. DM Pairing 生命周期

- Pairing 必须支持明确状态：`pending`、`paired`、`expired`、`revoked`。
- Pairing 失效（`expired`/`revoked`）后，Skill 调用必须被拒绝并提示重新配对。
- Pairing 生命周期事件必须可审计（创建、续期、失效、撤销）。

## 8. 索引一致性

- 刷新成功后整体替换索引快照。
- 刷新失败时保留旧快照。
- 任一时刻路由仅使用一个一致性快照。
