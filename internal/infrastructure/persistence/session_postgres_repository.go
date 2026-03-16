package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"

	"clawx/internal/domain/session"
)

type PostgresConfig struct {
	Host       string
	Port       int
	Database   string
	User       string
	Password   string
	SSLMode    string
	AutoCreate bool
}

type PostgresSessionRepository struct {
	db *sql.DB
}

func NewPostgresSessionStore(ctx context.Context, cfg PostgresConfig) (*sql.DB, *PostgresSessionRepository, error) {
	cfg = normalizePostgresConfig(cfg)

	if cfg.AutoCreate {
		if err := ensurePostgresDatabase(ctx, cfg); err != nil {
			return nil, nil, err
		}
	}

	db, err := sql.Open("postgres", buildPostgresDSN(cfg, cfg.Database))
	if err != nil {
		return nil, nil, fmt.Errorf("open postgres session db: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("ping postgres session db: %w", err)
	}

	repo := &PostgresSessionRepository{db: db}
	migrateCtx, migrateCancel := context.WithTimeout(ctx, 8*time.Second)
	defer migrateCancel()
	if err := repo.Migrate(migrateCtx); err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("migrate postgres session db: %w", err)
	}

	return db, repo, nil
}

func (r *PostgresSessionRepository) Migrate(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS clawx_sessions (
			id TEXT PRIMARY KEY,
			window_id TEXT NOT NULL DEFAULT '',
			agent_id TEXT NOT NULL DEFAULT '',
			backend TEXT NOT NULL,
			backend_session_id TEXT NOT NULL DEFAULT '',
			conversation_id TEXT NOT NULL,
			cwd TEXT NOT NULL,
			status TEXT NOT NULL,
			lock_token TEXT NOT NULL DEFAULT '',
			last_used_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clawx_sessions_conversation_last_used
			ON clawx_sessions (conversation_id, last_used_at DESC)`,
		`CREATE TABLE IF NOT EXISTS clawx_window_bindings (
			window_id TEXT PRIMARY KEY,
			current_session_id TEXT NOT NULL,
			conversation_id TEXT NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			last_used_at TIMESTAMPTZ NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_clawx_window_bindings_conversation
			ON clawx_window_bindings (conversation_id, last_used_at DESC)`,
	}
	for _, stmt := range statements {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresSessionRepository) Create(ctx context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO clawx_sessions (
			id, window_id, agent_id, backend, backend_session_id,
			conversation_id, cwd, status, lock_token, last_used_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
	`,
		record.ID,
		record.WindowID,
		record.AgentID,
		record.Backend,
		record.BackendSessionID,
		record.ConversationID,
		record.CWD,
		string(record.Status),
		record.LockToken,
		record.LastUsedAt.UTC(),
	)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr.Code == "23505" {
			return fmt.Errorf("create session %s: %w", record.ID, session.ErrInvalidSession)
		}
		return err
	}
	return nil
}

func (r *PostgresSessionRepository) Save(ctx context.Context, record session.Record) error {
	if err := record.Validate(); err != nil {
		return err
	}

	result, err := r.db.ExecContext(ctx, `
		UPDATE clawx_sessions
		SET
			window_id = $2,
			agent_id = $3,
			backend = $4,
			backend_session_id = $5,
			conversation_id = $6,
			cwd = $7,
			status = $8,
			lock_token = $9,
			last_used_at = $10
		WHERE id = $1
	`,
		record.ID,
		record.WindowID,
		record.AgentID,
		record.Backend,
		record.BackendSessionID,
		record.ConversationID,
		record.CWD,
		string(record.Status),
		record.LockToken,
		record.LastUsedAt.UTC(),
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return session.ErrSessionNotFound
	}
	return nil
}

