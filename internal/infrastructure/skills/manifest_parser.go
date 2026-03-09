package skills

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	skilldomain "synapsex/internal/domain/skill"
)

var ErrInvalidFrontmatter = errors.New("invalid skill frontmatter")

type ParseResult struct {
	Definition skilldomain.Definition
	Warnings   []string
}

func ParseManifest(path string, source skilldomain.Source) (ParseResult, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return ParseResult{}, fmt.Errorf("read skill manifest: %w", err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")
	frontmatter, instruction, err := splitFrontmatter(text)
	if err != nil {
		return ParseResult{}, err
	}

	meta := parseFrontmatter(frontmatter)
	def := skilldomain.Definition{
		Name:            firstNonEmpty(meta["name"]),
		Description:     firstNonEmpty(meta["description"]),
		Aliases:         parseAliases(meta),
		InstructionBody: strings.TrimSpace(instruction),
		Source:          source,
		BaseDir:         filepath.Dir(path),
		ManifestPath:    path,
		LoadedAt:        time.Now().UTC(),
	}
	definition, err := def.Normalize()
	if err != nil {
		return ParseResult{}, ErrInvalidFrontmatter
	}
	return ParseResult{Definition: definition}, nil
}

func splitFrontmatter(content string) (string, string, error) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---\n") {
		return "", "", ErrInvalidFrontmatter
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 3 {
		return "", "", ErrInvalidFrontmatter
	}

	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return "", "", ErrInvalidFrontmatter
	}

	frontmatter := strings.Join(lines[1:end], "\n")
	body := ""
	if end+1 < len(lines) {
		body = strings.Join(lines[end+1:], "\n")
	}
	return frontmatter, body, nil
}

func parseFrontmatter(raw string) map[string][]string {
	result := make(map[string][]string)
	lines := strings.Split(raw, "\n")
	currentKey := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "- ") && currentKey != "" {
			result[currentKey] = append(result[currentKey], strings.TrimSpace(strings.TrimPrefix(line, "- ")))
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		currentKey = key
		if value != "" {
			result[key] = append(result[key], value)
		} else if _, ok := result[key]; !ok {
			result[key] = nil
		}
	}
	return result
}

func parseAliases(meta map[string][]string) []string {
	values := append([]string(nil), meta["aliases"]...)
	if len(values) == 0 {
		return nil
	}
	return skilldomain.NormalizeAliases(values)
}

func firstNonEmpty(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(strings.Trim(value, `"'`))
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
