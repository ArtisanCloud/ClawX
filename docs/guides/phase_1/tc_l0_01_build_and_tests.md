# TC-L0-01 编译与单测通过

## 目标
- 确认代码基线可运行。

## 步骤
1. 在仓库根目录执行：`go env -w GOTOOLCHAIN=local`
2. 执行：`go test ./...`

## 预期
1. 所有包编译通过。
2. `tests/unit`、`tests/integration`、`tests/contract` 为 `ok`。
