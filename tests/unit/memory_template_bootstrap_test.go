package unit

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	memoryapp "clawx/internal/application/memory"
)

func TestTemplateManagerBootstrapAndHeal(t *testing.T) {
	fixedNow := time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)
	manager := memoryapp.NewTemplateManager(memoryapp.WithTemplateClock(func() time.Time { return fixedNow }))
	workspace := filepath.Join(t.TempDir(), "workspaces", "tools")

	result, err := manager.EnsureProjectTemplateDetailed(context.Background(), "tools", workspace)
	if err != nil {
		t.Fatalf("ensure template first time: %v", err)
	}
	if result.Drifted {
		t.Fatalf("first initialization should not report drift")
	}
	if len(result.CreatedFiles) == 0 {
		t.Fatalf("expected created files on first initialization")
	}

	required := []string{"AGENTS.md", "HEARTBEAT.md", "IDENTITY.md", "MEMORY.md", "SOUL.md", "TOOLS.md", "USER.md"}
	for _, name := range required {
		if _, err := os.Stat(filepath.Join(workspace, name)); err != nil {
			t.Fatalf("required template file %s missing: %v", name, err)
		}
	}

	manifestPath := filepath.Join(workspace, ".memory", "manifest.json")
	manifest := readTemplateManifestForTest(t, manifestPath)
	if manifest.TemplateVersion != memoryapp.DefaultTemplateVersion {
		t.Fatalf("unexpected template version: %q", manifest.TemplateVersion)
	}
	if !slices.Equal(sortedStrings(manifest.RequiredFiles), sortedStrings(required)) {
		t.Fatalf("unexpected required files in manifest: %#v", manifest.RequiredFiles)
	}

	if err := os.Remove(filepath.Join(workspace, "USER.md")); err != nil {
		t.Fatalf("remove USER.md: %v", err)
	}
	manifest.TemplateVersion = "v0"
	writeTemplateManifestForTest(t, manifestPath, manifest)

	result, err = manager.EnsureProjectTemplateDetailed(context.Background(), "tools", workspace)
	if err != nil {
		t.Fatalf("ensure template second time: %v", err)
	}
	if !result.Drifted {
		t.Fatalf("second initialization should detect drift")
	}
	if len(result.HealedFiles) != 1 || result.HealedFiles[0] != "USER.md" {
		t.Fatalf("unexpected healed files: %#v", result.HealedFiles)
	}
	if _, err := os.Stat(filepath.Join(workspace, "USER.md")); err != nil {
		t.Fatalf("USER.md should be healed: %v", err)
	}
}

type templateManifestPayload struct {
	TemplateVersion string    `json:"templateVersion"`
	RequiredFiles   []string  `json:"requiredFiles"`
	OptionalFiles   []string  `json:"optionalFiles"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func readTemplateManifestForTest(t *testing.T, path string) templateManifestPayload {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read template manifest: %v", err)
	}
	var payload templateManifestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode template manifest: %v", err)
	}
	return payload
}

func writeTemplateManifestForTest(t *testing.T, path string, payload templateManifestPayload) {
	t.Helper()
	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal template manifest: %v", err)
	}
	body = append(body, '\n')
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write template manifest: %v", err)
	}
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	slices.Sort(result)
	return result
}
