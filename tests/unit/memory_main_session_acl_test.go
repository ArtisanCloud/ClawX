package unit

import (
	"testing"

	memoryapp "clawx/internal/application/memory"
	memorydomain "clawx/internal/domain/memory"
)

func TestMemorySessionClassifierMainAndSharedACL(t *testing.T) {
	owners := []string{"owner-1", "owner-2"}

	t.Run("direct owner allowed becomes main", func(t *testing.T) {
		result := memoryapp.ClassifySession(memoryapp.SessionClassInput{
			RouteKey:        "telegram:default:direct:owner-1",
			UserID:          "owner-1",
			IsDirectMessage: true,
			OwnerAllowlist:  owners,
		})
		if result.ChatMode != memorydomain.ChatModeMain {
			t.Fatalf("expected main chat mode, got %s", result.ChatMode)
		}
		if !result.AllowMainPrivate || result.Degraded {
			t.Fatalf("expected allowMainPrivate=true and degraded=false, got %+v", result)
		}
	})

	t.Run("shared channel stays shared", func(t *testing.T) {
		result := memoryapp.ClassifySession(memoryapp.SessionClassInput{
			RouteKey:        "telegram:default:channel:g-1",
			UserID:          "owner-1",
			IsDirectMessage: false,
			OwnerAllowlist:  owners,
		})
		if result.ChatMode != memorydomain.ChatModeShared {
			t.Fatalf("expected shared mode, got %s", result.ChatMode)
		}
		if result.AllowMainPrivate {
			t.Fatalf("shared mode should not allow main private")
		}
	})

	t.Run("direct non-owner denied", func(t *testing.T) {
		result := memoryapp.ClassifySession(memoryapp.SessionClassInput{
			RouteKey:        "telegram:default:direct:user-x",
			UserID:          "user-x",
			IsDirectMessage: true,
			OwnerAllowlist:  owners,
		})
		if result.ChatMode != memorydomain.ChatModeShared {
			t.Fatalf("expected shared mode for non-owner")
		}
		if result.AllowMainPrivate {
			t.Fatalf("non-owner should not access main private")
		}
		if result.Degraded {
			t.Fatalf("non-owner should be strict deny, not degraded")
		}
	})

	t.Run("direct route/user conflict degrades", func(t *testing.T) {
		result := memoryapp.ClassifySession(memoryapp.SessionClassInput{
			RouteKey:        "telegram:default:direct:owner-1",
			UserID:          "owner-2",
			IsDirectMessage: true,
			OwnerAllowlist:  owners,
		})
		if !result.Degraded {
			t.Fatalf("expected degraded mode on identity conflict")
		}
		if result.ChatMode != memorydomain.ChatModeShared {
			t.Fatalf("degraded conflict should fallback to shared")
		}
	})
}

func TestApplyLoaderACL(t *testing.T) {
	profile := memorydomain.MemoryProfile{
		ScopeKey: memorydomain.MemoryScopeKey{
			AgentID:   "main",
			ProjectID: "memo",
			RouteKey:  "telegram:default:direct:owner-1",
			ChatMode:  memorydomain.ChatModeMain,
		},
		TokenBudget:      2048,
		AllowMainPrivate: true,
	}

	acl := memoryapp.ApplyLoaderACL(memoryapp.LoaderACLInput{
		Profile:         profile,
		RouteKey:        "telegram:default:direct:owner-1",
		UserID:          "owner-2",
		IsDirectMessage: true,
		OwnerAllowlist:  []string{"owner-1", "owner-2"},
	})
	if acl.Profile.ACLMode != memorydomain.ACLModeDegraded {
		t.Fatalf("expected degraded ACL mode, got %s", acl.Profile.ACLMode)
	}
	if acl.Profile.AllowMainPrivate {
		t.Fatalf("degraded policy should disable main private")
	}
}
