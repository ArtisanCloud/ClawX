package intent

import (
	"context"
	"fmt"
	"strings"
	"time"

	skillsinfra "synapsex/internal/infrastructure/skills"
	chatiface "synapsex/internal/interfaces/chat"
)

type ErrorCategory string

const (
	ErrorPermissionDenied ErrorCategory = "permission_denied"
	ErrorSkillNotFound    ErrorCategory = "skill_not_found"
	ErrorSkillDisabled    ErrorCategory = "skill_disabled"
	ErrorSkillInvalid     ErrorCategory = "skill_invalid"
	ErrorPairingExpired   ErrorCategory = "pairing_expired"
)

type RoutedError struct {
	Category ErrorCategory
	Message  string
}

func (e RoutedError) Error() string {
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return string(e.Category)
}

func (e RoutedError) Code() string {
	return string(e.Category)
}

type PermissionPolicy struct {
	Enabled       bool
	AllowUsers    map[string]struct{}
	AllowChannels map[string]struct{}
	DefaultMode   string
	PairingTTL    time.Duration
}

type PairingChecker interface {
	Get(channel, userID string) (skillsinfra.PairingRecord, bool)
}

type PermissionChecker struct {
	policy       PermissionPolicy
	pairingStore PairingChecker
}

func NewPermissionChecker(policy PermissionPolicy, pairingStore PairingChecker) *PermissionChecker {
	return &PermissionChecker{policy: policy, pairingStore: pairingStore}
}

func (c *PermissionChecker) Allow(ctx context.Context, message chatiface.Message, skillName string) error {
	_ = ctx
	if !c.policy.Enabled {
		return RoutedError{
			Category: ErrorPermissionDenied,
			Message:  "skills are disabled",
		}
	}
	if strings.TrimSpace(skillName) == "" {
		return RoutedError{
			Category: ErrorSkillNotFound,
			Message:  "skill is required",
		}
	}

	channelID := inferChannelID(message.ConversationID)
	userID := strings.TrimSpace(message.UserID)
	if _, ok := c.policy.AllowUsers[userID]; ok {
		return nil
	}
	if _, ok := c.policy.AllowChannels[channelID]; ok {
		return nil
	}

	if strings.TrimSpace(strings.ToLower(c.policy.DefaultMode)) == "channel_allowlist_dm_pairing" && message.ContextFlags.IsDirectMessage {
		// Bootstrap-friendly default:
		// when no allowlist is configured at all, allow DM skill usage directly.
		if len(c.policy.AllowUsers) == 0 && len(c.policy.AllowChannels) == 0 {
			return nil
		}
		if c.pairingStore == nil {
			return nil
		}
		record, ok := c.pairingStore.Get(message.Channel, userID)
		if !ok {
			return RoutedError{
				Category: ErrorPairingExpired,
				Message:  "dm pairing not found; re-pair required",
			}
		}
		if record.State == skillsinfra.PairingExpired || record.State == skillsinfra.PairingRevoked {
			return RoutedError{
				Category: ErrorPairingExpired,
				Message:  fmt.Sprintf("dm pairing is %s; re-pair required", record.State),
			}
		}
		return nil
	}

	return RoutedError{
		Category: ErrorPermissionDenied,
		Message:  "permission denied for this channel/user",
	}
}

func inferChannelID(conversationID string) string {
	parts := strings.Split(strings.TrimSpace(conversationID), ":")
	if len(parts) < 2 {
		return "-"
	}
	return strings.TrimSpace(parts[1])
}
