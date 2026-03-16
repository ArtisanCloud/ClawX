# Phase 4 渠道对齐验收脚本与门禁（Wave 2~4）

## 目的

汇总 Wave 2~4 的规格对齐验收步骤，确保“契约 + 测试三联 + 实现卡 + 治理门禁”完整可执行。

## 自动化门禁

### 1. 规格治理门禁（FR-026）

```bash
bash scripts/ci/check_spec_governance.sh
```

通过标准：

- `openclaw-channel-parity.md/spec.md/plan.md/tasks.md` 同步关系满足门禁规则
- 治理文档存在且包含 FR-021 / FR-022 检查项

### 2. 渠道测试三联骨架编译门禁

```bash
go test ./tests/unit/channels/...
go test ./tests/integration/channels/...
go test ./tests/contract/channels/...
```

通过标准：

- Wave 2、Wave 3、Wave 4 三层测试包全部可编译并通过

## 人工审计门禁

### Wave 2 审计点

- 契约文档齐备：Slack、WhatsApp、Signal、Google Chat、IRC
- 测试三联齐备：unit + integration + contract
- 实现卡齐备：`specs/004-channels/channels/*.md`

### Wave 3 审计点

- 契约文档齐备：Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal
- FR-021 插件优先判定记录已填写（见治理清单）

### Wave 4 审计点

- 契约文档齐备：BlueBubbles、iMessage legacy、Tlon、WebChat
- FR-022 业务门槛判定记录已填写（见治理清单）

## 发布结论模板

- 执行日期（UTC）：
- 执行人：
- 自动化门禁结果：
- 人工审计结论：
- Wave 范围标注：
- 阻塞项：
