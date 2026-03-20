package skillorchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type CatalogDigest struct {
	Version      int
	SkillCount   int
	BindingCount int
	Hash         string
}

type CatalogDigestBuilder struct{}

func NewCatalogDigestBuilder() *CatalogDigestBuilder {
	return &CatalogDigestBuilder{}
}

func (b *CatalogDigestBuilder) Build(metadata []skilldomain.SkillMetadata, bindings []skilldomain.SkillBinding) CatalogDigest {
	entries := make([]string, 0, len(metadata)+len(bindings))
	for _, item := range metadata {
		m := item.Normalize()
		entries = append(entries, strings.Join([]string{
			"m",
			m.SkillID,
			m.Version,
			string(m.Source),
			boolString(m.Enabled),
		}, "|"))
	}
	for _, item := range bindings {
		v := item.Normalize()
		entries = append(entries, strings.Join([]string{
			"b",
			v.SkillID,
			v.Version,
			string(v.Scope),
			v.ProjectID,
			v.AgentID,
		}, "|"))
	}
	sort.Strings(entries)
	payload := struct {
		Version int      `json:"version"`
		Entries []string `json:"entries"`
	}{
		Version: 1,
		Entries: entries,
	}
	raw, _ := json.Marshal(payload)
	sum := sha256.Sum256(raw)
	return CatalogDigest{
		Version:      1,
		SkillCount:   len(metadata),
		BindingCount: len(bindings),
		Hash:         hex.EncodeToString(sum[:]),
	}
}

func boolString(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
