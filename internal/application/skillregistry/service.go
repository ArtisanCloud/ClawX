package skillregistry

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	skilldomain "clawx/internal/domain/skill"
	skillsinfra "clawx/internal/infrastructure/skills"
)

var (
	ErrSkillNotFound = errors.New("skill not found")
	ErrSkillInvalid  = errors.New("skill invalid")
	ErrSkillDisabled = errors.New("skill disabled")
)

type Config struct {
	Sources    []skillsinfra.SourceSpec
	IndexPath  string
	Disabled   []string
	BuiltinDir string
	RefreshNow bool
}

type ReloadResult struct {
	Version   int64
	Entries   int
	Active    int
	UpdatedAt time.Time
}

type Service struct {
	mu         sync.RWMutex
	indexStore *skillsinfra.IndexStore
	snapshot   skilldomain.RegistrySnapshot
	disabled   map[string]struct{}
	sources    []skillsinfra.SourceSpec
	version    int64
}

func New(cfg Config) (*Service, error) {
	sources := normalizeSources(cfg.Sources, cfg.BuiltinDir)
	store := skillsinfra.NewIndexStore(strings.TrimSpace(cfg.IndexPath))
	snapshot, err := store.Load()
	if err != nil {
		return nil, err
	}

	service := &Service{
		indexStore: store,
		snapshot:   snapshot,
		disabled:   make(map[string]struct{}),
		sources:    sources,
		version:    snapshot.Version,
	}
	for _, name := range cfg.Disabled {
		normalized := skilldomain.NormalizeName(name)
		if normalized != "" {
			service.disabled[normalized] = struct{}{}
		}
	}
	if cfg.RefreshNow {
		if _, err := service.Reload(context.Background()); err != nil {
			return nil, err
		}
	}
	return service, nil
}

func (s *Service) Snapshot() skilldomain.RegistrySnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneSnapshot(s.snapshot)
}

func (s *Service) Reload(ctx context.Context) (ReloadResult, error) {
	snapshot, err := BuildSnapshot(ctx, s.sources, s.disabled, s.version+1)
	if err != nil {
		return ReloadResult{}, err
	}
	if err := s.indexStore.Save(snapshot); err != nil {
		return ReloadResult{}, err
	}

	active := 0
	for _, entry := range snapshot.Entries {
		if entry.Status == skilldomain.StatusActive {
			active++
		}
	}

	s.mu.Lock()
	s.version = snapshot.Version
	s.snapshot = snapshot
	s.mu.Unlock()

	return ReloadResult{
		Version:   snapshot.Version,
		Entries:   len(snapshot.Entries),
		Active:    active,
		UpdatedAt: snapshot.GeneratedAt,
	}, nil
}

func (s *Service) List() []skilldomain.CatalogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]skilldomain.CatalogEntry, len(s.snapshot.Entries))
	copy(result, s.snapshot.Entries)
	return result
}

func (s *Service) FindEntry(name string) (skilldomain.CatalogEntry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot.FindEntryByName(name)
}

func (s *Service) FindActiveDefinition(name string) (skilldomain.Definition, error) {
	entry, ok := s.FindEntry(name)
	if !ok {
		return skilldomain.Definition{}, ErrSkillNotFound
	}
	switch entry.Status {
	case skilldomain.StatusInvalid:
		return skilldomain.Definition{}, ErrSkillInvalid
	case skilldomain.StatusDisabled:
		return skilldomain.Definition{}, ErrSkillDisabled
	case skilldomain.StatusShadowed:
		return skilldomain.Definition{}, ErrSkillNotFound
	}
	if entry.Definition == nil {
		return skilldomain.Definition{}, ErrSkillInvalid
	}
	return *entry.Definition, nil
}

func (s *Service) Disable(name string) error {
	name = skilldomain.NormalizeName(name)
	if name == "" {
		return ErrSkillNotFound
	}

	s.mu.Lock()
	s.disabled[name] = struct{}{}
	s.mu.Unlock()
	return s.reloadWithoutLock()
}

func (s *Service) Enable(name string) error {
	name = skilldomain.NormalizeName(name)
	if name == "" {
		return ErrSkillNotFound
	}

	s.mu.Lock()
	delete(s.disabled, name)
	s.mu.Unlock()
	return s.reloadWithoutLock()
}

func (s *Service) reloadWithoutLock() error {
	_, err := s.Reload(context.Background())
	return err
}

func (s *Service) DisabledNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.disabled))
	for name := range s.disabled {
		out = append(out, name)
	}
	return out
}

func normalizeSources(values []skillsinfra.SourceSpec, builtinDir string) []skillsinfra.SourceSpec {
	result := make([]skillsinfra.SourceSpec, 0, len(values)+1)
	seen := make(map[string]struct{}, len(values)+1)
	for _, source := range values {
		root := strings.TrimSpace(source.Root)
		if root == "" {
			continue
		}
		key := string(source.Source) + ":" + filepath.Clean(root)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, source)
	}
	if strings.TrimSpace(builtinDir) != "" {
		key := string(skilldomain.SourceBuiltin) + ":" + filepath.Clean(builtinDir)
		if _, ok := seen[key]; !ok {
			result = append(result, skillsinfra.SourceSpec{
				Source: skilldomain.SourceBuiltin,
				Root:   builtinDir,
			})
		}
	}
	return result
}

func cloneSnapshot(snapshot skilldomain.RegistrySnapshot) skilldomain.RegistrySnapshot {
	out := skilldomain.RegistrySnapshot{
		Version:     snapshot.Version,
		GeneratedAt: snapshot.GeneratedAt,
		Entries:     make([]skilldomain.CatalogEntry, 0, len(snapshot.Entries)),
	}
	for _, entry := range snapshot.Entries {
		copied := entry
		if entry.Errors != nil {
			copied.Errors = append([]string(nil), entry.Errors...)
		}
		if entry.Definition != nil {
			def := *entry.Definition
			def.Aliases = append([]string(nil), def.Aliases...)
			copied.Definition = &def
		}
		out.Entries = append(out.Entries, copied)
	}
	return out
}

func ValidateSkillName(name string) error {
	if skilldomain.NormalizeName(name) == "" {
		return fmt.Errorf("skill name is required")
	}
	return nil
}
