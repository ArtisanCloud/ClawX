package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	skilldomain "synapsex/internal/domain/skill"
)

type SourceSpec struct {
	Source skilldomain.Source
	Root   string
}

type Candidate struct {
	Source       skilldomain.Source
	SourceRoot   string
	BaseDir      string
	ManifestPath string
}

func DiscoverCandidates(sources []SourceSpec) ([]Candidate, error) {
	out := make([]Candidate, 0)
	for _, source := range sources {
		root := strings.TrimSpace(source.Root)
		if root == "" {
			continue
		}
		resolvedRoot := expandHome(root)
		info, err := os.Stat(resolvedRoot)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("stat skill source %q: %w", resolvedRoot, err)
		}
		if !info.IsDir() {
			continue
		}

		err = filepath.WalkDir(resolvedRoot, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if strings.EqualFold(entry.Name(), "SKILL.md") {
				baseDir := filepath.Dir(path)
				out = append(out, Candidate{
					Source:       source.Source,
					SourceRoot:   resolvedRoot,
					BaseDir:      baseDir,
					ManifestPath: path,
				})
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("scan skill source %q: %w", resolvedRoot, err)
		}
	}
	return out, nil
}

func expandHome(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return path
	}
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}
