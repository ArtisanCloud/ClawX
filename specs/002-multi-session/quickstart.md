# 快速启动：第二阶段多窗口多会话

## 目标
在保持 Phase 1 链路稳定的前提下，完成窗口优先路由与会话切换能力。

## 开发前准备
1. 确认当前分支为 `002-multi-session`。
2. 阅读以下文档：
   - `/home/ubuntu/workspace/SynapseX/.specify/memory/constitution.md`
   - `/home/ubuntu/workspace/SynapseX/docs/plans/roadmap.md`
   - `/home/ubuntu/workspace/SynapseX/docs/plans/phase_2_multi_session.md`
   - `/home/ubuntu/workspace/SynapseX/specs/002-multi-session/spec.md`
   - `/home/ubuntu/workspace/SynapseX/specs/002-multi-session/plan.md`
3. 确认实现边界：只做多窗口多会话，不提前引入 Agent Registry 或复杂 Binding。

## 推荐实现顺序
1. 扩展消息模型与归一化输入，加入 `window_id` 与兼容值生成。
2. 扩展会话仓储接口，支持窗口绑定读写与窗口级查询。
3. 改造 Session Manager：`new/resume/continue` 刷新窗口绑定。
4. 改造 Router：执行与控制流都按“窗口优先，旧逻辑回退”。
5. 扩展控制命令：补齐 `/switch`，并固定命令优先级。
6. 补齐测试：多窗口不串线、同窗口切换、兼容回退、命令优先级。
7. 更新验证文档与交付摘要。

## 最小验收步骤
1. 在窗口 A 发送普通请求，创建会话 A1。
2. 在窗口 B 发送普通请求，创建会话 B1。
3. 确认窗口 A 后续请求继续进入 A1，窗口 B 不受影响。
4. 在窗口 A 执行 `/list`，确认可见会话列表与当前标记。
5. 在窗口 A 执行 `/switch <session_id>`，确认后续输入进入新会话。
6. 不传 `window_id` 路径继续发送请求，确认兼容行为不回退。

## 完成检查
- 多窗口路由不串线。
- 控制命令具备窗口语义。
- 兼容回退路径可用。
- 内建命令优先级稳定。
- `go test ./...` 通过。
