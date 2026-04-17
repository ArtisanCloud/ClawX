package autonomy

import (
	"encoding/json"
	"regexp"
	"strings"
)

var structuredPayloadBlockPattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\\})\\s*```")

func extractStructuredPayloads(text string) []map[string]any {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}
	candidates := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)
	appendCandidate := func(raw string) {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			return
		}
		if _, ok := seen[raw]; ok {
			return
		}
		seen[raw] = struct{}{}
		candidates = append(candidates, raw)
	}

	if matches := structuredPayloadBlockPattern.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			appendCandidate(match[1])
		}
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		appendCandidate(trimmed)
	}
	for _, snippet := range extractJSONObjectSnippets(trimmed) {
		appendCandidate(snippet)
	}

	payloads := make([]map[string]any, 0, len(candidates))
	for _, candidate := range candidates {
		var payload map[string]any
		if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
			continue
		}
		payloads = append(payloads, payload)
	}
	return payloads
}

func extractJSONObjectSnippets(text string) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	out := make([]string, 0, 4)
	depth := 0
	start := -1
	inString := false
	escaped := false

	for idx := 0; idx < len(text); idx++ {
		ch := text[idx]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			if depth == 0 {
				start = idx
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 && idx+1 <= len(text) {
				out = append(out, text[start:idx+1])
				start = -1
			}
		}
	}
	return out
}
