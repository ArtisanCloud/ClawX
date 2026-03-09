package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func loadDotEnvIfPresent(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open dotenv file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for lineNo := 1; scanner.Scan(); lineNo++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("parse dotenv file line %d: invalid assignment", lineNo)
		}

		key = strings.TrimSpace(key)
		if key == "" {
			return fmt.Errorf("parse dotenv file line %d: empty key", lineNo)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value = parseDotEnvValue(value)
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set dotenv key %s: %w", key, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan dotenv file: %w", err)
	}
	return nil
}

func parseDotEnvValue(raw string) string {
	value := strings.TrimSpace(raw)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}

	if commentIndex := strings.Index(value, " #"); commentIndex >= 0 {
		value = strings.TrimSpace(value[:commentIndex])
	}
	return value
}
