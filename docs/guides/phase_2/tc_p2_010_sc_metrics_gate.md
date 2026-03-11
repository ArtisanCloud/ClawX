# TC-P2-010 SC-001~SC-004 指标门禁

## 目标
- 采集并判定 SC-001~SC-004 是否满足发布阈值。

## 前置
1. 已设置 `GOCACHE` 与 `GOMODCACHE` 到仓库本地目录。
2. 已准备 JSONL 数据文件。
3. JSONL 每行格式示例：

```json
{"timestamp":"2026-03-09T10:00:00Z","sc_id":"SC-001","success":true}
```

## 步骤
1. 执行：

```bash
SYNAPSEX_PHASE2_METRICS_JSONL=/path/to/phase2_metrics.jsonl \
SYNAPSEX_PHASE2_METRICS_REPORT=docs/guides/phase_2/phase_2_metrics_report.md \
go test ./tests/integration -run TestMultiSessionMetricsReportFromJSONL -count=1
```

## 预期
1. 若满足门禁：测试通过并输出报告。
2. 若不满足门禁：测试失败并给出 failed SC 列表。

## 门禁阈值
- 统计窗口：最近 7 天
- 最小样本：每项 `>= 200`
- 阈值：
  - `SC-001 >= 95%`
  - `SC-002 >= 95%`
  - `SC-003 >= 100%`
  - `SC-004 >= 95%`
