# 008 Skill Package Export 计划草案（v0.1）

## 0. 相关文档
- 统一定时机制规划：`docs/plans/feature/007-unified-scheduler-center/plan.md`
- Memory Context Layer：`docs/plans/feature/006-memory-context-layer/plan.md`

## 1. 背景
当前能力主要以“项目代码 + ClawX 运行时”方式交付，缺少标准化“能力导出”机制。导致同一能力迁移到其他平台（Codex、Claude Code、其他代理运行时）时，需要重复改造。

## 2. 目标
建设可复用的技能导出机制，支持：
- 将项目能力导出为可分发 Skill 包。
- 支持多平台适配产物（先 Codex，再 Claude/其他）。
- 提供一致的导出清单、版本、依赖声明和验收脚本。

## 3. 范围与非目标
### 3.1 本期范围（MVP）
- 定义 Skill 包目录规范与 manifest。
- 提供 `clawx export-skill` 命令（或等效导出入口）。
- 支持 Codex 平台导出产物。
- 提供“图片工具”示例导出与验证流程。

### 3.2 非目标（本期不做）
- 一次性兼容所有第三方平台的完整运行时差异。
- 自动发布到外部市场。
- 远程仓库托管与签名分发体系。

## 4. 产物设计
### 4.1 导出目录建议
```text
exports/skills/<skill_name>/<version>/
├── SKILL.md
├── manifest.json
├── prompts/
├── scripts/
├── assets/
└── checks/
```

### 4.2 核心元数据
- skill_id、name、version
- target_platform（codex / claude / generic）
- required_runtime（可执行依赖）
- input_contract / output_contract
- install_steps / verify_steps

## 5. 命令与流程（MVP）
- `clawx export-skill --name <skill> --target codex --out <dir>`
- `clawx export-skill --name <skill> --target claude --out <dir>`（P2）
- `clawx verify-skill --path <export_dir>`

最小流程：
1. 从当前项目加载能力声明与依赖。
2. 生成目标平台模板文件。
3. 输出 manifest 与校验脚本。
4. 执行导出后验证并给出报告。

## 6. 实施阶段
### Phase A：规范与模型（P1）
- 定义 skill export manifest 与版本策略。
- 定义平台适配接口（adapter contract）。

### Phase B：Codex 导出实现（P1）
- 新增导出命令与 codex adapter。
- 产出 `SKILL.md + manifest + checks`。

### Phase C：验证与示例（P1）
- 以图片工具能力做首个导出样例。
- 增加验收脚本与回归测试。

### Phase D：多平台扩展（P2）
- 增加 Claude/其他平台 adapter。
- 建立兼容矩阵与差异提示。

## 7. 验收标准（MVP）
- 用户可在单命令下导出 Codex 可用 Skill 包。
- 导出产物包含可安装说明、依赖声明、校验脚本。
- 示例 Skill 可在干净环境中完成安装与最小功能验证。
- 导出失败时提供可行动错误信息。

## 8. 风险与缓解
- 风险：平台规范差异导致“通用包”失效。
  - 缓解：核心能力与平台 adapter 分离，按目标平台输出差异化产物。
- 风险：依赖不完整导致迁移失败。
  - 缓解：导出时强制生成依赖检查清单并执行 verify。
- 风险：版本漂移造成不可复现。
  - 缓解：manifest 固化版本与校验摘要。

## 9. 下一步
建议在独立分支启动该功能实现，先交付 Codex 导出 MVP，再扩展到 Claude/其他平台。
