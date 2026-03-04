package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrForbiddenCWD  = errors.New("cwd is outside allowed roots")
)

type Snapshot struct {
	AllowedRoots       []string
	DefaultCWD         string
	Timeout            time.Duration
	DiscordEnabled     bool
	TelegramEnabled    bool
	HealthProbeEnabled bool
}

func LoadFromEnv() (Snapshot, error) {
	allowedRoots := splitList(os.Getenv("SYNAPSEX_ALLOWED_ROOTS"))
	defaultCWD := strings.TrimSpace(os.Getenv("SYNAPSEX_DEFAULT_CWD"))
	if defaultCWD == "" {
		defaultCWD = "."
	}

	timeoutSeconds := 600
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TIMEOUT_SECONDS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Snapshot{}, fmt.Errorf("parse SYNAPSEX_TIMEOUT_SECONDS: %w", err)
		}
		timeoutSeconds = parsed
	}

	snapshot := Snapshot{
		AllowedRoots:       allowedRoots,
		DefaultCWD:         defaultCWD,
		Timeout:            time.Duration(timeoutSeconds) * time.Second,
		DiscordEnabled:     parseBoolOrDefault(os.Getenv("SYNAPSEX_DISCORD_ENABLED"), true),
		TelegramEnabled:    parseBoolOrDefault(os.Getenv("SYNAPSEX_TELEGRAM_ENABLED"), true),
		HealthProbeEnabled: parseBoolOrDefault(os.Getenv("SYNAPSEX_HEALTH_PROBE_ENABLED"), true),
	}

	if len(snapshot.AllowedRoots) == 0 {
		snapshot.AllowedRoots = []string{snapshot.DefaultCWD}
	}

	if err := snapshot.Validate(); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}

func (s Snapshot) Validate() error {
	if s.Timeout <= 0 {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.DefaultCWD) == "" {
		return ErrInvalidConfig
	}
	for _, root := range s.AllowedRoots {
		if strings.TrimSpace(root) == "" {
			return ErrInvalidConfig
		}
	}
	return nil
}

func (s Snapshot) ValidateWorkingDirectory(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		return ErrForbiddenCWD
	}

	target := filepath.Clean(cwd)
	for _, root := range s.AllowedRoots {
		allowed := filepath.Clean(root)
		relative, err := filepath.Rel(allowed, target)
		if err != nil {
			continue
		}
		if relative == "." || (!strings.HasPrefix(relative, "..") && relative != "..") {
			return nil
		}
	}
	return ErrForbiddenCWD
}

func (s Snapshot) IsChannelEnabled(channel string) bool {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "discord":
		return s.DiscordEnabled
	case "telegram":
		return s.TelegramEnabled
	default:
		return false
	}
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	roots := make([]string, 0, len(items))
	for _, item := range items {
		if value := strings.TrimSpace(item); value != "" {
			roots = append(roots, value)
		}
	}
	return roots
}

func parseBoolOrDefault(raw string, fallback bool) bool {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

