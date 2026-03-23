package main

import (
	"path/filepath"
	"testing"
)

func TestConversationAgentOverridesPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent_overrides.json")
	scope := routingScopeKey("discord", "default", "conv-1")

	overrides := newConversationAgentOverrides()
	if err := overrides.EnablePersistence(path); err != nil {
		t.Fatalf("enable persistence: %v", err)
	}
	overrides.Set(scope, "bid-all")

	reloaded := newConversationAgentOverrides()
	if err := reloaded.EnablePersistence(path); err != nil {
		t.Fatalf("reload persistence: %v", err)
	}
	agentID, ok := reloaded.Get(scope)
	if !ok || agentID != "bid-all" {
		t.Fatalf("expected persisted override, got ok=%v agent=%q", ok, agentID)
	}

	if !reloaded.Clear(scope) {
		t.Fatalf("expected clear to remove override")
	}
	reloadedAgain := newConversationAgentOverrides()
	if err := reloadedAgain.EnablePersistence(path); err != nil {
		t.Fatalf("reload after clear: %v", err)
	}
	if _, ok := reloadedAgain.Get(scope); ok {
		t.Fatalf("expected cleared override not to persist")
	}
}
