# 研究记录：Memory Context Layer

## 决策 1：主会话判定采用“私聊 + owner allowlist”
- **Decision**: 仅当 `peer_kind=direct` 且 `user_id` 命中 owner allowlist 时，判定为主会话，可加载长期私有记忆。
- **Rationale**: 这是最小泄露策略，能避免共享会话误读敏感记忆。
- **Alternatives considered**:
  - 仅按私聊判定：边界过宽，无法限制非 owner 私聊。
  - 按显式标签判定：灵活但高运维负担且易漏配。

## 决策 2：默认写回到 agent 私有层
- **Decision**: `/memory note` 默认写入 `.agents/<agent_id>/memory/YYYY-MM-DD.md`。
- **Rationale**: 与并发多 agent 隔离目标一致，减少共享污染。
- **Alternatives considered**:
  - 默认写项目共享层：跨 agent 易串线。
  - 双写私有+共享：实现复杂且审计噪音大。

## 决策 3：预算超限时优先保留 agent 私有层
- **Decision**: 超预算截断顺序为“先裁剪项目共享层，再裁剪公共模板层，最后才触达 agent 私有层”。
- **Rationale**: 私有层承载当前执行主体的稳定偏好，命中价值最高。
- **Alternatives considered**:
  - 均匀裁剪：简单但会破坏关键上下文。
  - 优先保留共享层：不符合多 agent 隔离优先级。

## 决策 4：digest 采用“手工 + 自动（默认关闭）”
- **Decision**: `/memory digest` 永远可手工触发；自动 digest 仅在配置开启后运行。
- **Rationale**: 默认安全，避免后台自动汇总导致不可预期写回。
- **Alternatives considered**:
  - 全手工：可控但运维成本高。
  - 全自动：体验好但风险和排障成本高。

## 决策 5：记忆隔离主键使用三元组
- **Decision**: `memory_scope_key = agent_id + project_id + route_key`。
- **Rationale**: 同时覆盖执行主体、项目边界和入口上下文，避免单维度冲突。
- **Alternatives considered**:
  - 仅 `project_id`：无法隔离同项目多 agent。
  - 仅 `session_id`：会话重建后难以承接长期记忆。

## 决策 6：模板版本采用显式 manifest
- **Decision**: 在 workspace 下维护模板版本清单（manifest），用于初始化、自愈和漂移检测。
- **Rationale**: 文件化协议需要可审计版本，否则无法稳定升级。
- **Alternatives considered**:
  - 隐式按文件存在判定：无法表达版本变更。
  - 把版本散落各文件头：实现复杂且校验成本高。

## 决策 7：加载失败默认降级继续执行
- **Decision**: 任何记忆文件解析/读取失败都记录审计并降级执行，不中断主任务链路。
- **Rationale**: 满足 FR-013，不因记忆子系统导致主流程不可用。
- **Alternatives considered**:
  - 失败即阻断：安全但可用性差。
  - 静默忽略：可用但不可审计。
