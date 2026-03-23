# Phase 8 - Prompt Caching 与分阶段 LLM 编排

## 背景
- 当前自然语言路径已实现结构化执行（`control_plan`、`requirement_sync`）与执行器审计闭环。
- 下一步核心目标是：在不牺牲安全性的前提下，降低 LLM token 成本、提升响应稳定性，并减少“大而全上下文”导致的漂移。
- 依据 OpenAI 官方 Prompt Caching 文档，平台侧需要显式引入缓存键设计、保留策略和命中观测。

## 目标
1. 对自然语言主路径引入 OpenAI Prompt Caching 的工程化接入规范。
2. 将当前单次大请求演进为分阶段调用：`Intent Planner -> Route Planner -> Structured Executor`。
3. 为缓存命中率、路由延迟、误执行率建立可观测与门禁。

## 范围
- 仅覆盖自然语言路径（非 `/` 开头消息）。
- 覆盖 `control_plan` 与 `requirement_sync` 两类结构化执行入口。
- 覆盖 OpenAI 请求参数：
  - `prompt_cache_key`
  - `prompt_cache_retention`
  - 响应观测 `usage.prompt_tokens_details.cached_tokens`

## 非目标
- 不改变命令路径（`/...`）行为。
- 不把 Prompt Caching 当作状态存储；会话状态与审计仍由 ClawX 持久化层承担。
- 不引入新的外部编排服务。

## 里程碑
1. **M1 - 协议与埋点**：
  - 请求支持 `prompt_cache_key`、`prompt_cache_retention`。
  - 响应记录 `cached_tokens` 到 trace/audit。
2. **M2 - 分阶段编排**：
  - 引入三段调用接口与结构化中间产物。
  - 每段上下文预算独立控制。
3. **M3 - 策略与门禁**：
  - 缓存键模板标准化。
  - 命中率/延迟/误执行率门禁与回滚策略。

## 成功标准
- SC-001：自然语言路径中，命中请求 `cached_tokens > 0` 的比例可观测且稳定提升。
- SC-002：平均输入 token 成本显著下降（同工作负载对比基线）。
- SC-003：结构化执行误触发率不回升（保持现有控制面安全门禁）。
- SC-004：分阶段调用后，关键路径 p95 延迟不高于现基线可接受范围。

## 风险与缓解
- 风险：缓存键不稳定导致低命中。
  - 缓解：固定系统前缀与阶段标识，动态内容后置。
- 风险：分阶段后调用次数增加，延迟抖动。
  - 缓解：低复杂请求走短路策略（跳过 Route Planner）。
- 风险：模型输出多样性影响中间结构化结果一致性。
  - 缓解：严格 schema 校验，失败回退到安全路径。

## 交付映射
- 对应规格目录：`specs/011-prompt-caching-staged-routing/`
- 对应任务清单：`specs/011-prompt-caching-staged-routing/tasks.md`
