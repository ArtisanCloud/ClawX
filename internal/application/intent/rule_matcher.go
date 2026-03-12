package intent

import (
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type Candidate struct {
	SkillName   string
	Reason      string
	Confidence  float64
	Instruction string
}

func MatchByRule(text string, snapshot skilldomain.RegistrySnapshot) []Candidate {
	text = normalizeName(text)
	if text == "" {
		return nil
	}

	active := snapshot.ActiveDefinitions()
	if len(active) == 0 {
		return nil
	}

	exact := make([]Candidate, 0, len(active))
	alias := make([]Candidate, 0, len(active))
	for _, def := range active {
		name := normalizeName(def.Name)
		if name == "" {
			continue
		}
		if text == name || strings.HasPrefix(text, name+" ") {
			exact = append(exact, Candidate{
				SkillName:   name,
				Reason:      "exact_name",
				Confidence:  0.95,
				Instruction: def.InstructionBody,
			})
			continue
		}
		for _, aliasValue := range def.Aliases {
			aliasName := normalizeName(aliasValue)
			if aliasName == "" {
				continue
			}
			if text == aliasName || strings.HasPrefix(text, aliasName+" ") {
				alias = append(alias, Candidate{
					SkillName:   name,
					Reason:      "alias_match",
					Confidence:  0.88,
					Instruction: def.InstructionBody,
				})
				break
			}
		}
	}

	if len(exact) > 0 {
		return exact
	}
	return alias
}

func normalizeName(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	if trimmed == "" {
		return ""
	}
	return strings.Join(strings.Fields(trimmed), " ")
}
