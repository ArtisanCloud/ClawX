# 渠道实现卡模板

## 基本信息
- 渠道：`<channel>`
- 波次：`Wave <2|3|4>`
- 当前状态：`planned|in_progress|done`
- 契约文档：`specs/004-channels/contracts/<contract>.md`

## 目标
- 复用统一消息模型与 Router/Session 链路。
- 保持控制命令语义一致：`/new`、`/resume`、`/list`、`/current`、`/switch`、`/cancel`。
- 满足签名/鉴权、去重、结构化日志三项最低安全要求。

## 接入模式
- 入站协议：`webhook | socket | bridge`
- 出站协议：`channel sdk | bridge api`
- 运行模式：`single-instance | multi-instance`

## 配置模型
- 在 `internal/infrastructure/config/config.go` 增加 `<Channel>Instance`。
- 字段最小集：`enabled/defaultAgent/instances`。
- 保证 `clawx config channel <name>` 为增量更新，非目标渠道不覆盖。

## 适配层落点
- 新建：`internal/interfaces/chat/<channel>/adapter.go`
- 要求：
  - 入站消息归一化到 `chatiface.Message`
  - 非法请求拒绝并记录结构化日志
  - 正常消息进入统一 Router

## 测试三联
- unit: `tests/unit/channels/waveX/<channel>_adapter_test.go`
- integration: `tests/integration/channels/waveX/<channel>_control_flow_test.go`
- contract: `tests/contract/channels/waveX/<channel>_contract_test.go`

## 发布门禁
- 通过测试三联后，才允许进入实现阶段。
- 任何绕过安全校验的行为必须在设计评审中明确否决。
