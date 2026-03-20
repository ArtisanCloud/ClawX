# 快速启动：Skill Governance Orchestration（最终验收）

## 目标
- 自然语言优先的技能路由与命令入口统一到 `SkillAction`
- 技能治理（注册/策略/绑定/隔离）闭环
- 高风险确认、审计、回放闭环
- 上下文压缩（`context_digest + skill_catalog_digest`）与性能门禁达标

## 环境
1. 进入仓库根目录：`/home/ubuntu/workspace/ClawX`
2. 可选启动服务：`go run ./cmd/clawx serve`
3. 推荐先跑专项回归：
   - `go test ./cmd/clawx -run "Skill|skill" -count=1`
   - `go test ./tests/contract -run "Skill|skill" -count=1`
   - `go test ./tests/integration -run "Skill|skill" -count=1`
   - `go test ./tests/unit -run "Skill|skill" -count=1`

## 手工验收脚本

### A. 路由与统一动作（US1）
1. 发送 NL：`请安装技能 bid.collect 到 bid-all`
2. 发送命令：`/skill install bid.collect --agent bid-all`
3. 发送非技能请求：`请帮我跑一下 go test ./...`

预期：
- 1/2 落到统一动作语义，返回安装成功。
- 低置信度请求仅澄清，不直接执行。
- 非技能请求回退原任务链路，不被 skill handler 吞掉。

### B. 注册与策略治理（US2）
1. 允许源安装：`/skill install bid.collect --source builtin --version v1.0.0`
2. 受限源安装：`/skill install bid.collect --source clawhub --version v1.0.0`（配合仅 builtin 白名单策略）
3. 禁用：`/skill disable bid.collect`

预期：
- 白名单来源通过。
- 非白名单返回 `策略拒绝`。
- 禁用后立即生效。

### C. 共享与隔离绑定（US3）
1. 全局绑定：`/skill bind bid.collect --scope global --version v1.0.0`
2. Agent 绑定：`/skill bind bid.collect --scope agent-local --agent bid-all --version v2.0.0`

预期：
- `agent-local > project > global` 优先级稳定。
- 删除上层后自动回落到下一层。

### D. 高风险确认与回放（US4）
1. 高风险执行：`/skill run bid.collect --risk high --agent bid-all`
2. 拒绝一次：`/skill run bid.collect --risk high --agent bid-all --reject <confirmation_id>`
3. 重新确认：`/skill run bid.collect --risk high --agent bid-all --confirm <confirmation_id>`
4. 回放：`/skill replay <trace_id>`

预期：
- 未确认不得执行。
- 拒绝确认无副作用。
- 回放能看到 `confirm_required` 与 `success` 等结构化事件链路。

### E. Digest 与性能门禁（Phase 7）
1. 路由输入包含 `message + context_digest + skill_catalog_digest`
2. 性能测试：`go test ./tests/integration -run "TestSkillRoutingPerformanceWithDigests" -count=1`

预期：
- digest 参与路由输入，行为稳定。
- 在 `catalog>=200` 与 `history=50` 条件下中位数路由时间不超过 `300ms`。

## 最终回归
- 全量：`go test ./...`
- 回归入口：`tests/integration/skill_orchestrator_regression_suite_test.go`

## Post-MVP 增量验收（Phase 9）

### F. 内置 Web 技能可见性
1. 运行：`go test ./tests/contract -run "TestSkillBuiltin(Discovery|WebSearch|WebFetch)Contract" -count=1`
2. 或直接查看：`clawx skill list`

预期：
- `web-search`、`web-fetch` 可被发现且状态为 active。

### G. 第三方归档安装（本地/`file://`）
1. 准备 skill 包（含 `SKILL.md`）。
2. 安装：`/skill install bid.market --source clawhub --version v1.2.3 --package file:///tmp/skill-package.tgz`

预期：
- 返回 `技能安装成功` 且包含 `安装目录`。
- 目录存在：`~/.clawx/skills/marketplace/bid.market/v1.2.3/`
- 安装失败时目录自动回滚，不残留半成品。

## 交付检查
- FR-001 ~ FR-027 均有落地代码与测试映射。
- SC-001 ~ SC-007 有可执行验证路径。
