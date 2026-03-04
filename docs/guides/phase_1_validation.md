# 第一阶段执行与检验指南

## 目标
- 给当前 `001-phase1-foundation` feature 提供一套可直接执行的本地运行与检验流程。
- 用于确认第一阶段的代码骨架、主链路和测试状态是否符合预期。

## 适用范围
- 当前仓库根目录：`SynapseX`
- 当前阶段：`Phase 1 - Foundation`
- 当前实现状态：
  - `Router -> Session Manager -> Backend Adapter` 已接通
  - `/new`、`/resume`、`/list`、`/cancel` 控制流已具备骨架
  - 输出分段、基础格式化、部分失败提示已具备骨架
  - Discord / Telegram 适配器为本地内存型占位实现

## 前置条件
1. 当前分支为 `001-phase1-foundation`
2. 已安装 Go 1.23
3. 建议关闭 Go 自动下载工具链

## 环境准备
在仓库根目录执行：

```bash
go env -w GOTOOLCHAIN=local
go version
```

预期：
- `go version` 输出为 `go1.23.x`

## 基础编译与测试
在仓库根目录执行：

```bash
go test ./...
```

预期：
- 所有包可成功编译
- `tests/contract`
- `tests/integration`
- `tests/unit`
  均返回 `ok`

如果需要避免本机缓存权限问题，可改用：

```bash
GOCACHE=/tmp/go-build-cache go test ./...
```

## 启动最小主程序
执行：

```bash
go run ./cmd/synapsex
```

预期：
- 标准输出包含：

```text
synapsex phase 1 chain and channel adapters initialized
```

这表示：
- 配置加载成功
- 内存态会话仓储已装配
- 最小后端执行器已装配
- 输出处理器已装配
- Discord / Telegram 适配器骨架已装配

## 代码结构检查
应确认以下核心文件存在：

- `cmd/synapsex/main.go`
- `internal/domain/session/session.go`
- `internal/domain/execution/run.go`
- `internal/application/service/session_manager.go`
- `internal/application/service/router.go`
- `internal/application/service/router_session_flow.go`
- `internal/application/service/router_control_flow.go`
- `internal/application/service/output_delivery.go`
- `internal/infrastructure/backend/runner.go`
- `internal/interfaces/chat/normalize.go`
- `internal/interfaces/chat/error_response.go`
- `internal/interfaces/admin/health_handler.go`

## 功能级人工检查
当前阶段的渠道与后端仍是骨架实现，因此这里做“结构性验证”，不是外部平台真联调。

### 会话主链路
重点确认代码路径：
- `new` 模式可创建新会话
- `continue` 模式在无会话时可自动创建
- `resume` 模式可恢复指定会话
- 同一会话运行中再次执行会返回繁忙错误

对应文件：
- `internal/application/service/session_manager_create.go`
- `internal/application/service/session_manager_continue.go`
- `internal/application/service/session_manager_resume.go`
- `internal/application/service/router_session_flow.go`

### 控制命令
重点确认控制命令已覆盖：
- `/new`
- `/resume <session_id>`
- `/list`
- `/cancel`

对应文件：
- `internal/application/command/control_command.go`
- `internal/application/service/router_control_flow.go`
- `internal/interfaces/chat/control_response.go`

### 输出送达
重点确认：
- 长输出会被分段
- 发送失败会重试
- 重试后失败会提示“部分输出发送失败”

对应文件：
- `internal/application/service/output_streamer.go`
- `internal/application/service/output_formatter.go`
- `internal/application/service/output_delivery.go`

## 当前已知限制
- Discord / Telegram 尚未接真实 SDK
- 后端执行器仍是占位实现，当前会返回 `accepted: <input>`
- 测试文件当前是基础骨架，不是完整业务测试
- 健康处理器已实现，但尚未接入实际 HTTP server

## 检验通过标准
满足以下条件即可认为 Phase 1 当前版本可进入下一轮开发或真实集成：

1. `go test ./...` 通过
2. `go run ./cmd/synapsex` 可启动并输出初始化完成信息
3. 关键主链路文件齐全
4. 控制流、会话流、输出流的骨架逻辑已可阅读并保持边界清晰
5. `specs/001-phase1-foundation/tasks.md` 中 `Phase 1` 到 `Phase 6` 已全部完成

## 下一步建议
1. 将 Discord / Telegram 适配器从内存型占位实现替换为真实 SDK 接入
2. 将最小后端执行器替换为真实 CLI 调用
3. 把当前测试骨架升级为可覆盖真实业务路径的测试

