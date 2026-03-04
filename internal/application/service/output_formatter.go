package service

import "strings"

type OutputFormatter struct{}

func NewOutputFormatter() *OutputFormatter {
	return &OutputFormatter{}
}

func (f *OutputFormatter) Format(content string) string {
	formatted := strings.ReplaceAll(content, "\r\n", "\n")
	formatted = strings.TrimSpace(formatted)
	if formatted == "" {
		return ""
	}

	// Keep code blocks readable by closing unbalanced fences.
	if strings.Count(formatted, "```")%2 != 0 {
		formatted += "\n```"
	}
	return formatted
}

