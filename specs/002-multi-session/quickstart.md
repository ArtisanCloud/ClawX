# 快速启动：第二阶段多窗口多会话

## 目标
在保持 Phase 1 链路稳定的前提下，完成窗口优先路由与会话切换能力。

## 开发前准备
1. 确认当前分支为 `002-multi-session`。
2. 阅读以下文档：
   - `/home/ubuntu/workspace/ClawX/.specify/memory/constitution.md`
   - `/home/ubuntu/workspace/ClawX/docs/plans/roadmap.md`
   - `/home/ubuntu/workspace/ClawX/docs/plans/phase_2_multi_session.md`
   - `/home/ubuntu/workspace/ClawX/specs/002-multi-session/spec.md`
   - `/home/ubuntu/workspace/ClawX/specs/002-multi-session/plan.md`
3. 确认实现边界：只做多窗口多会话，不提前引入 Agent Registry 或复杂 Binding。

## 推荐实现顺序
1. 扩展消息模型与归一化输入，加入 `window_id` 与兼容值生成。
2. 扩展会话仓储接口，支持窗口绑定读写与窗口级查询。
3. 改造 Session Manager：`new/resume/continue` 刷新窗口绑定。
4. 改造 Router：执行与控制流都按“窗口优先，旧逻辑回退”。
5. 扩展控制命令：补齐 `/switch`，并固定命令优先级。
6. 补齐测试：多窗口不串线、同窗口切换、兼容回退、命令优先级。
7. 更新验证文档与交付摘要。

## 最小验收步骤
1. 在窗口 A 发送普通请求，创建会话 A1。
2. 在窗口 B 发送普通请求，创建会话 B1。
3. 确认窗口 A 后续请求继续进入 A1，窗口 B 不受影响。
4. 在窗口 A 执行 `/list`，确认可见会话列表与当前标记。
5. 在窗口 A 执行 `/switch <session_id>`，确认后续输入进入新会话。
6. 不传 `window_id` 路径继续发送请求，确认兼容行为不回退。

## 自动化回归（T039）

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

说明：
- 在沙箱或受限环境下使用本地缓存目录，避免访问系统级 Go 缓存目录失败。
- 回归需覆盖 `tests/unit`、`tests/integration`、`tests/contract`。

## 指标采集与门禁（SC-001~SC-004）

输入日志格式（JSONL，每行一条）：

```json
{"timestamp":"2026-03-09T10:00:00Z","sc_id":"SC-001","success":true}
```

采集要求：
- 统计窗口：最近 7 天滚动窗口。
- 每个 SC 的最小样本：`>= 200`。
- 阈值：
  - `SC-001 >= 95%`
  - `SC-002 >= 95%`
  - `SC-003 >= 100%`
  - `SC-004 >= 95%`

执行命令：

```bash
CLAWX_PHASE2_METRICS_JSONL=/path/to/phase2_metrics.jsonl \
CLAWX_PHASE2_METRICS_REPORT=docs/guides/phase_2/phase_2_metrics_report.md \
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
go test ./tests/integration -run TestMultiSessionMetricsReportFromJSONL -count=1
```

## SC-005 人工验收脚本

目标：验证“用户可在两个窗口中独立继续会话，并在其中一个窗口切换会话”。

步骤：
1. 选择测试用户 `U`，在窗口 A 发送普通消息，记录 `session_a1`。
2. 在窗口 B 发送普通消息，记录 `session_b1`，确认 `session_b1 != session_a1`。
3. 在窗口 A 再发送一条普通消息，确认仍落在 `session_a1`。
4. 在窗口 A 执行 `/new`，记录 `session_a2`。
5. 在窗口 A 执行 `/switch <session_a1>`，随后发送普通消息，确认落在 `session_a1`。
6. 在窗口 B 发送普通消息，确认仍落在 `session_b1`。

通过标准：
- 单个测试用户在无人工干预下完成上述 6 步，记为通过。
- 通过率计算：`通过用户数 / 总测试用户数`。
- 发布门禁要求：`通过率 >= 90%`，且建议样本不少于 200。

## 完成检查
- 多窗口路由不串线。
- 控制命令具备窗口语义。
- 兼容回退路径可用。
- 内建命令优先级稳定。
- `go test ./...` 通过。
