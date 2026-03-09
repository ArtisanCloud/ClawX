package integration

import (
	"context"
	"testing"
	"time"

	"synapsex/internal/application/intent"
	skillsinfra "synapsex/internal/infrastructure/skills"
	chatiface "synapsex/internal/interfaces/chat"
)

func TestSkillPermissionPolicyDefaultAndDisabled(t *testing.T) {
	stack := newSkillTestStack(t, func(root string) error {
		writeSkillFile(t, root, "user-skills/echo/SKILL.md", `---
name: echo
description: echo text
---
echo`)
		return nil
	}, []string{"echo"})

	disabledMessage := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          "user-1",
		Text:            "/skill echo hello",
		IsDirectMessage: true,
		IsAllowed:       true,
	})
	_, err := stack.router.Route(context.Background(), disabledMessage)
	if err == nil {
		t.Fatalf("expected disabled skill error")
	}
	if category := errorCategory(err); category != "skill_disabled" {
		t.Fatalf("unexpected error category: %s", category)
	}

	checker := intent.NewPermissionChecker(intent.PermissionPolicy{
		Enabled:       true,
		AllowUsers:    map[string]struct{}{},
		AllowChannels: map[string]struct{}{},
		DefaultMode:   "channel_allowlist_dm_pairing",
		PairingTTL:    24 * time.Hour,
	}, nil)
	noPermission := mustNormalizeMessage(t, chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          "user-2",
		Text:            "/skill echo hello",
		IsDirectMessage: false,
		IsAllowed:       true,
	})
	err = checker.Allow(context.Background(), noPermission, "echo")
	if err == nil {
		t.Fatalf("expected permission denied")
	}
	if category := errorCategory(err); category != "permission_denied" {
		t.Fatalf("unexpected permission category: %s", category)
	}
}

func TestSkillPermissionPolicyPairingExpired(t *testing.T) {
	storePath := t.TempDir() + "/pairing.json"
	store, err := skillsinfra.NewPairingStore(storePath)
	if err != nil {
		t.Fatalf("new pairing store: %v", err)
	}

	expiresAt := time.Now().UTC().Add(-time.Minute)
	if err := store.Upsert(skillsinfra.PairingRecord{
		Channel:   "discord",
		UserID:    "user-9",
		State:     skillsinfra.PairingPaired,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("upsert pairing: %v", err)
	}
	if err := store.ExpireDue(time.Now().UTC()); err != nil {
		t.Fatalf("expire pairing: %v", err)
	}

	checker := intent.NewPermissionChecker(intent.PermissionPolicy{
		Enabled:       true,
		AllowUsers:    map[string]struct{}{"other-user": {}},
		AllowChannels: map[string]struct{}{},
		DefaultMode:   "channel_allowlist_dm_pairing",
		PairingTTL:    24 * time.Hour,
	}, store)

	message := chatiface.Message{
		ConversationID: "discord:-:-:user-9",
		UserID:         "user-9",
		Channel:        "discord",
		ContextFlags: chatiface.ContextFlags{
			IsDirectMessage: true,
			IsAllowed:       true,
		},
	}
	err = checker.Allow(context.Background(), message, "echo")
	if err == nil {
		t.Fatalf("expected pairing expired error")
	}
	if category := errorCategory(err); category != "pairing_expired" {
		t.Fatalf("unexpected pairing category: %s", category)
	}
}

func TestSkillPermissionPolicyDMAllowWhenAllowlistEmpty(t *testing.T) {
	checker := intent.NewPermissionChecker(intent.PermissionPolicy{
		Enabled:       true,
		AllowUsers:    map[string]struct{}{},
		AllowChannels: map[string]struct{}{},
		DefaultMode:   "channel_allowlist_dm_pairing",
		PairingTTL:    24 * time.Hour,
	}, nil)

	message := chatiface.Message{
		ConversationID: "discord:-:1472233703659802706:user-1",
		UserID:         "user-1",
		Channel:        "discord",
		ContextFlags: chatiface.ContextFlags{
			IsDirectMessage: true,
			IsAllowed:       true,
		},
	}
	if err := checker.Allow(context.Background(), message, "echo"); err != nil {
		t.Fatalf("expected dm allow when allowlist empty, got %v", err)
	}
}

func errorCategory(err error) string {
	type coded interface {
		Code() string
	}
	if err == nil {
		return ""
	}
	if value, ok := err.(coded); ok {
		return value.Code()
	}
	return ""
}
