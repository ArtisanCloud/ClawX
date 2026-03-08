# Phase 2 - Multi Session

## 阶段目标
- 让“多窗口 = 多 Session”成为默认交互模型。
- 在不引入 `Agent` 复杂度的前提下，让同一用户可同时持有多个独立会话。
- 保持 Phase 1 已完成的直连执行链路不变，只在其上增加窗口绑定、会话选择和切换能力。

## 阶段定位
- Phase 1 已解决：单窗口单会话、最短执行链路、基础控制流、输出分段、骨架级渠道接入。
- Phase 2 要解决：用户如何稳定地同时管理多个会话，并明确“当前输入到底进入哪个 Session”。
- Phase 2 不解决：运行环境隔离、`Agent Registry`、入口级绑定策略；这些仍属于 Phase 3。

## 核心结论
- `Session` 继续作为上下文隔离主单位。
- `Window Context` 是交互容器，不等于必须先做完整 UI 窗口。
- 一个 `Window Context` 在任一时刻只绑定一个当前 Session。
- 一个用户可以同时拥有多个 `Window Context`，每个上下文可绑定不同 Session。
- 路由默认行为应从“按 conversation 自动落最近 Session”升级为“窗口优先，再回退旧逻辑”。

## 第二阶段的完成定义
以下 5 项同时满足，才能认为 Phase 2 完成：
- 调用方可以显式传入 `window_id`，系统会按窗口绑定会话。
- 同一用户可在多个 `window_id` 下继续不同会话，互不串线。
- 当前窗口可以查看会话列表并切换当前会话。
- 未传入 `window_id` 时，仍保留 Phase 1 的兼容行为。
- `go test ./...` 通过，且新增多窗口场景测试覆盖核心路径。

## 范围
- `Window Context`
- `window_id -> current_session_id` 绑定
- 窗口级最近使用会话
- 窗口级会话列表
- 当前窗口会话切换
- 无 `window_id` 时的兼容回退
- 多窗口、多会话的测试与验证

## 不做
- `Agent Registry`
- 复杂 `Binding` 规则引擎
- 运行环境隔离
- 多租户权限系统
- 新渠道扩展
- 完整 Web 管理界面
- 渠道侧复杂窗口 UI 设计

## 开发前提
- Phase 1 代码骨架已完成，并通过基础 `go test ./...`
- 当前主链路仍是：
  - `Router -> Session Manager -> Backend Adapter`
- 当前代码中尚未把 `window_id` 作为一等路由条件
- Phase 2 必须在不破坏以下行为的前提下推进：
  - 同一 Session 串行执行
  - 超时语义不变
  - 取消语义不变
  - 输出分段与错误映射不回退

## 开发入口文档
进入 Phase 2 前，先统一以下文档口径：
- `docs/plans/roadmap.md`
- `docs/features/multi_agent/overview.md`
- `docs/features/multi_agent/architecture.md`
- `docs/01_concepts/concepts.md`
- `docs/02_architecture/architecture.md`
- `docs/plans/persistence_strategy.md`

阅读目标：
- 明确 Phase 2 只做多窗口、多会话，不提前把 `Agent` 变成新前置概念。
- 明确 Phase 2 是对 Phase 1 的路由行为升级，不是推翻现有执行链路。

## 当前代码基线
第二阶段改造必须围绕现有代码展开，不新增无关层级：
- 路由入口：
  - `internal/application/service/router.go`
  - `internal/application/service/router_session_flow.go`
  - `internal/application/service/router_control_flow.go`
- 会话管理：
  - `internal/application/service/session_manager.go`
  - `internal/application/service/session_manager_create.go`
  - `internal/application/service/session_manager_resume.go`
  - `internal/application/service/session_manager_continue.go`
  - `internal/application/service/session_manager_list.go`
  - `internal/application/service/session_manager_cancel.go`
- 领域模型：
  - `internal/domain/session/session.go`
  - `internal/domain/session/repository.go`
