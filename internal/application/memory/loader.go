package memory

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	memorydomain "clawx/internal/domain/memory"
)

type Reader interface {
	ReadFile(ctx context.Context, path string) ([]byte, error)
}

type osReader struct{}

func (osReader) ReadFile(_ context.Context, path string) ([]byte, error) {
	return os.ReadFile(path)
}

type Loader struct {
	reader Reader
}

type LoaderOption func(*Loader)

func WithLoaderReader(reader Reader) LoaderOption {
	return func(loader *Loader) {
		if reader != nil {
			loader.reader = reader
		}
	}
}

func NewLoader(options ...LoaderOption) *Loader {
	loader := &Loader{reader: osReader{}}
	for _, option := range options {
		if option != nil {
			option(loader)
		}
	}
	return loader
}

type LoaderInput struct {
	ScopeKey   memorydomain.MemoryScopeKey
	Profile    memorydomain.MemoryProfile
	Candidates []memorydomain.MemoryLoadItem
}

type LoaderOutput struct {
	PromptContext string
	LoadedFiles   []string
	DeniedFiles   []string
	LoadItems     []memorydomain.MemoryLoadItem
	ErrorSummary  string
	Degraded      bool
}

func (l *Loader) Load(ctx context.Context, input LoaderInput) (LoaderOutput, error) {
	if err := input.ScopeKey.Validate(); err != nil {
		return LoaderOutput{}, err
	}
	if input.Profile.ScopeKey.AgentID == "" && input.Profile.ScopeKey.ProjectID == "" && input.Profile.ScopeKey.RouteKey == "" {
		input.Profile.ScopeKey = input.ScopeKey
	}
	if err := input.Profile.Validate(); err != nil {
		return LoaderOutput{}, err
	}

	reader := l.reader
	if reader == nil {
		reader = osReader{}
	}

	output := LoaderOutput{
		LoadedFiles: make([]string, 0),
		DeniedFiles: make([]string, 0),
		LoadItems:   make([]memorydomain.MemoryLoadItem, 0, len(input.Candidates)),
	}
	var promptBuilder strings.Builder
	consumed := 0
	errorsFound := make([]string, 0)

	for _, rawItem := range input.Candidates {
		item := rawItem
		item.Path = strings.TrimSpace(item.Path)
		if err := item.Validate(); err != nil {
			item.Decision = memorydomain.DecisionError
			item.Reason = "invalid_item"
			output.LoadItems = append(output.LoadItems, item)
			output.Degraded = true
			errorsFound = append(errorsFound, "invalid_item")
			continue
		}

		if item.Layer == memorydomain.LayerMainPrivate && !input.Profile.AllowMainPrivate {
			item.Decision = memorydomain.DecisionSkippedACL
			item.Reason = "main_private_requires_acl"
			output.DeniedFiles = append(output.DeniedFiles, item.Path)
			output.LoadItems = append(output.LoadItems, item)
			continue
		}

		body, err := reader.ReadFile(ctx, item.Path)
		if err != nil {
			item.Decision = memorydomain.DecisionError
			item.Reason = "read_failed"
			output.LoadItems = append(output.LoadItems, item)
			output.Degraded = true
			errorsFound = append(errorsFound, fmt.Sprintf("read_failed:%s", item.Path))
			continue
		}
		item.SizeBytes = len(body)
		if consumed+len(body) > input.Profile.TokenBudget {
			item.Decision = memorydomain.DecisionSkippedBudget
			item.Reason = "token_budget_exceeded"
			output.DeniedFiles = append(output.DeniedFiles, item.Path)
			output.LoadItems = append(output.LoadItems, item)
			continue
		}

		if promptBuilder.Len() > 0 {
			promptBuilder.WriteString("\n\n")
		}
		promptBuilder.WriteString("# memory ")
		promptBuilder.WriteString(string(item.Layer))
		promptBuilder.WriteString(" ")
		promptBuilder.WriteString(item.Path)
		promptBuilder.WriteString("\n")
		promptBuilder.Write(body)

		consumed += len(body)
		item.Decision = memorydomain.DecisionLoaded
		item.Reason = ""
		output.LoadedFiles = append(output.LoadedFiles, item.Path)
		output.LoadItems = append(output.LoadItems, item)
	}

	output.PromptContext = promptBuilder.String()
	output.ErrorSummary = summarizeLoaderErrors(errorsFound)
	if output.ErrorSummary != "" {
		output.Degraded = true
	}
	return output, nil
}

func summarizeLoaderErrors(values []string) string {
	if len(values) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(values))
	uniq := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		uniq = append(uniq, value)
	}
	if len(uniq) == 0 {
		return ""
	}
	sort.Strings(uniq)
	return strings.Join(uniq, ";")
}
