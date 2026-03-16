# PostgreSQL 可选接入（非默认路径）

## 适用场景
- 你明确需要把 Session 元数据写入外部数据库。
- 你接受更高的部署和运维复杂度。

不适用场景：
- 第一阶段主线联调。
- 只想快速跑通 Discord/Telegram -> Codex 链路。

## 说明
- 项目当前默认方向是轻量持久化（本地文件优先）。
- PostgreSQL 作为可选扩展保留，不作为主线验收前置。

## 1. 安装 PostgreSQL（Ubuntu）
```bash
sudo apt-get update
sudo apt-get install -y postgresql postgresql-contrib
```

## 2. 启动并设置开机自启
```bash
sudo systemctl enable --now postgresql
sudo systemctl status postgresql --no-pager
```

## 3. 创建业务账号与密码（示例）
```bash
sudo -u postgres psql -c "CREATE ROLE clawx LOGIN PASSWORD 'your_password';"
sudo -u postgres psql -c "ALTER ROLE clawx CREATEDB;"
```

## 4. 可选：手动创建数据库
```bash
sudo -u postgres createdb -O clawx claw_x
```

## 5. 验证账号可连通
```bash
PGPASSWORD=your_password psql -h 127.0.0.1 -p 5432 -U clawx -d postgres -c "SELECT 1;"
```

## 6. 在 ClawX 配置中启用数据库
运行：

```bash
go run ./cmd/clawx
```

数据库相关建议填写：
- `启用 PostgreSQL 持久化 Session`: `是`
- `Host`: `127.0.0.1`
- `Port`: `5432`
- `Database Name`: `claw_x`
- `Database User`: `clawx`
- `Database Password`: 你的密码
- `SSL Mode`: `disable`（本机联调常用）
- `启动时自动创建数据库并执行迁移`: `是`

## 7. 启动后验收
- 启动日志应包含：`session store initialized: driver=postgres ...`
- 发送 `/new` 后，数据库里应有会话数据：

```bash
PGPASSWORD=your_password psql -h 127.0.0.1 -p 5432 -U clawx -d claw_x -c "SELECT id,conversation_id,status,last_used_at FROM clawx_sessions ORDER BY last_used_at DESC LIMIT 5;"
```

## 8. 可选：使用 Make 执行迁移/种子/刷新

```bash
make db-create DB_USER=clawx DB_PASSWORD=your_password
make db-migrate DB_USER=clawx DB_PASSWORD=your_password
make db-seed DB_USER=clawx DB_PASSWORD=your_password
make db-refresh DB_USER=clawx DB_PASSWORD=your_password
make db-status DB_USER=clawx DB_PASSWORD=your_password
```
