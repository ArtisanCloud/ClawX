# 研究记录：Unified Scheduler Center

## 决策 1：默认任务作用域采用 `project + agent`
- **Decision**: 任务默认绑定 `project_id + agent_id`，route 绑定作为可选扩展。
- **Rationale**: 满足隔离与复用平衡，避免 route 级重复注册任务。
- **Alternatives considered**:
  - `project+agent+route` 默认：隔离更强但运维成本高，重复任务多。
  - `project` 默认：会造成跨 agent 任务串扰风险。

## 决策 2：自然语言“每周清理图片记录”默认时间固定为每周日 03:00
- **Decision**: 未显式指定时间时，创建每周日 03:00 的周期任务。
- **Rationale**: 对齐低峰运维窗口，减少业务高峰干扰。
- **Alternatives considered**:
  - 周一 00:00：可读性强但不一定是低峰。
  - 每次追问用户时间：交互成本高，降低自动化效率。

## 决策 3：图片清理默认保留最近 30 天
- **Decision**: `image.cleanup` 默认仅清理超过 30 天的记录。
- **Rationale**: 平衡空间回收与回溯需求，符合常见运维策略。
- **Alternatives considered**:
  - 7 天：清理激进，回溯窗口过短。
  - 90 天：空间回收效果弱。

## 决策 4：任务失败后保持启用，等待下一周期
- **Decision**: 单次执行失败仅记录状态，不自动暂停或删除。
- **Rationale**: 降低误中断风险，避免短暂故障导致任务永久失活。
- **Alternatives considered**:
  - 立即重试多次：可能放大故障并造成抖动。
  - 失败即暂停：需要人工恢复，运维负担大。

## 决策 5：调度中心先采用单进程内 runner + 文件持久化
- **Decision**: 在 `clawx serve` 内启动调度 runner，任务定义与执行记录落本地文件。
- **Rationale**: 符合当前单节点部署约束，交付快且运维简单。
- **Alternatives considered**:
  - 外部 cron/systemd 组合：分散治理，无法统一命令管理。
  - 引入分布式调度组件：超出当前 feature 范围。

## 决策 6：自然语言调度采用“显式规则映射”
- **Decision**: 先对高价值表达建立确定性映射（如“每周清理图片记录”）。
- **Rationale**: 可控、可测试、低误判。
- **Alternatives considered**:
  - 全量 LLM 自由解析：灵活但可预测性和审计性不足。
  - 仅支持命令：学习成本高，不符合当前目标。

## 决策 7：执行记录必须结构化保存并可查询最近结果
- **Decision**: 每次执行都记录开始、结束、状态、错误摘要与结果摘要。
- **Rationale**: 支撑运维排障和成功标准 SC-004/SC-005。
- **Alternatives considered**:
  - 只在日志中留痕：难以按任务聚合查询。
  - 仅保留最后一次状态：丢失历史诊断信息。
