# 研究记录：Config Intent Plan（配置意图统一控制）

## 决策 1：混合意图冲突时先澄清再执行
- **Decision**: 当单条消息同时命中配置意图与任务意图时，先返回澄清问题，由用户确认走向。
- **Rationale**: 该场景误判代价最高；先澄清可同时保护配置安全与任务正确性。
- **Alternatives considered**:
  - 配置优先：易误改配置。
  - 任务优先：会吞掉真实配置需求。
  - 关键词硬匹配：在复杂句下不稳定。

## 决策 2：低置信度配置意图采用“建议不执行”
- **Decision**: 低置信度时仅输出建议（如引导 `/config plan`），不自动改 plan。
- **Rationale**: 保守策略可将误操作降到最低，且不阻断用户继续确认。
- **Alternatives considered**:
  - 直接改 plan：风险不可接受。
  - 直接当普通任务：丢失引导与可用性。

## 决策 3：pending plan 作用域固定为会话级
- **Decision**: 作用域采用 `channel + instance + conversation`。
- **Rationale**: 与现有会话语义一致，最小化串线与跨窗口污染。
- **Alternatives considered**:
  - 用户级共享：跨窗口误覆盖风险高。
  - 频道级共享：多人协作冲突高。
  - agent 级共享：过粗粒度，不符合交互预期。

## 决策 4：patch 历史完整保留至生命周期结束
- **Decision**: 在 `apply/cancel` 前保留完整 patch 历史，并生成审计摘要。
- **Rationale**: 支撑可追溯与回放，便于排障与治理。
- **Alternatives considered**:
  - 仅保留最终值：无法审计过程。
  - 仅保留最近 N 次：复杂会话会丢关键信息。

## 决策 5：权限分层采用“可编辑不可应用”
- **Decision**: 非管理员允许创建/编辑/查看/取消计划，禁止 apply。
- **Rationale**: 平衡协作效率与治理安全，降低管理者操作负担。
- **Alternatives considered**:
  - 全禁非管理员：协作门槛过高。
  - 允许非管理员 apply：风险过高。

## 决策 6：保持 confirm-first 单写盘出口
- **Decision**: 仅 `/config apply` 可触发配置文件写入。
- **Rationale**: 明确单一写盘出口，保证审计一致与行为可预测。
- **Alternatives considered**:
  - patch 即写盘：破坏治理边界。
  - 语义“自动提交”：用户不可控、风险高。

## 决策 7：普通任务链路必须无回归
- **Decision**: 配置意图未命中时，完整回退现有任务执行链路。
- **Rationale**: 功能增强不能牺牲主链路稳定性。
- **Alternatives considered**:
  - 所有消息先强制进配置解析：会增加正常任务延迟与误拒绝。
