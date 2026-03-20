package skillorchestrator

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type InstallResult struct {
	SkillID      string
	Version      string
	Source       skilldomain.RegistrySource
	InstalledDir string
	AgentState   *AgentState
}

type InstallService struct {
	registry           *RegistryService
	policy             *PolicyEngine
	agentStateProvider AgentStateProvider
}

func NewInstallService(registry *RegistryService, policy *PolicyEngine, provider AgentStateProvider) *InstallService {
	return &InstallService{
		registry:           registry,
		policy:             policy,
		agentStateProvider: provider,
	}
}

func (s *InstallService) Install(ctx context.Context, metadata skilldomain.SkillMetadata, targetAgentID string) (InstallResult, error) {
	if s == nil || s.registry == nil {
		return InstallResult{}, fmt.Errorf("install service is not configured")
	}
	normalized := metadata.Normalize()
	if err := normalized.Validate(); err != nil {
		return InstallResult{}, err
	}
	if s.policy != nil {
		if err := s.policy.Evaluate(ctx, normalized); err != nil {
			return InstallResult{}, err
		}
	}
	installedDir := ""
	if shouldInstallPackage(normalized) {
		dir, err := installSkillPackage(normalized)
		if err != nil {
			return InstallResult{}, err
		}
		installedDir = dir
	}
	if err := s.registry.Register(ctx, normalized); err != nil {
		if installedDir != "" {
			_ = os.RemoveAll(installedDir)
		}
		return InstallResult{}, err
	}
	result := InstallResult{
		SkillID:      normalized.SkillID,
		Version:      normalized.Version,
		Source:       normalized.Source,
		InstalledDir: installedDir,
	}
	targetAgentID = strings.TrimSpace(targetAgentID)
	if targetAgentID != "" && s.agentStateProvider != nil {
		if state, ok := s.agentStateProvider.Get(targetAgentID); ok {
			result.AgentState = &state
		}
	}
	return result, nil
}

func shouldInstallPackage(metadata skilldomain.SkillMetadata) bool {
	uri := extractPackageURI(metadata)
	return uri != ""
}

func extractPackageURI(metadata skilldomain.SkillMetadata) string {
	if metadata.InputSchema == nil {
		return ""
	}
	lookup := []string{"package_uri", "package", "archive", "source_uri"}
	for _, key := range lookup {
		value, ok := metadata.InputSchema[key]
		if !ok {
			continue
		}
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text != "" {
			return text
		}
	}
	return ""
}

func installSkillPackage(metadata skilldomain.SkillMetadata) (string, error) {
	packageURI := extractPackageURI(metadata)
	sourcePath, err := resolveLocalPackagePath(packageURI)
	if err != nil {
		return "", err
	}
	root, err := resolveSkillInstallRoot()
	if err != nil {
		return "", err
	}
	marketplaceRoot := filepath.Join(root, "marketplace")
	if err := os.MkdirAll(marketplaceRoot, 0o755); err != nil {
		return "", fmt.Errorf("prepare marketplace root: %w", err)
	}
	skillIDSegment := sanitizePathSegment(metadata.SkillID)
	versionSegment := sanitizePathSegment(metadata.Version)
	targetDir := filepath.Join(marketplaceRoot, skillIDSegment, versionSegment)
	stageDir, err := os.MkdirTemp(marketplaceRoot, ".install-stage-*")
	if err != nil {
		return "", fmt.Errorf("create install stage: %w", err)
	}
	stageFailed := true
	defer func() {
		if stageFailed {
			_ = os.RemoveAll(stageDir)
		}
	}()
	if err := extractPackageArchive(sourcePath, stageDir); err != nil {
		return "", err
	}
	skillManifests, err := findSkillManifests(stageDir)
	if err != nil {
		return "", err
	}
	if len(skillManifests) == 0 {
		return "", fmt.Errorf("invalid skill package: missing SKILL.md")
	}
	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("cleanup target dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(targetDir), 0o755); err != nil {
		return "", fmt.Errorf("prepare target root: %w", err)
	}
	if err := os.Rename(stageDir, targetDir); err != nil {
		return "", fmt.Errorf("activate installed package: %w", err)
	}
	stageFailed = false
	return targetDir, nil
}

func resolveSkillInstallRoot() (string, error) {
	if custom := strings.TrimSpace(os.Getenv("CLAWX_SKILL_INSTALL_ROOT")); custom != "" {
		if err := os.MkdirAll(custom, 0o755); err != nil {
			return "", fmt.Errorf("create install root: %w", err)
		}
		return filepath.Clean(custom), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home for skill install: %w", err)
	}
	root := filepath.Join(home, ".clawx", "skills")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("create install root: %w", err)
	}
	return root, nil
}

func resolveLocalPackagePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("package uri is required")
	}
	if strings.HasPrefix(raw, "file://") {
		parsed, err := url.Parse(raw)
		if err != nil {
			return "", fmt.Errorf("parse package uri: %w", err)
		}
		if parsed.Scheme != "file" {
			return "", fmt.Errorf("unsupported package scheme: %s", parsed.Scheme)
		}
		path := parsed.Path
		if path == "" {
			return "", fmt.Errorf("package uri path is empty")
		}
		path, err = url.PathUnescape(path)
		if err != nil {
			return "", fmt.Errorf("decode package uri path: %w", err)
		}
		raw = path
	}
	if !filepath.IsAbs(raw) {
		raw = filepath.Clean(raw)
	}
	info, err := os.Stat(raw)
	if err != nil {
		return "", fmt.Errorf("stat package path %q: %w", raw, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("package path must be an archive file")
	}
	return raw, nil
}

func extractPackageArchive(archivePath, destRoot string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("open package archive: %w", err)
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("open gzip archive: %w", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar entry: %w", err)
		}
		name := filepath.Clean(strings.TrimPrefix(header.Name, "./"))
		if name == "." || name == "" {
			continue
		}
		target := filepath.Join(destRoot, name)
		rel, err := filepath.Rel(destRoot, target)
		if err != nil || strings.HasPrefix(rel, "..") {
			return fmt.Errorf("unsafe archive entry: %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("create directory from archive: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return fmt.Errorf("create parent directory from archive: %w", err)
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
			if err != nil {
				return fmt.Errorf("create file from archive: %w", err)
			}
			if _, err := io.Copy(out, tarReader); err != nil {
				_ = out.Close()
				return fmt.Errorf("write file from archive: %w", err)
			}
			if err := out.Close(); err != nil {
				return fmt.Errorf("close extracted file: %w", err)
			}
		default:
			// Ignore unsupported entry types to keep installer deterministic.
		}
	}
	return nil
}

func findSkillManifests(root string) ([]string, error) {
	var manifests []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "SKILL.md") {
			manifests = append(manifests, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan installed package: %w", err)
	}
	sort.Strings(manifests)
	return manifests, nil
}

func sanitizePathSegment(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "unknown"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	return replacer.Replace(raw)
}
