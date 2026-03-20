package skill

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const ContextDigestVersion = 1

var ErrInvalidContextDigest = errors.New("invalid skill context digest")

type ContextDigestEntry struct {
	TraceID    string
	SkillID    string
	Intent     string
	Source     string
	Result     string
	RiskLevel  RiskLevel
	OccurredAt time.Time
}

type ContextDigest struct {
	Version        int
	ConversationID string
	Entries        []ContextDigestEntry
	GeneratedAt    time.Time
	Hash           string
}

func (d ContextDigest) Normalize() ContextDigest {
	out := d
	if out.Version <= 0 {
		out.Version = ContextDigestVersion
	}
	out.ConversationID = strings.TrimSpace(out.ConversationID)
	if out.GeneratedAt.IsZero() {
		out.GeneratedAt = time.Now().UTC()
	}
	if out.Entries == nil {
		out.Entries = make([]ContextDigestEntry, 0)
	}
	for i := range out.Entries {
		out.Entries[i].TraceID = strings.TrimSpace(out.Entries[i].TraceID)
		out.Entries[i].SkillID = strings.TrimSpace(out.Entries[i].SkillID)
		out.Entries[i].Intent = strings.TrimSpace(out.Entries[i].Intent)
		out.Entries[i].Source = strings.TrimSpace(out.Entries[i].Source)
		out.Entries[i].Result = strings.TrimSpace(out.Entries[i].Result)
		if out.Entries[i].OccurredAt.IsZero() {
			out.Entries[i].OccurredAt = out.GeneratedAt
		}
		if strings.TrimSpace(string(out.Entries[i].RiskLevel)) == "" {
			out.Entries[i].RiskLevel = RiskLow
		}
	}
	if strings.TrimSpace(out.Hash) == "" {
		out.Hash = out.ComputeHash()
	}
	return out
}

func (d ContextDigest) Validate() error {
	normalized := d.Normalize()
	if normalized.Version != ContextDigestVersion {
		return ErrInvalidContextDigest
	}
	if normalized.ConversationID == "" {
		return ErrInvalidContextDigest
	}
	if normalized.Hash != normalized.ComputeHash() {
		return ErrInvalidContextDigest
	}
	return nil
}

func (d ContextDigest) ComputeHash() string {
	payload := struct {
		Version        int                  `json:"version"`
		ConversationID string               `json:"conversation_id"`
		Entries        []ContextDigestEntry `json:"entries"`
	}{
		Version:        d.Version,
		ConversationID: strings.TrimSpace(d.ConversationID),
		Entries:        d.Entries,
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
