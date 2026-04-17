package autonomy

import (
	"testing"
	"time"
)

func TestRecoveryRegistry_RegisterAndResolve(t *testing.T) {
	registry := NewRecoveryRegistry()
	err := registry.Register(RecoveryPolicy{
		Name:        "network_retry",
		Class:       FailureClassNetwork,
		Retryable:   true,
		MaxRetries:  3,
		BaseBackoff: time.Second,
	})
	if err != nil {
		t.Fatalf("register policy failed: %v", err)
	}
	got, ok := registry.Resolve(FailureClassNetwork)
	if !ok {
		t.Fatalf("expected network policy")
	}
	if got.MaxRetries != 3 || got.BaseBackoff != time.Second || !got.Retryable {
		t.Fatalf("unexpected policy: %#v", got)
	}
}

func TestDefaultRecoveryRegistry_Strategies(t *testing.T) {
	registry := NewDefaultRecoveryRegistry()

	network, ok := registry.Resolve(FailureClassNetwork)
	if !ok || network.MaxRetries != 3 || network.BaseBackoff != time.Second || !network.Retryable {
		t.Fatalf("unexpected network strategy: %#v", network)
	}
	tool, ok := registry.Resolve(FailureClassTool)
	if !ok || tool.MaxRetries != 2 || tool.BaseBackoff != 500*time.Millisecond || !tool.Retryable {
		t.Fatalf("unexpected tool strategy: %#v", tool)
	}
	resource, ok := registry.Resolve(FailureClassResource)
	if !ok || resource.MaxRetries != 1 || resource.BaseBackoff != 1500*time.Millisecond || !resource.Retryable {
		t.Fatalf("unexpected resource strategy: %#v", resource)
	}
	permission, ok := registry.Resolve(FailureClassPermission)
	if !ok || permission.Retryable || permission.MaxRetries != 0 {
		t.Fatalf("unexpected permission strategy: %#v", permission)
	}
	unknown, ok := registry.Resolve(FailureClassUnknown)
	if !ok || !unknown.Retryable || unknown.MaxRetries != 1 {
		t.Fatalf("unexpected unknown strategy: %#v", unknown)
	}
}

func TestRecoveryPolicyBackoffForAttempt(t *testing.T) {
	policy := RecoveryPolicy{BaseBackoff: time.Second}
	if got := policy.BackoffForAttempt(1); got != time.Second {
		t.Fatalf("attempt 1 backoff: got %v", got)
	}
	if got := policy.BackoffForAttempt(2); got != 2*time.Second {
		t.Fatalf("attempt 2 backoff: got %v", got)
	}
	if got := policy.BackoffForAttempt(3); got != 4*time.Second {
		t.Fatalf("attempt 3 backoff: got %v", got)
	}
}
