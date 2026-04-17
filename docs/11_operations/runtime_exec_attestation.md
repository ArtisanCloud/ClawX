# Runtime Exec 审计字段说明（`runtime_exec.jsonl`）

## 1. 日志位置
- 默认文件：`~/.clawx/logs/runtime_exec.jsonl`
- 可通过环境变量覆盖：`CLAWX_RUNTIME_EXEC_LOG_FILE`

每一行是一个 JSON 记录，对应一次 shell 执行（含 Lead Worker 执行）。

## 2. 核心字段
- `timestamp`：记录写入时间（UTC）。
- `exec_id`：本次执行唯一 ID（用户回执里的“执行凭证”）。
- `conversation_id`：会话唯一键，用于跨回合追踪。
- `agent_id`：执行所属 agent。
- `cwd`：执行目录。
- `command`：实际执行命令（包含改写后的命令）。
- `plan_reason`：计划层原因（例如 `runtime_lead_worker_dispatch`）。
- `step_reason`：步骤层原因（例如 `runtime_exec_decision_build_continue_fallback`）。
- `exit_code`：退出码（当前实现成功=0，失败=1）。
- `duration_ms`：执行耗时（毫秒）。
- `success`：是否成功。
- `output_digest`：输出摘要（SHA-256）。
- `output_preview`：输出预览（截断）。
- `error_summary`：错误摘要（失败时）。

## 3. 决策来源字段（重点）
- `runtime_exec_decision_mode`
  - 本次执行应用的决策模式。
  - 常见值：`service.retry_only`、`service.deep_repair`、`build.continue`、`build.switch_source`、`blocked.command_override`。
- `runtime_exec_decision_apply_source`
  - 本轮“应用该模式”的来源。
  - 常见值：`user_phrase`、`persisted_state`、`none`。
- `runtime_exec_decision_lock_source`
  - 当前锁定策略的来源（谁锁定了模式）。
  - 常见值：`user_phrase`、`persisted_state`、`none`。
- `runtime_exec_decision_source`
  - 兼容字段，当前与 `runtime_exec_decision_lock_source` 保持一致。
- `runtime_exec_decision_fallback_source`
  - 当命令由兜底策略注入时，记录兜底命令来源。
  - 常见值：`env`、`workspace`、`default`。

## 4. 兜底来源优先级
- 决策兜底命令来源优先级：`env > workspace > default`。
- 当一轮执行包含多条命令，用户回执会聚合为：
  - `执行来源：mode=... apply_source=... lock_source=... fallback_source=env,workspace`
- 该行可与 `runtime_exec.jsonl` 中的字段一一对应。

## 5. 快速排障
- 查看最近 20 条：
```bash
tail -n 20 ~/.clawx/logs/runtime_exec.jsonl
```

- 只看某会话最近 10 条：
```bash
tail -n 2000 ~/.clawx/logs/runtime_exec.jsonl \
  | jq -c 'select(.conversation_id=="<conversation_id>")' \
  | tail -n 10
```

- 观察决策来源链路：
```bash
tail -n 2000 ~/.clawx/logs/runtime_exec.jsonl \
  | jq -r '[.exec_id,.runtime_exec_decision_mode,.runtime_exec_decision_apply_source,.runtime_exec_decision_lock_source,.runtime_exec_decision_fallback_source,.success] | @tsv'
```

## 6. 用户回执映射
- 当系统触发 `execution_blocker`（需要用户确认）时，用户可见回执会包含：
  - `执行来源：mode=... apply_source=... lock_source=... fallback_source=...`
- 在状态查询类回执（如 `runtime.task.status`、`runtime.task.delegates`、`runtime.task.control status`、`runtime.release.status`）中，也会输出同一行。
- 在发布动作回执 `runtime.release` 中，同样会携带该行，便于解释“本次发布为何按当前策略执行”。
- 在动作失败回执（例如 `runtime.release|runtime.service|runtime.task.*|runtime.supervisor|runtime.bootstrap|config.exec|requirement.sync|agent.use|action.kind|action_plan 执行失败`）中，也会携带该行，便于定位“失败发生在何种自治策略下”。
- 在非失败但“需解释上下文”的回执（例如 `agent.use` 的 `applied/noop`、`requirement.sync` 的 `applied/suggest/skipped`、`config.exec` 的 `applied`）中，也会携带该行。
- 该行由当前会话关联的 attestation 聚合生成，用于直接解释“这次自治为什么这么执行”。
