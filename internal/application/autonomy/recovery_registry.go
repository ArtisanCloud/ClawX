package autonomy

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type RecoveryPolicy struct {
	Name        string
	Class       FailureClass
	Retryable   bool
	MaxRetries  int
	BaseBackoff time.Duration
	MaxDuration time.Duration
}

func (p RecoveryPolicy) Normalize() RecoveryPolicy {
	normalized := p
	normalized.Name = strings.TrimSpace(normalized.Name)
	if normalized.MaxRetries < 0 {
		normalized.MaxRetries = 0
	}
	if normalized.MaxDuration <= 0 {
		normalized.MaxDuration = 30 * time.Second
	}
	return normalized
}

func (p RecoveryPolicy) BackoffForAttempt(attempt int) time.Duration {
	if attempt < 1 || p.BaseBackoff <= 0 {
		return 0
	}
	backoff := p.BaseBackoff
	for i := 1; i < attempt; i++ {
		backoff *= 2
	}
	return backoff
}

type RecoveryRegistry struct {
	mu       sync.RWMutex
	policies map[FailureClass]RecoveryPolicy
}

func NewRecoveryRegistry() *RecoveryRegistry {
	return &RecoveryRegistry{
		policies: make(map[FailureClass]RecoveryPolicy),
	}
}

func (r *RecoveryRegistry) Register(policy RecoveryPolicy) error {
	policy = policy.Normalize()
	if policy.Class == "" {
		return fmt.Errorf("recovery policy class is required")
	}
	if policy.Name == "" {
		policy.Name = string(policy.Class) + "_recovery"
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.policies[policy.Class] = policy
	return nil
}

func (r *RecoveryRegistry) Resolve(class FailureClass) (RecoveryPolicy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	policy, ok := r.policies[class]
	if !ok {
		return RecoveryPolicy{}, false
	}
	return policy, true
}

func NewDefaultRecoveryRegistry() *RecoveryRegistry {
	registry := NewRecoveryRegistry()
	_ = registry.Register(RecoveryPolicy{
		Name:        "network_retry",
		Class:       FailureClassNetwork,
		Retryable:   true,
		MaxRetries:  3,
		BaseBackoff: time.Second,
		MaxDuration: 30 * time.Second,
	})
	_ = registry.Register(RecoveryPolicy{
		Name:        "tool_retry",
		Class:       FailureClassTool,
		Retryable:   true,
		MaxRetries:  2,
		BaseBackoff: 500 * time.Millisecond,
		MaxDuration: 30 * time.Second,
	})
	_ = registry.Register(RecoveryPolicy{
		Name:        "resource_retry",
		Class:       FailureClassResource,
		Retryable:   true,
		MaxRetries:  1,
		BaseBackoff: 1500 * time.Millisecond,
		MaxDuration: 20 * time.Second,
	})
	_ = registry.Register(RecoveryPolicy{
		Name:        "permission_block",
		Class:       FailureClassPermission,
		Retryable:   false,
		MaxRetries:  0,
		BaseBackoff: 0,
		MaxDuration: 5 * time.Second,
	})
	return registry
}
