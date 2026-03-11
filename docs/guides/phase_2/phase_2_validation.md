# Phase 2 验证指南与结果

## 目的

记录第二阶段“多窗口多会话”的回归验证、人工验收与发布门禁结论。

## 验收范围

- 多窗口独立会话不串线
- 同窗口会话恢复与切换
- 兼容路径（无显式 `window_id`）
- 控制命令窗口语义
- SC-001 ~ SC-005 成功标准

## 执行环境

- 执行日期（UTC）：2026-03-09
- 执行分支：`002-multi-session`
- 执行目录：`/home/ubuntu/workspace/SynapseX`
- Go 缓存策略：`GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache`

## 自动化回归（T039）

执行命令：

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

结果：通过（unit/integration/contract 全量通过）。

## SC-005 人工验收报告（T043）

验收脚本来源：`specs/002-multi-session/quickstart.md` 中的“SC-005 人工验收脚本”。

本轮执行方式：
- 以脚本步骤为基线进行场景复核。
- 使用以下命令对“多窗口隔离 + 窗口内切换”核心流程进行重复演练：

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
go test ./tests/integration \
  -run 'TestMultiSessionWindowRoutingWindowIsolationParallelContinue|TestMultiSessionControlFlowListCurrentSwitch' \
  -count=10
```

统计结果：
- 样本总数：20（2 个核心场景 * 10 轮）
- 通过样本：20
- 失败样本：0
- 通过率：100%

结论：
- SC-005 阈值（通过率 >= 90%）在当前样本下满足。
- 但未达到规范要求的样本下限（>= 200），仅可视为预验收结果。

## SC-001 ~ SC-004 指标采集状态

- 指标采集脚本：`tests/integration/multi_session_metrics_report_test.go`
- 统计口径：7 天窗口，单项 >= 200 样本，阈值按规格执行。
- 当前状态：统计脚本已就绪；尚未注入真实 JSONL 指标数据，因此正式门禁结果待采集。

执行模板：

```bash
SYNAPSEX_PHASE2_METRICS_JSONL=/path/to/phase2_metrics.jsonl \
SYNAPSEX_PHASE2_METRICS_REPORT=docs/guides/phase_2/phase_2_metrics_report.md \
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
go test ./tests/integration -run TestMultiSessionMetricsReportFromJSONL -count=1
```

## 发布建议

- 工程质量：可进入 RC（代码与自动化回归稳定）。
- 发布门禁：暂不放行。
- 阻塞项：
  - SC-001 ~ SC-004 缺真实 7 天数据样本。
  - SC-005 样本量不足 200。
