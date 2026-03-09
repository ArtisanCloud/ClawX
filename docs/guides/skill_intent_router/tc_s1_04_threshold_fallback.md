# TC-S1-04 低置信阈值回退

## 目标
- 验证低置信度不会误触发 Skill，会回退到普通任务路径。

## 步骤
1. 在 `~/.synapsex/config.json` 中确认：
   - `intentRouter.llmFallback.enabled=true`
   - `intentRouter.llmFallback.confidenceThreshold=0.72`
2. 重启服务。
3. 发送一条模糊文本（不显式提 Skill，且与 Skill 关键词弱相关）。

## 预期
1. 请求进入普通任务路径。
2. 日志可见 `intent.reason=llm_below_threshold` 或等价回退原因。
