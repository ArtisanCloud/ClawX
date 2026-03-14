package memory

import (
	"context"
	"time"
)

type TemplateRepository interface {
	GetManifest(ctx context.Context, projectID string) (TemplateManifest, error)
	SaveManifest(ctx context.Context, projectID string, manifest TemplateManifest) error
	ReadFile(ctx context.Context, projectID, relativePath string) ([]byte, error)
	WriteFile(ctx context.Context, projectID, relativePath string, body []byte) error
}

type JournalRepository interface {
	Append(ctx context.Context, scopeKey MemoryScopeKey, entry JournalEntry) error
	ListByScope(ctx context.Context, scopeKey MemoryScopeKey, scope string, day time.Time) ([]JournalEntry, error)
}

type AuditRepository interface {
	Append(ctx context.Context, record AuditRecord) error
	ListByProject(ctx context.Context, projectID string, limit int) ([]AuditRecord, error)
}

type DigestRepository interface {
	Upsert(ctx context.Context, job DigestJob) error
	Get(ctx context.Context, projectID, jobID string) (DigestJob, error)
	ListByProject(ctx context.Context, projectID string, limit int) ([]DigestJob, error)
}
