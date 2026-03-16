package integration

import (
	"testing"
	"time"

	skillsinfra "clawx/internal/infrastructure/skills"
)

func TestDMPairingLifecycleTransitions(t *testing.T) {
	store, err := skillsinfra.NewPairingStore(t.TempDir() + "/pairing.json")
	if err != nil {
		t.Fatalf("new pairing store: %v", err)
	}

	expiresAt := time.Now().UTC().Add(time.Minute)
	if err := store.Upsert(skillsinfra.PairingRecord{
		Channel:   "discord",
		UserID:    "user-1",
		State:     skillsinfra.PairingPending,
		ExpiresAt: &expiresAt,
	}); err != nil {
		t.Fatalf("upsert pending: %v", err)
	}

	if err := store.MarkState("discord", "user-1", skillsinfra.PairingPaired, "", &expiresAt); err != nil {
		t.Fatalf("mark paired: %v", err)
	}
	record, ok := store.Get("discord", "user-1")
	if !ok || record.State != skillsinfra.PairingPaired {
		t.Fatalf("expected paired state, got=%v ok=%v", record.State, ok)
	}

	if err := store.MarkState("discord", "user-1", skillsinfra.PairingRevoked, "manual revoke", nil); err != nil {
		t.Fatalf("mark revoked: %v", err)
	}
	record, ok = store.Get("discord", "user-1")
	if !ok || record.State != skillsinfra.PairingRevoked {
		t.Fatalf("expected revoked state, got=%v ok=%v", record.State, ok)
	}
}
