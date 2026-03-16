# 快速启动：Memory Context Layer

## 目标
完成 006 功能的 MVP 验收：
- 项目与 agent 私有记忆骨架自动初始化
- 新会话首轮执行前完成分层加载
- 主/共享会话 ACL 生效
- 写回与审计链路可验证

## 开发前准备
1. 分支与规格
- 当前 feature：`006-memory-context-layer`
- 阅读：`spec.md`、`plan.md`、`research.md`、`contracts/`

2. 启动与测试命令
- 服务启动：`go run ./cmd/clawx serve`
- 全量测试：`GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...`

## 目录预期（.clawx）

```text
~/.clawx/
├── projects/
│   ├── projects.json
│   └── bindings.json
└── workspaces/
    └── <project_id>/
        ├── AGENTS.md
        ├── HEARTBEAT.md
        ├── IDENTITY.md
        ├── SOUL.md
        ├── TOOLS.md
        ├── USER.md
        ├── MEMORY.md                 # 主会话可读
        ├── memory/YYYY-MM-DD.md
        └── .agents/<agent_id>/
            ├── IDENTITY.md
            ├── TOOLS.md
            ├── MEMORY.md
            └── memory/YYYY-MM-DD.md
```

## 手工验收脚本

### A. 骨架初始化（US1）
1. `/project create tools 图片工具项目`
2. `/project use tools`
3. `/new`

预期：
- `~/.clawx/workspaces/tools/` 下基础模板文件存在。
- `.agents/<agent_id>/` 私有目录自动补齐。

### B. 多 agent 隔离（US2）
1. Agent A 在项目 `tools` 写入 `/memory note A-only`。
2. Agent B 在同项目执行加载。

预期：
- B 的加载清单不包含 A 的 `.agents/<A>/` 私有文件。
- 审计记录中出现 `denied` 或未命中行为，不出现串读。

### C. 主/共享 ACL（US3）
1. 在私聊主会话执行一次首轮加载。
2. 在群聊共享会话执行同样请求。

预期：
- 主会话可读取长期私有层（满足 owner allowlist）。
- 共享会话必定跳过 `MEMORY.md`。

### D. 写回与治理（US4）
1. `/memory note 修复图片拼接参数`
2. `/memory digest`
3. `/memory audit`

预期：
- note 默认写入 agent 私有日记。
- digest 手工可触发；自动 digest 默认关闭。
- audit 输出模板完整性、ACL 拒绝与预算裁剪统计。

## 建议自动化覆盖
- 单元：模板初始化、ACL 判定、预算截断
- 集成：首轮加载注入、多 agent 并发隔离、共享会话禁读长期私有层
- 契约：`/memory note`、`/memory digest`、`/memory audit` 语义一致性

## 完成检查
- FR-001 ~ FR-020 对应测试全部通过。
- SC-001 ~ SC-006 采样与门禁规则可执行。
- 日志可检索 `memory_loaded_files`、`memory_scope`、`memory_acl_mode` 字段。
