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
- 006-memory-context-layer: Added Go 1.23 + Go 标准库、现有 DDD 模块（domain/application/infrastructure/interfaces）、`gopkg.in/yaml.v3`、现有 Discord/Telegram/Feishu/WeCom 适配层
- 002-multi-session: Added Go 1.23 + Go 标准库、现有 ClawX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化模块
- 003-skill-intent-router: Added Go 1.23 + Go 标准库、现有 ClawX DDD 模块、现有 Discord/Telegram 适配层、现有配置与持久化组件


<!-- MANUAL ADDITIONS START -->
<!-- MANUAL ADDITIONS END -->
