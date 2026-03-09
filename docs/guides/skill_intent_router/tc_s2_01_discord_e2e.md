# TC-S2-01 Discord 端到端联调

## 目标
- 在 Discord 中完整验证 Skill Registry + Intent Router。

## 前置
1. Discord Bot 已连通，服务日志出现 `discord gateway ready`。
2. `echo` Skill 已加载为 `active`。

## 步骤
1. 私聊 Bot：`/new`
2. 发送：`/sx-skills`
3. 发送：`/sx-skill echo 请返回 discord-skill-ok`
4. 发送：`echo discord-exact-ok`
5. 发送：`repeat discord-alias-ok`
6. 发送：`请做一件和 skill 无关的普通任务`
7. 发送：`/cancel`

## 预期
1. 第 1/7 步均为控制命令响应。
2. 第 2 步返回 SynapseX Skill 列表。
3. 第 3/4/5 步命中 Skill 路径，返回可识别输出。
4. 第 6 步回退普通任务路径。
5. Skill 路径回复前缀为 `[SynapseX Skill]`，普通任务前缀为 `[Agent Direct]`。
4. 日志可追踪 `intent.kind/reason/skill/confidence`。