- 持久化存储（本地文件优先）：
  - `internal/infrastructure/persistence/session_file_repository.go`
  - `internal/infrastructure/persistence/session_memory_repository.go`（缓存与锁）
- 渠道消息模型：
  - `internal/interfaces/chat/message.go`
  - `internal/interfaces/chat/normalize.go`

Phase 2 的重点不是再开新骨架，而是把上述模块扩展到多窗口可用。

## 术语定义

### Window Context
- 一个用户当前的交互容器。
- 它可以是：
  - 未来 Web UI 的一个标签页或会话面板
  - 本地调试中的一个临时上下文标识
  - 某个渠道中被显式区分的独立输入入口
- Phase 2 不要求先做完整 UI，但要求代码层有 `window_id` 这个一等概念。

### Current Session
- 某个 `window_id` 当前绑定的 Session。
- 新输入默认进入这个 Session，除非用户明确指定：
  - 新建会话
  - 恢复指定会话
  - 切换到其他会话

### Recent Sessions
- 当前窗口最近使用过的会话列表。
- 用于恢复、切换和展示“当前上下文”。

### Window Binding
- 一个独立于 `Session` 的绑定记录。
- 用于表示“当前这个窗口现在指向哪个会话”，而不是把所有状态都塞进会话实体。

## 产品能力要求

### 1. 窗口与会话绑定
- 每个 `window_id` 必须能绑定一个当前 Session。
- 当窗口首次出现且无绑定时：
  - 可以新建 Session
  - 或在显式恢复时绑定已有 Session
- 切换窗口时，不得错误复用其他窗口的当前 Session。

### 2. 多窗口并行持有
- 同一用户应可同时持有多个窗口上下文。
- 每个窗口的当前 Session 独立。
- 窗口 A 的操作不得污染窗口 B 的当前 Session 指针。

### 3. 会话列表与切换
- 系统必须能列出当前窗口可见的 Session。
- 系统必须能明确切换当前窗口的 `current_session_id`。
- 切换后，后续自然语言输入默认进入新的当前会话。

### 4. 最近使用顺序
- 系统应维护窗口级最近使用顺序。
- 最近使用时间应在以下动作后刷新：
  - 新建
  - 恢复
  - 切换
  - 成功执行

### 5. 向后兼容
- 当调用方暂时没有传入 `window_id` 时：
  - 必须仍可回退到 Phase 1 的最小行为
  - 但内部逻辑应尽量统一到窗口模型
- Phase 2 是行为升级，不是把现有主链路推翻重写。

## 数据模型变化

### Session 字段调整
- `Session.window_id`
  - 从预留字段变为真正使用
  - 表示会话最近一次被绑定到的窗口上下文
- `Session.last_used_at`
  - 若当前尚未作为明确排序依据，Phase 2 需要把它用于最近会话排序

### 新增结构：Window Binding
- 建议新增显式结构：
  - `window_id`
  - `conversation_id`
  - `current_session_id`
  - `last_used_at`

### 查询视角
- 按 `window_id` 查询当前 Session
- 按 `window_id` 列出窗口下最近会话
- 按 `conversation_id` 查询该 conversation 下的窗口绑定

## 建议接口变化

### Session Repository
Phase 2 至少需要新增以下能力：
- `GetCurrentSessionByWindow(windowID string)`
- `SetCurrentSessionForWindow(windowID string, sessionID string)`
- `ListSessionsByWindow(windowID string)`
- `ListRecentSessionsByWindow(windowID string, limit int)`

若现有 `SessionRepository` 不适合直接承载全部窗口行为，可以拆为：
- `SessionRepository`
- `WindowBindingRepository`

但不要为了 Phase 2 引入过度抽象；只要边界清楚即可。

### Router
现有 `Router` 需要升级为优先读取 `window_id`。

建议路由优先级：
1. 显式控制命令指定的 Session
2. 当前窗口绑定的 Session
3. `conversation_id` 下最近会话
4. 若都不存在，则新建

