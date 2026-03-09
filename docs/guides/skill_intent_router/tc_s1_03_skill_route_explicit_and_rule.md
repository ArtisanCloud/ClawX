# TC-S1-03 Skill 显式与规则路由

## 目标
- 验证显式调用与规则匹配都能进入 Skill 路径。

## 前置
1. `echo` Skill 已是 `active`。
2. `aliases` 包含 `repeat`。

## 步骤
1. 在渠道发送：`/sx-skill echo 请回显 hello-explicit`
2. 在渠道发送：`echo hello-exact`
3. 在渠道发送：`repeat hello-alias`

## 预期
1. 三条请求都进入 Skill 路径。
2. 日志中可看到：
   - 显式路径：`intent.reason=explicit_skill`
   - 精确匹配：`intent.reason=exact_name`
   - 别名匹配：`intent.reason=alias_match`
