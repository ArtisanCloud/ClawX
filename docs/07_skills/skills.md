# Skills 规划（V1）

V1 明确不做通用 Skill/Plugin 框架，仅保留后续扩展位：

- 占位接口：Channel Adapter / Session Manager / CLI Adapter 均保持可替换性。
- 若未来接 PowerX 或 Tool Skill，可在 Router 层新增指令分流，不影响现有 Codex 流程。
- 代码中避免耦合具体模型或第三方插件，接口以最小必要参数定义（command, cwd, timeout, session_id）。