### Session Manager
除现有会话逻辑外，需要新增：
- 按窗口获取当前 Session
- 更新窗口当前 Session
- 列出窗口下会话
- 切换窗口当前 Session
- 在 `new / resume / continue` 后刷新窗口绑定

### Chat Message
`internal/interfaces/chat/message.go` 应把 `window_id` 升级为常规输入字段，而不是可忽略附带信息。

## 控制命令扩展建议
Phase 2 不需要大改命令体系，但应补齐窗口语义：
- `/new`
  - 在当前窗口创建新会话，并把该会话设为当前会话
- `/resume <session_id>`
  - 恢复指定会话，并把它设为当前窗口当前会话
- `/list`
  - 默认列出当前窗口可见的会话，并标记当前会话
- `/cancel`
  - 仍针对当前窗口的当前会话

建议新增：
- `/switch <session_id>`
  - 只切换当前窗口的 `current_session_id`
  - 不立即执行，不隐式生成新输入

如果本阶段不新增 `/switch`，则必须明确：
- `resume` 是否同时承担“恢复 + 切换”语义
- 文档和测试必须保持一致，不能出现双重定义

## 模块改造清单

### 1. 领域层
- `internal/domain/session/session.go`
  - 让 `window_id` 成为真实字段
  - 明确合法状态与更新时间规则
- `internal/domain/session/repository.go`
  - 增加窗口级查询与绑定接口

### 2. 应用层
- `internal/application/service/session_manager.go`
  - 增加窗口绑定读写能力
- `internal/application/service/session_manager_create.go`
  - 新建会话后写入窗口绑定
- `internal/application/service/session_manager_resume.go`
  - 恢复时刷新窗口绑定
- `internal/application/service/session_manager_continue.go`
  - 优先使用窗口当前会话
- `internal/application/service/session_manager_list.go`
  - 支持窗口级列表与当前会话标记
- `internal/application/service/router.go`
  - 把 `window_id` 作为显式输入条件
- `internal/application/service/router_control_flow.go`
  - 控制命令需要更新窗口当前会话指针
- `internal/application/service/router_session_flow.go`
  - 自然语言输入默认进入窗口当前会话

### 3. 基础设施层
- `internal/infrastructure/persistence/session_file_repository.go`
  - 持久化 `window_id -> current_session_id`
  - 持久化最近会话查询与排序基础数据
  - 管理 `sessions.json` 与 `*.jsonl` 的写入一致性
- `internal/infrastructure/persistence/session_memory_repository.go`
  - 保留运行态锁与短时缓存

### 4. 接口层
- `internal/interfaces/chat/message.go`
  - 规范 `window_id`
- `internal/interfaces/chat/normalize.go`
  - 若外部输入没有 `window_id`，按兼容规则生成默认值

## 兼容策略
为避免破坏 Phase 1 调用方，按以下顺序兼容：
- 若显式传入 `window_id`，走 Phase 2 路由。
- 若未传入 `window_id`，内部生成兼容值：
  - 优先可使用 `conversation_id`
  - 仅用于回退，不视为长期替代方案
- 若没有窗口绑定记录，回退到 Phase 1 最近会话逻辑。

这意味着：
- Phase 2 不要求所有入口马上都升级到真实窗口模型。
- 但内部实现必须先具备真实窗口模型，否则后续 Phase 3 会被卡住。

## 建议开发顺序

### 第一步：补齐模型与仓储接口
- 明确 `window_id` 的生命周期和合法性
- 为窗口绑定新增最小存储接口
- 在文件仓储中实现窗口绑定持久化

### 第二步：改 Session Manager
- 把“按窗口获取当前 Session / 切换 Session / 刷新最近使用”做成明确方法

### 第三步：改 Router
- 让路由决策从“conversation 最近会话”切换为“窗口优先，旧逻辑回退”

### 第四步：补控制命令
- 让 `/new`、`/resume`、`/list`、`/cancel` 都具备窗口语义
- 若决定增加 `/switch`，在此步一并落地

### 第五步：补测试
- 补多窗口并发场景测试
- 补窗口切换不串线测试
- 补无 `window_id` 回退兼容测试

