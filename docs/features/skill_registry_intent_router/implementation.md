# 实现方案

## 模块划分（DDD 对齐）
- `internal/domain/skill`
- Skill 元数据、校验规则、冲突规则。
- `internal/application/skillregistry`
- 扫描、索引、启停、缓存刷新。
- `internal/application/intent`
- 路由决策与优先级编排。
- `internal/interfaces/channel`
- 接入 Discord/Telegram 消息，调用 Router。
- `internal/infrastructure/skills`
- 文件系统读取与索引落盘。

## 配置结构（config.json）
```json
{
  "skills": {
    "enabled": true,
    "sources": {
      "user_dir": "~/.synapsex/skills",
      "workspace_dir": ".synapsex/skills",
      "builtin_enabled": true
    },
    "disabled_names": [],
    "allowlist": {
      "users": [],
      "channels": []
    }
  },
  "intent_router": {
    "mode": "rule_first_llm_fallback",
    "llm_fallback": {
      "enabled": true,
      "confidence_threshold": 0.72
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
- `channel`
- `agent_id`
- `intent.kind`
- `intent.skill_name`
- `intent.reason`
- `intent.confidence`
- `session_id`
- `duration_ms`
