# ClawX Development Guidelines

Auto-generated from all feature plans. Last updated: 2026-03-03

## Active Technologies
- Go 1.23 + Go 标准库、`discordgo`、Telegram Bot SDK（Go）、`gopkg.in/yaml.v3` (001-phase1-foundation)
- 内存态会话存储 + 环境变量 / YAML 配置文件 (001-phase1-foundation)
- Go 1.23 + Go 标准库、现有 ClawX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化模块 (002-multi-session)
- 内存会话仓储（已存在）+ 计划新增窗口绑定持久化结构（内存优先，兼容后续文件/数据库扩展） (002-multi-session)
- Go 1.23 + Go 标准库、现有 ClawX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化组件 (003-skill-intent-router)
- 本地文件存储（`~/.clawx`）+ 现有会话存储；Skill 索引以文件缓存形式维护 (003-skill-intent-router)
- Go 1.23 + Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、`gopkg.in/yaml.v3`、现有 Discord/Telegram/Feishu/WeCom 适配层 (006-memory-context-layer)
- 本地文件存储（`~/.clawx/workspaces/<project_id>/` 与 `~/.clawx/workspaces/<project_id>/.agents/<agent_id>/`）+ 现有 `~/.clawx/projects/*.json` 持久化 (006-memory-context-layer)
- Go 1.23 + Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、现有 Router/Intent 管线、现有本地文件持久化组件 (007-unified-scheduler-center)
- 本地文件存储（`~/.clawx/workspaces/<project_id>/.agents/<agent_id>/scheduler/`） (007-unified-scheduler-center)
- Go 1.23 + Go 标准库、现有 `cmd/clawx/config_chat.go` 配置链路、现有 Router/Intent 管线、现有文件配置持久化模块 (009-config-intent-plan)
- 内存态 pending plan（按会话隔离）+ 本地配置文件写入（`~/.clawx/config.json`）+ 结构化日志审计 (009-config-intent-plan)

- (001-phase1-foundation)

## Project Structure

```text
src/
tests/
```

## Commands

# Add commands for 

## Code Style

: Follow standard conventions

## Recent Changes
- 009-config-intent-plan: Added Go 1.23 + Go 标准库、现有 `cmd/clawx/config_chat.go` 配置链路、现有 Router/Intent 管线、现有文件配置持久化模块
- 007-unified-scheduler-center: Added Go 1.23 + Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、现有 Router/Intent 管线、现有本地文件持久化组件
- 006-memory-context-layer: Added Go 1.23 + Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、`gopkg.in/yaml.v3`、现有 Discord/Telegram/Feishu/WeCom 适配层


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->

## Requirement Sync

- [2026-03-22T07:01:17Z] agent=main source=runtime_cwd requirement=构建一个招标信息助手系统：1) 定期抓取用户关注范围内的线上招标公告；2) 对抓取结果进行清洗、去重与汇总，生成可读的汇报通知；3) 将通知自动发送到企业微信、钉钉等办公通讯渠道；4) 通知内支持点击进入公告详情页查看完整招标内容；5) 支持用户按公司名称主动查询并返回该公司的相关招标项目公告；6) 整体目标是形成“定时订阅 + 主动检索”一体化能力。
- [2026-03-22T14:27:33Z] agent=main source=runtime_cwd requirement=项目目标是建设一个“招标信息助手”系统：支持按用户关注范围定期抓取线上招标公告，进行汇总并生成通知；通知可自动发送到企业微信、钉钉等办公通讯工具，并提供可点击的详情入口查看完整招标内容；同时支持用户按公司名称主动查询并返回该公司的相关招标项目信息。整体形态为“定时订阅推送 + 主动查询检索”的一体化能力。
- [2026-03-22T14:48:03Z] agent=main source=runtime_cwd requirement=建设一个招标信息助手：可按用户关注范围定期抓取线上招标公告，自动汇总为汇报通知，并发送到企业微信、钉钉等办公通讯工具；通知中提供可点击的详情入口查看完整招标内容；同时支持用户按公司名称主动查询并返回该公司的相关招标项目信息，形成“定时推送 + 主动检索”的一体化能力。
