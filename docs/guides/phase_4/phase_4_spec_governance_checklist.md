# Phase 4 规格治理检查清单（FR-021 / FR-022 / FR-026）

## 目的

为 Wave 2~4 渠道扩展提供统一的“进入实现前”治理门禁，确保插件边界、业务门槛与规格更新顺序可审计。

## FR-021：Wave 3 插件优先判定

适用渠道：Matrix、Mattermost、Microsoft Teams、Nextcloud Talk、LINE、Nostr、Synology Chat、Twitch、Zalo、Zalo Personal。

判定规则：满足以下任一条件时，必须采用插件优先策略。

- 需要私有化/内网部署
- 需要企业身份系统集成
- 预计维护成本 > 2 人日/月

执行记录模板：

- 渠道：
- 判定日期（UTC）：
- 触发条件：
- 结论（插件优先/非插件）：
- 审批人：
- 备注：

## FR-022：Wave 4 业务门槛判定

适用渠道：BlueBubbles、iMessage legacy、Tlon、WebChat。

接入前必须满足至少一项：

- 至少 1 个付费客户明确需求并确认上线窗口
- 内部月活预测 >= 50
- 存在明确合规/法务要求

执行记录模板：

- 渠道：
- 判定日期（UTC）：
- 业务门槛（至少一项）：
- 结论（进入实现/保持 backlog）：
- 审批人：
- 备注：

## FR-026：技术规范更新顺序门禁

规范文件顺序：

1. `specs/004-channels/openclaw-channel-parity.md`
2. `specs/004-channels/spec.md`
3. `specs/004-channels/plan.md`
4. `specs/004-channels/tasks.md`

检查项：

- [ ] 若本次变更触发渠道对齐范围更新，四份文件均已同步。
- [ ] PR 描述已标注波次范围（Wave 1/2/3/4）。
- [ ] `tasks.md` 中新增任务与 `spec/plan` 保持一致。
- [ ] 相关治理文档（本文件与验收文档）已更新。

## 审核结论

- 审核日期（UTC）：
- 审核人：
- 结论（通过/驳回）：
- 阻塞项：
