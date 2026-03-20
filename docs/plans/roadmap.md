# Roadmap

## 阶段划分

| 阶段 | 目标 | 核心交付 |
| --- | --- | --- |
| Phase 1 | 打稳最短执行链路 | 路由、会话管理、后端适配、单窗口单会话（不依赖数据库） |
| Phase 2 | 默认支持多窗口、多会话 | Window Context、多窗口绑定、多会话切换、本地文件持久化 |
| Phase 3 | 引入轻量多 Agent | Agent Registry、运行模板、默认 Agent、可选 Binding |
| Phase 4 | 渠道扩展与接入稳定 | Wave 1: Telegram webhook/polling、Feishu/WeCom、增量渠道配置；Wave 2~4: 按 OpenClaw 对齐清单扩展其余渠道 |
| Phase 5 | 项目空间隔离与路由 | `project` 维度路由、`/project` 命令闭环、同 bot 多项目 workspace 隔离、confirm-first 项目切换建议 |
| Phase 6 | Agent/Context/Intent 一体化 | 指令与自然语言融合、统一 pending plan、语义 patch、confirm-first 配置治理、控制面审计 |
| Phase 7 | Agent Skill 治理与编排 | Skill Registry/Policy/Binding、自然语言 LLM-first skill 路由、技能执行审计、共享与隔离规则 |

## 规则
- 每个阶段只引入当前必须的复杂度。
- `Session` 优先于 `Agent`。
- `Agent` 优先作为模板，不优先做成重型系统。

## 关联文档
- 功能专题：`docs/features/`
- 基础架构：`docs/02_architecture/`
