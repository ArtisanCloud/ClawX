package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	memorydomain "clawx/internal/domain/memory"
)

const DefaultTemplateVersion = "v1"

type TemplateManager struct {
	now func() time.Time
}

type TemplateOption func(*TemplateManager)

type EnsureTemplateResult struct {
	ManifestVersion string
	CreatedFiles    []string
	HealedFiles     []string
	Drifted         bool
}

func WithTemplateClock(now func() time.Time) TemplateOption {
	return func(m *TemplateManager) {
		if now != nil {
			m.now = now
		}
	}
}

func NewTemplateManager(options ...TemplateOption) *TemplateManager {
	manager := &TemplateManager{
		now: func() time.Time { return time.Now().UTC() },
	}
	for _, option := range options {
		if option != nil {
			option(manager)
		}
	}
	return manager
}

func (m *TemplateManager) EnsureProjectTemplate(ctx context.Context, projectID, workspacePath string) error {
	_, err := m.EnsureProjectTemplateDetailed(ctx, projectID, workspacePath)
	return err
}

func (m *TemplateManager) EnsureProjectTemplateDetailed(_ context.Context, projectID, workspacePath string) (EnsureTemplateResult, error) {
	projectID = strings.TrimSpace(projectID)
	workspacePath = strings.TrimSpace(workspacePath)
	if projectID == "" || workspacePath == "" {
		return EnsureTemplateResult{}, memorydomain.ErrInvalidMemoryScope
	}

	if err := os.MkdirAll(workspacePath, 0o755); err != nil {
		return EnsureTemplateResult{}, fmt.Errorf("create memory workspace root: %w", err)
	}

	now := m.now().UTC()
	templates := defaultTemplateFiles(projectID)
	required := sortedKeys(templates)
	optional := []string{"memory/README.md"}

	manifestPath := filepath.Join(workspacePath, ".memory", "manifest.json")
	previous, hasManifest, err := readTemplateManifest(manifestPath)
	if err != nil {
		return EnsureTemplateResult{}, err
	}

	result := EnsureTemplateResult{ManifestVersion: DefaultTemplateVersion}
	if hasManifest && strings.TrimSpace(previous.TemplateVersion) != DefaultTemplateVersion {
		result.Drifted = true
	}
	if hasManifest {
		if !slices.Equal(sortedCopy(previous.RequiredFiles), required) {
			result.Drifted = true
		}
	}

	for _, relativePath := range required {
		absPath := filepath.Join(workspacePath, relativePath)
		content := templates[relativePath]
		_, statErr := os.Stat(absPath)
		switch {
		case statErr == nil:
			continue
		case !os.IsNotExist(statErr):
			return EnsureTemplateResult{}, fmt.Errorf("stat memory template file %q: %w", absPath, statErr)
		}

		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return EnsureTemplateResult{}, fmt.Errorf("create memory template directory %q: %w", absPath, err)
		}
		if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
			return EnsureTemplateResult{}, fmt.Errorf("write memory template file %q: %w", absPath, err)
		}

		if hasManifest {
			result.HealedFiles = append(result.HealedFiles, relativePath)
			result.Drifted = true
		} else {
			result.CreatedFiles = append(result.CreatedFiles, relativePath)
		}
	}

	for _, relativePath := range optional {
		absPath := filepath.Join(workspacePath, relativePath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return EnsureTemplateResult{}, fmt.Errorf("create memory optional directory %q: %w", absPath, err)
		}
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			if err := os.WriteFile(absPath, []byte(defaultOptionalContent(relativePath)), 0o644); err != nil {
				return EnsureTemplateResult{}, fmt.Errorf("write memory optional file %q: %w", absPath, err)
			}
		}
	}

	manifest := memorydomain.TemplateManifest{
		TemplateVersion: DefaultTemplateVersion,
		RequiredFiles:   required,
		OptionalFiles:   optional,
		UpdatedAt:       now,
	}
	if err := writeTemplateManifest(manifestPath, manifest); err != nil {
		return EnsureTemplateResult{}, err
	}

	return result, nil
}

func defaultTemplateFiles(projectID string) map[string]string {
	title := strings.TrimSpace(projectID)
	if title == "" {
		title = "project"
	}
	return map[string]string{
		"AGENTS.md":    "# AGENTS\n\n- project: " + title + "\n",
		"HEARTBEAT.md": "# HEARTBEAT\n\n- status: active\n",
		"IDENTITY.md":  "# IDENTITY\n\n- role: project context\n",
		"SOUL.md":      "# SOUL\n\n- principle: keep tasks focused\n",
		"TOOLS.md":     "# TOOLS\n\n- command-first workflow\n",
		"USER.md":      "# USER\n\n- preference: concise output\n",
		"MEMORY.md":    "# MEMORY\n\n- long-term notes\n",
	}
}

func defaultOptionalContent(relativePath string) string {
	switch filepath.ToSlash(relativePath) {
	case "memory/README.md":
		return "# memory\n\nDaily journal files are stored in this directory.\n"
	default:
		return ""
	}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	slices.Sort(result)
	return result
}

func readTemplateManifest(path string) (memorydomain.TemplateManifest, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return memorydomain.TemplateManifest{}, false, nil
		}
		return memorydomain.TemplateManifest{}, false, fmt.Errorf("open memory manifest: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var manifest memorydomain.TemplateManifest
	if err := decoder.Decode(&manifest); err != nil {
		return memorydomain.TemplateManifest{}, false, fmt.Errorf("parse memory manifest: %w", err)
	}
	if decoder.More() {
		return memorydomain.TemplateManifest{}, false, fmt.Errorf("parse memory manifest: trailing data")
	}
	return manifest, true, nil
}

func writeTemplateManifest(path string, manifest memorydomain.TemplateManifest) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create memory manifest directory: %w", err)
	}
	payload, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal memory manifest: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		return fmt.Errorf("write memory manifest: %w", err)
	}
	return nil
}
