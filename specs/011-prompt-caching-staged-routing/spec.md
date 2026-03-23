# 功能规格说明：Prompt Caching + Staged LLM Routing

**功能分支**: `011-prompt-caching-staged-routing`  
**创建时间**: 2026-03-22  
**状态**: Draft  
**输入**: 用户要求“补齐开发文档，准备开发 Prompt Caching 与分阶段请求能力”。

## 用户场景与测试

### 用户故事 1 - 缓存可观测与可控（P1）

作为平台维护者，我希望自然语言请求可以使用 Prompt Caching，并且我能在日志中看到命中情况，这样我能评估成本与性能收益。

**验收场景**:
1. 当请求携带 `prompt_cache_key` 时，系统应将 key 写入 trace。
2. 当响应返回 `usage.prompt_tokens_details.cached_tokens` 时，系统应记录该值。
3. 当未命中缓存时，系统不应误报命中（`cached_tokens=0`）。

### 用户故事 2 - 分阶段请求编排（P1）

作为产品开发者，我希望将自然语言路径拆为 Planner -> Route Planner -> Executor，这样每一步上下文更小、更稳定。

**验收场景**:
1. 输入复杂自然语言后，系统先产出 `IntentPlan`，再产出具体 `ActionPlan`。
2. 执行阶段仅消费结构化计划，不直接消费自由文本命令。
3. 对低复杂请求，系统可短路 Route Planner（减少一次调用）。

### 用户故事 3 - 安全与回退一致性（P1）

作为运维，我希望即使缓存失效或分阶段某一步失败，系统仍走安全回退而不是误执行。

**验收场景**:
1. 任一阶段解析失败时，系统必须回退到“仅建议/澄清”路径。
2. `control_plan` 与 `requirement_sync` 仍必须通过既有 schema + allowlist 校验。
3. 不得从普通文本中提取 `/command` 自动执行。

## 功能需求

- **FR-001**: 自然语言 LLM 请求必须支持 `prompt_cache_key`。
- **FR-002**: 自然语言 LLM 请求必须支持 `prompt_cache_retention`，默认 `in_memory`。
- **FR-003**: 系统必须记录 `usage.prompt_tokens_details.cached_tokens`。
- **FR-004**: 系统必须提供分阶段调用接口：`Intent Planner`、`Route Planner`、`Structured Executor`。
- **FR-005**: 每个阶段必须定义独立结构化输出 schema，并进行校验。
- **FR-006**: 对低复杂度请求必须支持短路策略，避免不必要的第二阶段调用。
- **FR-007**: 执行阶段必须只消费结构化计划（`control_plan` / `requirement_sync` / SkillAction）。
- **FR-008**: 任一阶段失败必须回退到安全路径，不得发生副作用执行。
- **FR-009**: 系统必须提供缓存命中指标聚合（按阶段、按通道、按 agent）。
- **FR-010**: 文档必须定义缓存键模板规范，禁止把高变字段放入稳定前缀。

## 关键实体

- **PromptCacheConfig**: 缓存策略配置（retention/key strategy）。
- **IntentPlan**: 第一阶段意图识别结果（intent/target_agent/risk/route）。
- **RoutePlan**: 第二阶段线路计划（skills/tools/context budget）。
- **ExecutionPlan**: 第三阶段结构化执行计划（可执行）。
- **CacheUsageRecord**: 缓存命中观测记录（cached_tokens/request_tokens/stage）。

## 成功标准

- **SC-001**: 关键路径可观测 `cached_tokens` 完整率 100%（有响应即有记录）。
- **SC-002**: 相同工作负载下，prompt token 成本较基线下降。
- **SC-003**: 分阶段引入后，结构化执行误触发率不高于当前基线。
- **SC-004**: Planner 阶段 p95 不高于可接受门限（由基线测试定义）。

## 范围边界与衔接

- 本规格仅覆盖：
  - Prompt Caching 参数接入与命中观测；
  - Planner -> Route Planner -> Executor 分阶段编排；
  - 分阶段失败后的安全回退（不执行副作用）。
- 本规格不覆盖：
  - 通用“执行失败分类 -> 自动恢复 -> 升级提问”自治闭环；
  - 包管理器/网络镜像/依赖安装等运行时恢复策略库；
  - 长任务异步回执规范（任务已接收/进度/失败模板）的平台级契约。
- 与后续能力关系：
  - 011 产出的 `IntentPlan`、`RoutePlan`、`ExecutionPlan` 可作为后续自治引擎的输入；
  - 后续自治能力应复用 011 的结构化输出与审计链路，不得绕过 schema 与 allowlist。
