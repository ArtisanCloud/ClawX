# Phase 3 - Multi Agent

## 目标
- 在多 Session 稳定后，引入轻量多 Agent。

## 范围
- `Agent Registry`
- Agent 作为运行模板
- Session 记录 `agent_id`
- 默认 Agent
- 可选 `window -> agent` Binding

## 不做
- 重型中台
- 高级插件系统
- 多节点调度

## 验收标准
- 同一 Agent 下可创建多个 Session
- 不同 Agent 可拥有不同默认工作目录与运行配置
- 未配置 Agent 时仍可回退到默认流程