### 第六步：补验证文档
- 在 `docs/guides/` 增加第二阶段的验证说明
- 明确哪些是代码级验证，哪些是运行时可操作验证

## 测试矩阵
第二阶段至少补齐以下测试：

### 单元测试
- 同一用户多个窗口绑定不同会话时，绑定结果正确
- 切换窗口当前会话时，旧窗口不受影响
- 无 `window_id` 时，回退逻辑仍返回可用会话

### 集成测试
- 窗口 A / B 分别继续不同会话，不串线
- 同一窗口从会话 1 切换到会话 2 后，后续输入进入会话 2
- `/resume` 后当前窗口绑定被刷新
- `/cancel` 只影响当前窗口当前会话

### 回归测试
- Phase 1 的单窗口单会话路径不回退
- 同一 Session 串行约束不被破坏
- 超时与取消行为不回退

## 实施约束
- 不允许把窗口逻辑直接塞进渠道适配器
- 不允许为实现多会话而提前引入 `Agent`
- 不允许把窗口绑定状态分散到多个无主结构中
- 不允许破坏 Phase 1 已完成的单会话路径

## 风险与处理

### 风险 1：窗口概念与渠道概念混淆
- 风险：把 `conversation_id` 直接当唯一窗口模型，会让后续 Web/UI 扩展受限。
- 处理：允许 Phase 2 初期兼容映射，但内部仍保留独立 `window_id` 概念。

### 风险 2：旧行为回退不兼容
- 风险：已有 Phase 1 调用方没有 `window_id`，直接改路由会导致行为突变。
- 处理：保持缺省回退逻辑，先兼容再逐步推动调用方传入真实 `window_id`。

### 风险 3：切换与恢复语义混淆
- 风险：`resume`、`switch`、`continue` 可能被实现成同一件事，导致状态不清晰。
- 处理：明确三者差异：
  - `continue`: 用当前窗口绑定的 Session 继续执行
  - `resume`: 指定恢复一个已有 Session，并将其绑定为当前窗口当前会话
  - `switch`: 仅改变窗口的当前会话指针，不触发执行

### 风险 4：最近会话排序失真
- 风险：只在执行成功时更新时间，会导致切换后列表不符合用户预期。
- 处理：新建、恢复、切换、成功执行都应刷新最近使用时间。

## 里程碑拆分

### M2-1：窗口模型入库
- 仓储接口与文件持久化实现支持窗口绑定
- `window_id` 成为有效字段

### M2-2：路由切换完成
- Router 已按“窗口优先，旧逻辑回退”生效
- `new / resume / continue` 能正确刷新窗口绑定

### M2-3：控制流闭环
- `list / cancel` 具备窗口语义
- 若纳入 `/switch`，命令行为清晰稳定

### M2-4：测试与文档闭环
- 多窗口场景测试覆盖到位
- 验证指南补齐
- 文档能直接支撑进入 Phase 3

## 验收标准
- 用户可同时持有多个窗口上下文
- 每个窗口可持有不同 Session
- 切换窗口不会串线
- 切换当前会话后，后续自然语言请求进入正确 Session
- 不传 `window_id` 时，旧的单会话行为仍可工作
- `go test ./...` 通过
- 新增多窗口测试覆盖以下场景：
  - 多窗口独立绑定
  - 同窗口切换 Session
  - 不同窗口同时继续各自会话
  - 缺省回退逻辑

## 开发完成后的输出物
- Phase 2 代码实现
- Phase 2 测试补充
- `docs/guides/phase_2_validation.md`
- 如需进入 spec 流程：
  - `specs/002-phase2-multi-session/` 对应规格、计划、任务文档

## 完成标志
- `window_id` 已从预留字段升级为真实运行字段
- `Router` 已按窗口优先决策
- `Session Manager` 已具备窗口绑定能力
- 测试已覆盖多窗口多会话核心场景
- 文档已更新到能支持 Phase 3 在此基础上引入轻量 `Agent`
