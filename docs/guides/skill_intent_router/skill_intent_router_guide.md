# Skill Registry + Intent Router 主线联调指南

## 目标
- 验证 Skill 能被发现、刷新、启停，并可进入路由。
- 验证消息会稳定分流到：控制命令、Skill 路径、普通任务路径。
- 验证默认治理策略：权限拒绝、禁用拒绝、DM pairing 生命周期。

## 前置
1. Go 1.23.x
2. `go test ./...` 可通过
3. 已有可运行配置文件：`~/.clawx/config.json`
4. 如需渠道联调，已完成 Discord/Telegram Bot 基础接入

## Step 1: 本地编译与测试
```bash
go test ./...
```

## Step 2: Starter Skill 说明
首次启动会自动创建：

- `~/.clawx/skills/echo/SKILL.md`

如果你想覆盖为自己的内容，再手动改这个文件。示例：

```md
---
name: echo
description: 回显输入
aliases:
  - repeat
---
请回显用户输入。
```

## Step 3: 校验 Registry CLI
```bash
go run ./cmd/clawx skill list
go run ./cmd/clawx skill reload
go run ./cmd/clawx skill disable echo
go run ./cmd/clawx skill enable echo
```

预期：
1. `skill list` 能看到 `echo`，状态可观测。
2. `disable/enable` 后 `list` 状态随之变化。

## Step 4: 启动服务
```bash
go run ./cmd/clawx
```

关注日志字段：
- `intent.kind`
- `intent.reason`
- `intent.skill`
- `intent.confidence`

## Step 5: 渠道联调最小路径
在 Discord 或 Telegram 中发送：
1. `/new`
2. `list`
3. `/clawx-skills`（查看 ClawX 技能目录）
4. `/sx-skill echo 请返回: hello-skill`
5. `echo hello-exact`
6. `repeat hello-alias`

## 结果判定
- 控制命令仍走控制路径（不被 Skill 抢占）。
- 显式 Skill 请求命中 Skill 路径。
- 非命中请求回退普通任务路径。
- 拒绝场景有明确错误信息（非静默失败）。
- 回包来源可区分：
  - `[ClawX Skill]`：由 ClawX Skill 路径执行
  - `[Agent Direct]`：普通执行路径（由执行器自行决定内部技能）

## 对应用例
- `tc_s1_01_skill_registry_list_reload.md`
- `tc_s1_02_control_priority.md`
- `tc_s1_03_skill_route_explicit_and_rule.md`
- `tc_s1_04_threshold_fallback.md`
- `tc_s1_05_permission_and_disable.md`
- `tc_s1_06_dm_pairing_lifecycle.md`
- `tc_s2_01_discord_e2e.md`
