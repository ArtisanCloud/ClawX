package intent

import "strings"

type ExplicitSkill struct {
	Name  string
	Input string
	Valid bool
}

func ParseExplicitSkill(text string) ExplicitSkill {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return ExplicitSkill{}
	}
	command := normalizeCommand(fields[0])
	if command != "skill" && command != "sx-skill" {
		return ExplicitSkill{}
	}
	if len(fields) < 2 {
		return ExplicitSkill{}
	}
	name := normalizeName(fields[1])
	if name == "" {
		return ExplicitSkill{}
	}
	input := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), fields[0]))
	input = strings.TrimSpace(strings.TrimPrefix(input, fields[1]))
	return ExplicitSkill{
		Name:  name,
		Input: input,
		Valid: true,
	}
}

func normalizeCommand(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	return strings.TrimPrefix(value, "/")
}
