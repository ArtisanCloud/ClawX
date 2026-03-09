package intent

import (
	"context"
	"strings"

	skilldomain "synapsex/internal/domain/skill"
)

type LLMFallback interface {
	Match(ctx context.Context, text string, snapshot skilldomain.RegistrySnapshot) (Candidate, bool, error)
}

type DefaultLLMFallback struct{}

func (f DefaultLLMFallback) Match(_ context.Context, text string, snapshot skilldomain.RegistrySnapshot) (Candidate, bool, error) {
	normalizedText := normalizeName(text)
	if normalizedText == "" {
		return Candidate{}, false, nil
	}

	best := Candidate{}
	found := false
	for _, def := range snapshot.ActiveDefinitions() {
		name := normalizeName(def.Name)
		if name == "" {
			continue
		}
		score := softMatchScore(normalizedText, name, def.Aliases)
		if !found || score > best.Confidence {
			best = Candidate{
				SkillName:   name,
				Reason:      "llm_fallback",
				Confidence:  score,
				Instruction: def.InstructionBody,
			}
			found = true
		}
	}
	if !found {
		return Candidate{}, false, nil
	}
	return best, true, nil
}

func softMatchScore(text, name string, aliases []string) float64 {
	if text == name {
		return 0.99
	}
	if strings.Contains(text, name) {
		return 0.85
	}

	for _, alias := range aliases {
		normalizedAlias := normalizeName(alias)
		if normalizedAlias == "" {
			continue
		}
		if text == normalizedAlias {
			return 0.9
		}
		if strings.Contains(text, normalizedAlias) {
			return 0.8
		}
	}

	nameWords := strings.Fields(name)
	textWords := strings.Fields(text)
	if len(nameWords) == 0 || len(textWords) == 0 {
		return 0.0
	}

	matched := 0
	for _, word := range nameWords {
		for _, target := range textWords {
			if word == target {
				matched++
				break
			}
		}
	}
	if matched == 0 {
		return 0.0
	}
	return float64(matched) / float64(len(nameWords)) * 0.79
}
