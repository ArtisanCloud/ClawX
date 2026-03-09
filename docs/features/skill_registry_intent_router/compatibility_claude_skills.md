# Claude Skill 兼容规范

## 兼容原则
- 采用 Claude Code Skill 约定，不定义新格式。
- SynapseX 只负责加载、路由、权限与执行注入。

## 必须支持
1. Skill 根目录 `SKILL.md`
2. frontmatter 字段：
- `name`
- `description`
3. Markdown body 作为技能执行上下文指令。

## 推荐支持
- `references/`：按需加载的参考文档。
- `scripts/`：可执行脚本资源。
- `assets/`：模板与素材资源。

## 不做的兼容扩展（本阶段）
- 不强制依赖额外元数据文件。
- 不修改第三方 Skill 原文件内容。
- 不要求 Skill 为 SynapseX 专有字段。

## 冲突处理
- 技能名与系统保留命令冲突时，Registry 标记为 `invalid` 并告警。
- 同名技能按 source 优先级只激活一个，其他记录为 `shadowed`。
