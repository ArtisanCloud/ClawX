# 实现状态（已落地）

## 模块落地
- `internal/domain/skill`:
  - Skill 定义、状态、快照、意图决策模型。
- `internal/application/skillregistry`:
  - 发现/解析/冲突处理/刷新编排。
  - `list/reload/enable/disable` 所需服务接口。
- `internal/application/intent`:
  - 固定优先级路由流水线。
  - 显式 `/skill`、规则匹配、LLM 兜底、多候选自动单选。
  - 权限评估与错误分类。
- `internal/infrastructure/skills`:
  - `SKILL.md` frontmatter 解析。
  - 多来源扫描。
  - 索引原子写入与 DM pairing 文件存储。
- `cmd/synapsex`:
  - 新增 `skill` CLI 子命令。
  - 启动时注入 Skill Registry + Intent Router。
  - Discord/Telegram 入站分支支持 `kind=skill`。

## 配置结构（config.json）
```json
{
  "skills": {
    "enabled": true,
    "sources": {
      "userDir": "~/.synapsex/skills",
      "workspaceDir": ".synapsex/skills",
      "builtinEnabled": true,
      "builtinDir": "internal/skills/builtin"
    },
    "disabledNames": [],
    "allowlist": {
      "users": [],
      "channels": []
    },
    "defaultMode": "channel_allowlist_dm_pairing",
    "pairingTTLSeconds": 604800
  },
  "intentRouter": {
    "mode": "rule_first_llm_fallback",
    "llmFallback": {
      "enabled": true,
      "confidenceThreshold": 0.72
    }
  }
}
```

## CLI 命令
- `synapsex skill list`
- `synapsex skill reload`
- `synapsex skill enable <name>`
- `synapsex skill disable <name>`

## 日志字段
- `conversation_id`
- `intent.kind`
- `intent.reason`
- `intent.skill`
- `intent.confidence`
- `session_id`
- `duration_ms`
