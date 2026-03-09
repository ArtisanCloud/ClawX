# 测试与验收

## 单元测试
1. `SKILL.md` 解析成功/失败。
2. 缺少 `name` 或 `description` 的拒绝逻辑。
3. 重名与保留名冲突校验。
4. Router 优先级顺序测试。
5. 权限拒绝路径测试。

## 集成测试
1. 运行中 `skill reload` 后生效。
2. Discord 文本与 slash 下 `/skill` 可用。
3. Telegram 文本命令触发技能成功。
4. 未命中技能时回退普通任务。

## 验收用例
1. 导入一个标准 Claude Skill，本地 `skill list` 可见。
2. 发送“使用 <skill-name> ...”命中并返回结果。
3. 禁用该 skill 后再次发送，返回禁用提示。
4. 调用 `/current` `/list` `/resume` 不受 Skill 影响。
5. 日志中可追踪完整路由决策字段。

## 通过标准
- 功能可用、日志可追踪、命令不回归、跨频道行为一致。
