# TC-L3-02 频道内对话与可选 Agent 切换

## 目标
- 先验证 `main` agent 可在 channel 内持续对话。
- 在具备第二个 agent 时，再验证切换。

## 前置
1. 至少已有 `main` agent（默认即有）。
2. `main` 对应 profile 可执行（例如 `codex`）。
3. 若要做“切换”部分，再额外准备第二个 agent（例如 `reviewer`）。

## 步骤 A（必做：main 对话）
1. 发送：`/agent current`
2. 发送普通开发指令（例如：`请列出当前目录并解释每个文件用途`）

## 预期 A
1. 未显式切换时，使用默认路由（通常是 `main`）。
2. 普通开发指令可被执行并返回结果。

## 步骤 B（可选：切换 agent）
1. 发送：`/agent list`
2. 发送：`/agent use reviewer`
3. 发送：`/agent current`
4. 发送普通开发指令
5. 发送：`/agent clear`
6. 再发送普通开发指令

## 预期 B
1. `list` 可看到可用 agent。
2. `use` 成功后，`current` 显示已固定到 `reviewer`。
3. 第一次普通开发指令由 `reviewer` 对应 profile/workspace 执行。
4. `clear` 后恢复默认路由。
