# 多会话与多 Agent（专题总览）

## 功能定位
- 本专题定义 ClawX 的多会话与多 Agent 能力。
- 当前默认目标是：先完成多窗口、多会话，再逐步引入多 Agent。

## 核心结论
- `Session` 负责上下文隔离。
- `Agent` 负责运行环境隔离。
- 当前阶段不应把 `Agent` 作为前置复杂度。

## 文档入口
- 架构设计： [architecture.md](./architecture.md)
- 参考实现： [implementation.md](./implementation.md)

## 阶段对应
- Phase 2：以多 Session 为主
- Phase 3：引入轻量多 Agent
