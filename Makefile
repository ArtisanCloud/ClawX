DB_HOST ?= 127.0.0.1
DB_PORT ?= 5432
DB_USER ?= clawx
DB_PASSWORD ?=
DB_NAME ?= claw_x
DB_SSLMODE ?= disable

MIGRATIONS_DIR ?= database/migrations
SEEDS_DIR ?= database/seeds

PSQL = PGPASSWORD="$(DB_PASSWORD)" psql -v ON_ERROR_STOP=1 -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" -d "$(DB_NAME)"

.PHONY: help db-create db-drop db-migrate db-seed db-refresh db-status

help:
	@echo "Available targets:"
	@echo "  make db-create    - create database if not exists"
	@echo "  make db-drop      - drop database if exists"
	@echo "  make db-migrate   - apply SQL files from $(MIGRATIONS_DIR)"
	@echo "  make db-seed      - apply SQL files from $(SEEDS_DIR)"
	@echo "  make db-refresh   - drop + create + migrate + seed"
	@echo "  make db-status    - list tables in current database"
	@echo ""
	@echo "Override vars: DB_HOST DB_PORT DB_USER DB_PASSWORD DB_NAME DB_SSLMODE"

db-create:
	@set -e; \
	echo "Ensuring database $(DB_NAME) exists..."; \
	PGPASSWORD="$(DB_PASSWORD)" psql -v ON_ERROR_STOP=1 -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" -d postgres -tc "SELECT 1 FROM pg_database WHERE datname='$(DB_NAME)'" | grep -q 1 || \
	PGPASSWORD="$(DB_PASSWORD)" createdb -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" "$(DB_NAME)"

db-drop:
	@set -e; \
	echo "Dropping database $(DB_NAME) if exists..."; \
	PGPASSWORD="$(DB_PASSWORD)" dropdb --if-exists -h "$(DB_HOST)" -p "$(DB_PORT)" -U "$(DB_USER)" "$(DB_NAME)"

db-migrate:
	@set -e; \
	count=$$(ls "$(MIGRATIONS_DIR)"/*.sql 2>/dev/null | wc -l); \
	if [ "$$count" -eq 0 ]; then \
		echo "No migration files found in $(MIGRATIONS_DIR)"; \
		exit 0; \
	fi; \
	for file in $$(ls "$(MIGRATIONS_DIR)"/*.sql | sort); do \
		echo "Applying migration: $$file"; \
		$(PSQL) -f "$$file"; \
	done

db-seed:
	@set -e; \
	count=$$(ls "$(SEEDS_DIR)"/*.sql 2>/dev/null | wc -l); \
	if [ "$$count" -eq 0 ]; then \
		echo "No seed files found in $(SEEDS_DIR)"; \
		exit 0; \
	fi; \
	for file in $$(ls "$(SEEDS_DIR)"/*.sql | sort); do \
		echo "Applying seed: $$file"; \
		$(PSQL) -f "$$file"; \
	done

db-refresh: db-drop db-create db-migrate db-seed
	@echo "Database refresh completed for $(DB_NAME)"

db-status:
	@$(PSQL) -c "\dt"
