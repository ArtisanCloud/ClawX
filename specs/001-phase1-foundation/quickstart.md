# 快速启动：第一阶段基座能力

## 目标
帮助开发者按第一阶段范围快速启动实现工作，并验证主链路是否成立。

## 开发前准备
1. 确认当前分支为 `001-phase1-foundation`。
2. 阅读以下文档：
   - `.specify/memory/constitution.md`
   - `docs/plans/phase_1_foundation.md`
   - `specs/001-phase1-foundation/spec.md`
   - `specs/001-phase1-foundation/plan.md`
3. 确认实现语言为 Go，且代码按 DDD 分层组织。

## 推荐实现顺序
1. 建立 Go 项目基本目录：
   - `cmd/`
   - `internal/domain/`
   - `internal/application/`
   - `internal/infrastructure/`
   - `internal/interfaces/`
2. 先实现 `Session` 领域模型和会话锁规则。
3. 实现 `Session Manager` 的内存态版本。
4. 实现 `Router`，打通“新建 / 恢复 / 继续执行”的主判断流程。
5. 实现 `Backend Adapter`，接入当前受支持后端的最小执行能力。
6. 实现 `Output Streamer`，处理长输出分段与顺序送达。
7. 分别接入 Discord 与 Telegram 的基础渠道适配器。
8. 补齐结构化日志和最小健康探针。
9. 补齐控制命令、输出分段、渠道适配器与收尾测试骨架。

## 第一阶段完成检查
- 能从受支持渠道创建新会话。
- 能恢复已有会话。
- 能在当前会话中继续执行。
- 同一会话内不会并发执行。
- 长输出可分段且顺序正确。
- 超时、取消、失败都有清晰反馈。
- 进程与后端探针可被观测。

## 当前实现状态
- 已建立 `Router -> Session Manager -> Backend Adapter` 直连主链路。
- 已提供 `/new`、`/resume`、`/list`、`/cancel` 的控制流骨架。
- 已提供长输出分段、基础格式化、部分失败提示以及 Discord / Telegram 适配器骨架。
- 已补齐统一错误映射、健康处理器和测试文件骨架。
- 在具备 Go 工具链后，应运行 `go test ./...` 完成本地编译与测试验证。

## 明确不做
- 多窗口管理
- 多 Agent 管理
- 自动路由
- 重型中台或复杂调度层
