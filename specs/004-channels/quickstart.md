# 快速启动：第四阶段渠道扩展

## 目标
在保持多会话与多窗口语义不回退的前提下，完成 Wave 1 交付：
- Telegram 双模式（polling/webhook）
- Feishu webhook 接入
- WeCom webhook 接入
- 增量配置闭环（`clawx config channel <name>`）

## 开发前准备
1. 确认分支：`004-channels`
2. 阅读：
   - `docs/plans/phase_4_channels.md`
   - `specs/004-channels/spec.md`
   - `specs/004-channels/contracts/*`
   - `specs/004-channels/openclaw-channel-parity.md`
3. 基础命令可运行：
   - `go run ./cmd/clawx serve`
   - `go run ./cmd/clawx config channel telegram|feishu|wecom`
   - `GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...`

## 渠道配置（增量）
推荐全部使用交互式增量配置：

```bash
go run ./cmd/clawx config channel telegram
go run ./cmd/clawx config channel feishu
go run ./cmd/clawx config channel wecom
```

要求：
- 仅修改目标渠道字段。
- 非目标渠道配置必须保持不变。

## 人工验收脚本（Wave 1）

### A. Telegram
1. `mode=polling`：发送 `/new`、普通文本、`/list`。
2. 切到 `mode=webhook`，重启后确认日志含 `telegram webhook route registered`。
3. 再次执行 `/current`、`/switch`、`/resume`、`/cancel`。

### B. Feishu
1. 在飞书后台配置回调 URL：`/webhooks/feishu/<instance-id>`。
2. 保存时通过 challenge。
3. 在会话中执行：`/new`、`/list`、`/current`、普通文本。

### C. WeCom
1. 在企业微信后台配置回调 URL：`/webhooks/wecom/<instance-id>`。
2. 保存时通过 URL 验证（echostr）。
3. 在会话中执行：`/new`、`/switch`、`/resume`、`/cancel`。

### D. 跨渠道一致性
在 Discord/Telegram/Feishu/WeCom 各执行一次：
- `/new`
- `/resume <session_id>`
- `/switch <session_id>`
- `/list`
- `/current`
- `/cancel`

预期：命令语义一致，窗口绑定行为一致。

## 自动化回归

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache go test ./...
```

## 指标采集与门禁（SC-001~SC-006）

### 1) 合成数据门禁测试

```bash
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
  go test ./tests/integration -run TestPhase4ChannelsMetricsReportGateWithSyntheticDataset
```

### 2) 真实日志门禁测试（可选）
准备 JSONL（字段：`timestamp`、`sc_id`，以及 `success` 或 `latency_ms`）。

```bash
export CLAWX_PHASE4_METRICS_JSONL=/path/to/phase4_metrics.jsonl
export CLAWX_PHASE4_METRICS_REPORT=docs/guides/phase_4/phase_4_metrics_report.md
GOCACHE=$(pwd)/.gocache GOMODCACHE=$(pwd)/.gomodcache \
  go test ./tests/integration -run TestPhase4ChannelsMetricsReportFromJSONL
```

门禁规则：
- SC-001 >= 99%
- SC-002 >= 99%
- SC-003 = 100%
- SC-004 = 100%
- SC-005 = 100%
- SC-006 p95 < 120ms

## 完成检查
- Wave 1 三渠道链路可用且单渠道故障不拖垮主进程。
- `config channel` 增量流程可用且不覆盖其他渠道。
- 契约/集成/指标门禁测试通过。
