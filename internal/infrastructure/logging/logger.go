package logging

import (
	"encoding/json"
	"io"
	"log"
	"sort"
	"strings"
	"time"
)

type Logger struct {
	std *log.Logger
}

func New(writer io.Writer) *Logger {
	return &Logger{
		std: log.New(writer, "", 0),
	}
}

func (l *Logger) Info(message string, fields map[string]any) {
	l.write("info", message, fields)
}

func (l *Logger) Error(message string, fields map[string]any) {
	l.write("error", message, fields)
}

func (l *Logger) write(level, message string, fields map[string]any) {
	record := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"level":     level,
		"message":   message,
	}

	keys := make([]string, 0, len(fields))
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		record[key] = sanitizeValue(key, fields[key])
	}

	payload, err := json.Marshal(record)
	if err != nil {
		l.std.Printf(`{"timestamp":"%s","level":"error","message":"marshal log payload failed"}`, time.Now().UTC().Format(time.RFC3339Nano))
		return
	}
	l.std.Println(string(payload))
}

func sanitizeValue(key string, value any) any {
	lowerKey := strings.ToLower(key)
	switch lowerKey {
	case "prompt_tokens", "prompt_cached_tokens", "completion_tokens", "total_tokens":
		return value
	}
	if strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "secret") || strings.Contains(lowerKey, "password") {
		return "[REDACTED]"
	}
	return value
}