func (r *PostgresSessionRepository) GetByID(ctx context.Context, sessionID string) (session.Record, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, window_id, agent_id, backend, backend_session_id,
			conversation_id, cwd, status, lock_token, last_used_at
		FROM clawx_sessions
		WHERE id = $1
	`, sessionID)
	record, err := scanSessionRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session.Record{}, session.ErrSessionNotFound
		}
		return session.Record{}, err
	}
	return record, nil
}

func (r *PostgresSessionRepository) GetLatestByConversation(ctx context.Context, conversationID string) (session.Record, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			id, window_id, agent_id, backend, backend_session_id,
			conversation_id, cwd, status, lock_token, last_used_at
		FROM clawx_sessions
		WHERE conversation_id = $1
		ORDER BY last_used_at DESC
		LIMIT 1
	`, conversationID)
	record, err := scanSessionRecord(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session.Record{}, session.ErrSessionNotFound
		}
		return session.Record{}, err
	}
	return record, nil
}

func (r *PostgresSessionRepository) ListByConversation(ctx context.Context, conversationID string) ([]session.Record, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT
			id, window_id, agent_id, backend, backend_session_id,
			conversation_id, cwd, status, lock_token, last_used_at
		FROM clawx_sessions
		WHERE conversation_id = $1
		ORDER BY last_used_at DESC
	`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]session.Record, 0)
	for rows.Next() {
		record, err := scanSessionRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (r *PostgresSessionRepository) GetWindowBinding(ctx context.Context, windowID string) (session.WindowBinding, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT window_id, current_session_id, conversation_id, updated_at, last_used_at
		FROM clawx_window_bindings
		WHERE window_id = $1
	`, strings.TrimSpace(windowID))

	var binding session.WindowBinding
	if err := row.Scan(
		&binding.WindowID,
		&binding.CurrentSessionID,
		&binding.ConversationID,
		&binding.UpdatedAt,
		&binding.LastUsedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session.WindowBinding{}, session.ErrWindowBindingNotFound
		}
		return session.WindowBinding{}, err
	}
	return binding, nil
}

