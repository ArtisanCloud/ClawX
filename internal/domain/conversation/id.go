package conversation

import (
	"errors"
	"fmt"
	"strings"
)

var ErrInvalidConversationParts = errors.New("invalid conversation parts")

type Parts struct {
	Channel  string
	GuildID  string
	ThreadID string
	UserID   string
}

func BuildID(parts Parts) (string, error) {
	channel := strings.TrimSpace(parts.Channel)
	userID := strings.TrimSpace(parts.UserID)
	if channel == "" || userID == "" {
		return "", ErrInvalidConversationParts
	}

	guildID := normalizeOptional(parts.GuildID)
	threadID := normalizeOptional(parts.ThreadID)

	return fmt.Sprintf("%s:%s:%s:%s", channel, guildID, threadID, userID), nil
}

func normalizeOptional(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	return value
}

