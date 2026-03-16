# WhatsApp 渠道实现卡

## 状态
- 波次：`Wave 2`
- 当前状态：`spec-ready`（技术规范与测试骨架已就位，代码实现未开始）
- 契约：`specs/004-channels/contracts/whatsapp-event-contract.md`

## 目标
- 将 WhatsApp 接入统一渠道框架，不引入专属执行链路。
- 满足统一控制命令语义与会话窗口隔离要求。
- 完成测试三联后，再进入适配器实现。

## 最小实现清单
- 适配层：`internal/interfaces/chat/whatsapp/adapter.go`
- 启动注册：`cmd/clawx/main.go`
- 配置模型：`internal/infrastructure/config/config.go`
- 增量配置：`cmd/clawx/config_channel.go`

## 测试三联
- unit：`tests/unit/channels/wave2/whatsapp_adapter_test.go`
- integration：`tests/integration/channels/wave2/whatsapp_control_flow_test.go`
- contract：`tests/contract/channels/wave2/whatsapp_contract_test.go`

## 验收门禁
- 安全校验覆盖（签名/鉴权/重放）
- 控制命令语义一致性覆盖
- 非目标渠道配置不被覆盖
