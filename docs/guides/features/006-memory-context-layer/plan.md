# 006 Memory Context Layer 计划草案（v0.1）

## 1. 背景
当前 ClawX 已实现 route/window/session/project 四层上下文隔离，但“长期记忆”仍主要依赖后端会话线程（如 Codex `BackendSessionID`）和短期消息上下文。

这会导致两个问题：
- 会话重建或线程丢失后，助手人格与用户偏好无法稳定恢复。
- 缺少可治理、可审计、可隔离的文件化记忆层。

## 2. 目标
在 `.clawx/workspaces/<project_id>/` 引入文件化记忆协议，提供：
- 启动即读的人格/用户/工具上下文（静态记忆）
- 项目内日记与长期记忆（动态记忆）
- 主会话与共享会话的隐私隔离策略（memory ACL）

## 3. 对齐策略（落到 ClawX）
按现有 `.clawx` 项目空间落地，不复用外部命名空间：

```text
~/.clawx/
├── workspaces/
│   ├── main/
│   │   ├── AGENTS.md
│   │   ├── BOOTSTRAP.md (可选，首次初始化后可删除)
│   │   ├── HEARTBEAT.md
│   │   ├── IDENTITY.md
│   │   ├── SOUL.md
│   │   ├── TOOLS.md
│   │   ├── USER.md
│   │   ├── MEMORY.md (仅主会话可读)
│   │   └── memory/YYYY-MM-DD.md
│   └── <project_id>/
│       ├── AGENTS.md
│       ├── HEARTBEAT.md
│       ├── IDENTITY.md
│       ├── SOUL.md
│       ├── TOOLS.md
│       ├── USER.md
│       └── memory/YYYY-MM-DD.md
└── state/
    └── memory/ (索引/缓存/审计)
```

### 3.1 Agent 隔离补充（你提到的重点）
记忆隔离主键不是单一 `project_id`，而是三元组：
- `agent_id`（执行主体）
- `project_id`（项目空间）
- `route_key`（会话入口）

分层策略如下：
- 会话状态：按 agent 隔离，已存在于 `~/.clawx/agents/<agent_id>/sessions/*`。
- 项目共享记忆：按 project 隔离，位于 `~/.clawx/workspaces/<project_id>/`。
- Agent 专属记忆（新增）：位于 `~/.clawx/workspaces/<project_id>/.agents/<agent_id>/`。

建议目录：

```text
~/.clawx/workspaces/<project_id>/
├── AGENTS.md / SOUL.md / USER.md / TOOLS.md / ...
├── memory/YYYY-MM-DD.md
└── .agents/
    └── <agent_id>/
        ├── IDENTITY.md
        ├── TOOLS.md
        ├── MEMORY.md (agent-private)
        └── memory/YYYY-MM-DD.md
```

加载顺序（MVP）：
1. `<project>/.agents/<agent_id>/`（agent 私有）
2. `<project>/`（项目共享）
3. 仅主会话可读 `<project>/MEMORY.md`（若存在）

写回策略（MVP）：
- 默认写入 agent 私有日记：`.agents/<agent_id>/memory/YYYY-MM-DD.md`。
- 显式共享写入（后续命令支持）才写入项目共享 `memory/YYYY-MM-DD.md`。

## 4. 非目标（本阶段不做）
- 跨节点共享记忆一致性（分布式场景）。
- 向量数据库/Embedding 检索层。
- 自动外发动作（邮件、社媒）中的 memory 授权代理。

## 5. 设计原则
- 项目隔离优先：记忆文件默认按 project scope 读取。
- 最小泄露：`MEMORY.md` 默认仅允许“主会话（私聊）”加载。
- 显式可审计：每次执行记录已加载文件清单与来源。
- 文件优先：可读可改可备份，避免黑箱状态。

## 6. 实施阶段

### Phase A：基础骨架（P1）
- 在 `/project create` 时初始化记忆模板文件。
- 为 default project 补齐初始化（首次启动或修复时）。
- 新增模板渲染与版本字段（便于后续升级）。

代码落点建议：
- `internal/application/project/service_create.go`
- `internal/application/project/service_repair.go`
- `internal/infrastructure/persistence/`（模板与版本管理）

### Phase B：加载策略（P1）
- 新增 `MemoryLoader`，按 route/project/session 类型加载文件。
- 加载顺序：`SOUL -> USER -> TOOLS -> AGENTS -> daily memory`。
- 主会话私有扩展：可额外加载 `MEMORY.md`。
- 共享会话禁读 `MEMORY.md`。
- 同项目多 agent 并发时，优先加载 `.agents/<agent_id>/` 私有记忆，再加载项目共享层。

代码落点建议：
- `internal/application/service/router_session_flow.go`
- `internal/infrastructure/backend/profile_runner.go`（执行前注入）

### Phase C：写回能力（P2）
- 增加 `/memory note <text>`：追加到 `memory/YYYY-MM-DD.md`。
- 增加 `/memory digest`：把日记摘要合并到 `MEMORY.md`（仅主会话）。
- 增加写回节流与长度限制。

### Phase D：观测与治理（P2/P3）
- 审计日志新增字段：
  - `memory_loaded_files`
  - `memory_scope`（project/main/shared）
  - `memory_acl_mode`
- 新增自检命令：`/memory audit`（缺失文件、权限问题、模板版本漂移）。

## 7. 安全与 ACL 规则
- 规则 1：共享 route（群聊/频道）禁止加载 `MEMORY.md`。
- 规则 2：跨项目禁止读取其他项目 workspace 下 memory。
- 规则 3：跨 agent 禁止读取其他 agent 的 `.agents/<other_agent_id>/`。
- 规则 4：写回默认写入当前 `agent_id + project_id` 私有目录。
- 规则 5：敏感字段禁止进入公开日志（只记录文件名，不记录全文）。

## 8. 验收标准（MVP）
- `/project create <id>` 后，`~/.clawx/workspaces/<id>/` 自动具备记忆模板文件。
- 新会话首轮执行前，系统能按策略加载记忆文件。
- 群聊场景无法读取 `MEMORY.md`。
- 同项目双 agent 并发时，不会读取对方 agent-private 记忆。
- 记忆加载过程可在日志中追踪到文件级元信息。

## 9. 测试计划
- 单元测试：
  - 记忆模板初始化
  - ACL 判定（main vs shared）
  - 文件加载顺序与截断策略
- 集成测试：
  - 创建项目后模板存在
  - `/new` 首次执行加载记忆
  - 群聊 route 下 `MEMORY.md` 屏蔽
  - 双 agent 同项目并发时 agent-private 互不可见
- 契约测试：
  - `/memory note`、`/memory digest`、`/memory audit` 命令语义

## 10. 风险与缓解
- 风险：上下文注入过长影响模型响应。
  - 缓解：分层截断（固定头部 + 预算上限 + 重要性优先）。
- 风险：用户在共享频道误暴露私有记忆。
  - 缓解：ACL 强制 + 审计告警 + 默认 deny。
- 风险：模板文件漂移造成行为不一致。
  - 缓解：模板版本号 + `/memory audit` 漂移检测。

## 11. 建议里程碑
- M1（2-3 天）：Phase A 完成 + 基础测试。
- M2（2-4 天）：Phase B 完成 + ACL 回归。
- M3（2-3 天）：Phase C 最小命令集。
- M4（1-2 天）：Phase D 审计与文档收口。

## 12. 下一步
建议直接新开 feature：`006-memory-context-layer`，先落地 Phase A + Phase B（MVP），完成后再迭代写回与治理。
