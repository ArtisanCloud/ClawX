package skills

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type IndexStore struct {
	path string
}

func NewIndexStore(path string) *IndexStore {
	return &IndexStore{path: strings.TrimSpace(path)}
}

func (s *IndexStore) Path() string {
	return s.path
}

func (s *IndexStore) Save(snapshot skilldomain.RegistrySnapshot) error {
	if strings.TrimSpace(s.path) == "" {
		return nil
	}

	payload := indexSnapshot{
		Version:     snapshot.Version,
		GeneratedAt: snapshot.GeneratedAt.UTC(),
		Entries:     make([]indexEntry, 0, len(snapshot.Entries)),
	}
	for _, entry := range snapshot.Entries {
		item := indexEntry{
			Key:        entry.Key,
			SkillName:  entry.SkillName,
			Source:     string(entry.Source),
			Status:     string(entry.Status),
			ShadowedBy: entry.ShadowedBy,
			Errors:     append([]string(nil), entry.Errors...),
			UpdatedAt:  entry.UpdatedAt.UTC(),
		}
		if entry.Definition != nil {
			item.Definition = &indexDefinition{
				Name:            entry.Definition.Name,
				Description:     entry.Definition.Description,
				Aliases:         append([]string(nil), entry.Definition.Aliases...),
				InstructionBody: entry.Definition.InstructionBody,
				Source:          string(entry.Definition.Source),
				BaseDir:         entry.Definition.BaseDir,
				ManifestPath:    entry.Definition.ManifestPath,
				LoadedAt:        entry.Definition.LoadedAt.UTC(),
			}
		}
		payload.Entries = append(payload.Entries, item)
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal skill index: %w", err)
	}
	body = append(body, '\n')

	dir := filepath.Dir(s.path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create skill index directory: %w", err)
		}
	}

	tmp, err := os.CreateTemp(dir, "skills-index-*.tmp")
	if err != nil {
		return fmt.Errorf("create skill index temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write skill index temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close skill index temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace skill index file: %w", err)
	}
	return nil
}

func (s *IndexStore) Load() (skilldomain.RegistrySnapshot, error) {
	if strings.TrimSpace(s.path) == "" {
		return skilldomain.EmptySnapshot(), nil
	}
	body, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return skilldomain.EmptySnapshot(), nil
		}
		return skilldomain.RegistrySnapshot{}, fmt.Errorf("read skill index: %w", err)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return skilldomain.EmptySnapshot(), nil
	}

	var raw indexSnapshot
	if err := json.Unmarshal(body, &raw); err != nil {
		return skilldomain.RegistrySnapshot{}, fmt.Errorf("parse skill index: %w", err)
	}

	snapshot := skilldomain.RegistrySnapshot{
		Version:     raw.Version,
		GeneratedAt: raw.GeneratedAt,
		Entries:     make([]skilldomain.CatalogEntry, 0, len(raw.Entries)),
	}
	for _, item := range raw.Entries {
		entry := skilldomain.CatalogEntry{
			Key:        strings.TrimSpace(item.Key),
			SkillName:  skilldomain.NormalizeName(item.SkillName),
			Source:     skilldomain.Source(strings.TrimSpace(item.Source)),
			Status:     skilldomain.Status(strings.TrimSpace(item.Status)),
			ShadowedBy: strings.TrimSpace(item.ShadowedBy),
			Errors:     append([]string(nil), item.Errors...),
			UpdatedAt:  item.UpdatedAt,
		}
		if item.Definition != nil {
			def := skilldomain.Definition{
				Name:            strings.TrimSpace(item.Definition.Name),
				Description:     strings.TrimSpace(item.Definition.Description),
				Aliases:         skilldomain.NormalizeAliases(item.Definition.Aliases),
				InstructionBody: strings.TrimSpace(item.Definition.InstructionBody),
				Source:          skilldomain.Source(strings.TrimSpace(item.Definition.Source)),
				BaseDir:         strings.TrimSpace(item.Definition.BaseDir),
				ManifestPath:    strings.TrimSpace(item.Definition.ManifestPath),
				LoadedAt:        item.Definition.LoadedAt,
			}
			entry.Definition = &def
		}
		snapshot.Entries = append(snapshot.Entries, entry)
	}
	skilldomain.SortEntries(snapshot.Entries)
	if snapshot.GeneratedAt.IsZero() {
		snapshot.GeneratedAt = time.Now().UTC()
	}
	return snapshot, nil
}

type indexSnapshot struct {
	Version     int64        `json:"version"`
	GeneratedAt time.Time    `json:"generated_at"`
	Entries     []indexEntry `json:"entries"`
}

type indexEntry struct {
	Key        string           `json:"key"`
	SkillName  string           `json:"skill_name"`
	Source     string           `json:"source"`
	Status     string           `json:"status"`
	ShadowedBy string           `json:"shadowed_by,omitempty"`
	Errors     []string         `json:"errors,omitempty"`
	Definition *indexDefinition `json:"definition,omitempty"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

type indexDefinition struct {
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	Aliases         []string  `json:"aliases,omitempty"`
	InstructionBody string    `json:"instruction_body"`
	Source          string    `json:"source"`
	BaseDir         string    `json:"base_dir"`
	ManifestPath    string    `json:"manifest_path"`
	LoadedAt        time.Time `json:"loaded_at"`
}
