# Phase 4 - Channels

## 阶段目标
- 在保持现有 Session/Agent 主链路稳定的前提下，扩展渠道能力。
- 完成 Wave 1 交付：Telegram 双模式 + Feishu + WeCom。
- 建立与 OpenClaw 渠道能力对齐的波次路线，并同步到技术规范与任务清单。

## 交付状态（2026-03-12）

### 已完成
- Telegram：polling/webhook 双模式、webhook 校验、setWebhook 与重试隔离。
- Feishu：challenge、签名校验、文本消息归一化、控制命令链路。
- WeCom：URL 验证、签名校验、消息解密、文本消息归一化、控制命令链路。
- 配置：`clawx config channel telegram|feishu|wecom` 增量交互流程可用。
- 稳定性：渠道级重试隔离（单渠道失败不退出主进程）。
- 测试：
  - 适配器单测（Telegram/Feishu/WeCom）
  - 控制流集成测试（Telegram/Feishu/WeCom）
  - 跨渠道控制语义契约测试
  - 增量配置保留测试
  - Phase 4 指标门禁测试（SC-001~SC-006）

### 已固化文档
- 规格与任务：`specs/004-channels/spec.md`、`specs/004-channels/tasks.md`
- 契约：`specs/004-channels/contracts/*`
- 指南：
  - `docs/guides/telegram_bot_setup.md`
  - `docs/guides/feishu_bot_setup.md`
  - `docs/guides/wecom_bot_setup.md`
  - `docs/guides/telegram_config_incremental.md`

## 当前范围结论
- Wave 1：已完成并可独立验收。
- Wave 2~4：已进入规格与任务清单（对齐路线完成，代码未启动）。

## 风险与缓解

### R1. 外部渠道网络稳定性
- 风险：代理/网络抖动导致 webhook 回发失败或超时。
- 缓解：渠道级指数退避 + 错误日志字段（`channel/instance/retry_count/last_error`）。

### R2. 渠道签名/加解密实现差异
- 风险：各渠道验签细节差异造成误判。
- 缓解：按渠道 contract 固化协议，适配器单测覆盖 challenge/签名/解密失败路径。

### R3. 增量配置误操作
- 风险：修改单渠道时误覆盖其他渠道配置。
- 缓解：批量 patch + 原子写盘 + 非目标渠道保留测试。

### R4. 指标采集数据质量
- 风险：SC 统计样本不足或 JSONL 字段不规范。
- 缓解：门禁测试强制 `>=200` 样本与字段校验，输出统一 markdown 报告。

## 下一步
1. 进入 Phase 7A：补齐 Wave 2~4 的契约与测试三联骨架。
2. 在 CI 中引入 Phase 4 指标门禁（可复用 `channels_metrics_report_test.go`）。
3. 基于对齐矩阵按业务优先级启动 Wave 2 渠道实现。