func (r *PostgresSessionRepository) SetWindowBinding(ctx context.Context, binding session.WindowBinding) error {
	binding.WindowID = strings.TrimSpace(binding.WindowID)
	binding.CurrentSessionID = strings.TrimSpace(binding.CurrentSessionID)
	binding.ConversationID = strings.TrimSpace(binding.ConversationID)
	if err := binding.Validate(); err != nil {
		return err
	}
	now := time.Now().UTC()
	if binding.UpdatedAt.IsZero() {
		binding.UpdatedAt = now
	}
	if binding.LastUsedAt.IsZero() {
		binding.LastUsedAt = binding.UpdatedAt
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO clawx_window_bindings (
			window_id, current_session_id, conversation_id, updated_at, last_used_at
		) VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (window_id) DO UPDATE SET
			current_session_id = EXCLUDED.current_session_id,
			conversation_id = EXCLUDED.conversation_id,
			updated_at = EXCLUDED.updated_at,
			last_used_at = EXCLUDED.last_used_at
	`,
		binding.WindowID,
		binding.CurrentSessionID,
		binding.ConversationID,
		binding.UpdatedAt.UTC(),
		binding.LastUsedAt.UTC(),
	)
	return err
}

func (r *PostgresSessionRepository) ListWindowBindingsByConversation(ctx context.Context, conversationID string) ([]session.WindowBinding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT window_id, current_session_id, conversation_id, updated_at, last_used_at
		FROM clawx_window_bindings
		WHERE conversation_id = $1
		ORDER BY last_used_at DESC
	`, strings.TrimSpace(conversationID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bindings := make([]session.WindowBinding, 0)
	for rows.Next() {
		var binding session.WindowBinding
		if err := rows.Scan(
			&binding.WindowID,
			&binding.CurrentSessionID,
			&binding.ConversationID,
			&binding.UpdatedAt,
			&binding.LastUsedAt,
		); err != nil {
			return nil, err
		}
		bindings = append(bindings, binding)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return bindings, nil
}

func (r *PostgresSessionRepository) Acquire(ctx context.Context, sessionID string) (string, error) {
	lockToken := fmt.Sprintf("%s-%d", sessionID, time.Now().UTC().UnixNano())
	result, err := r.db.ExecContext(ctx, `
		UPDATE clawx_sessions
		SET lock_token = $2
		WHERE id = $1 AND lock_token = ''
	`, sessionID, lockToken)
	if err != nil {
		return "", err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	if rows == 1 {
		return lockToken, nil
	}

	var currentLock string
	err = r.db.QueryRowContext(ctx, `SELECT lock_token FROM clawx_sessions WHERE id = $1`, sessionID).Scan(&currentLock)
	if errors.Is(err, sql.ErrNoRows) {
		return "", session.ErrSessionNotFound
	}
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(currentLock) != "" {
		return "", ErrLockHeldByAnotherProcess
	}
	return "", ErrLockHeldByAnotherProcess
}

func (r *PostgresSessionRepository) Release(ctx context.Context, sessionID, lockToken string) error {
	var currentLock string
	err := r.db.QueryRowContext(ctx, `SELECT lock_token FROM clawx_sessions WHERE id = $1`, sessionID).Scan(&currentLock)
	if errors.Is(err, sql.ErrNoRows) {
		return session.ErrSessionNotFound
	}
	if err != nil {
		return err
	}

	currentLock = strings.TrimSpace(currentLock)
	if currentLock == "" {
		return nil
	}
	if currentLock != strings.TrimSpace(lockToken) {
		return session.ErrInvalidLock
	}

	_, err = r.db.ExecContext(ctx, `UPDATE clawx_sessions SET lock_token = '' WHERE id = $1`, sessionID)
	return err
}

func normalizePostgresConfig(cfg PostgresConfig) PostgresConfig {
	cfg.Host = strings.TrimSpace(cfg.Host)
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port <= 0 {
		cfg.Port = 5432
	}
	cfg.Database = strings.TrimSpace(cfg.Database)
	if cfg.Database == "" {
		cfg.Database = "claw_x"
	}
	cfg.User = strings.TrimSpace(cfg.User)
	if cfg.User == "" {
		cfg.User = "postgres"
	}
	cfg.SSLMode = strings.TrimSpace(cfg.SSLMode)
	if cfg.SSLMode == "" {
		cfg.SSLMode = "disable"
	}
	return cfg
}

func ensurePostgresDatabase(ctx context.Context, cfg PostgresConfig) error {
	adminDB, err := sql.Open("postgres", buildPostgresDSN(cfg, "postgres"))
	if err != nil {
		return fmt.Errorf("open postgres admin db: %w", err)
	}
	defer adminDB.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := adminDB.PingContext(pingCtx); err != nil {
		return fmt.Errorf("ping postgres admin db: %w", err)
	}

	var exists int
	err = adminDB.QueryRowContext(ctx, `SELECT 1 FROM pg_database WHERE datname = $1`, cfg.Database).Scan(&exists)
	if err == nil {
		return nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = adminDB.ExecContext(ctx, "CREATE DATABASE "+quoteIdentifier(cfg.Database))
	if err != nil {
		return fmt.Errorf("create database %q: %w", cfg.Database, err)
	}
	return nil
}

func buildPostgresDSN(cfg PostgresConfig, dbName string) string {
	parts := []string{
		fmt.Sprintf("host=%s", cfg.Host),
		fmt.Sprintf("port=%d", cfg.Port),
		fmt.Sprintf("user=%s", cfg.User),
		fmt.Sprintf("dbname=%s", dbName),
		fmt.Sprintf("sslmode=%s", cfg.SSLMode),
		"connect_timeout=5",
	}
	if strings.TrimSpace(cfg.Password) != "" {
		parts = append(parts, fmt.Sprintf("password=%s", cfg.Password))
	}
	return strings.Join(parts, " ")
}

func quoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanSessionRecord(scanner rowScanner) (session.Record, error) {
	var record session.Record
	var status string
	if err := scanner.Scan(
		&record.ID,
		&record.WindowID,
		&record.AgentID,
		&record.Backend,
		&record.BackendSessionID,
		&record.ConversationID,
		&record.CWD,
		&status,
		&record.LockToken,
		&record.LastUsedAt,
	); err != nil {
		return session.Record{}, err
	}
	record.Status = session.Status(strings.TrimSpace(status))
	if err := record.Validate(); err != nil {
		return session.Record{}, err
	}
	return record, nil
}
