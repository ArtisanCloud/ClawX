package unit

import (
	"fmt"
	"sync"
	"testing"

	"clawx/internal/application/configplan"
)

func TestConfigPlanMemoryStoreConcurrentSet(t *testing.T) {
	store := configplan.NewMemoryStore()
	const workers = 32
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			plan := configplan.Plan{
				Kind:           configplan.KindSetDefaultAgent,
				ConversationID: fmt.Sprintf("conv-%d", idx),
				DefaultAgentID: "main",
			}
			if err := store.Set(plan.ConversationID, plan); err != nil {
				t.Errorf("set %d: %v", idx, err)
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < workers; i++ {
		if _, ok := store.Get(fmt.Sprintf("conv-%d", i)); !ok {
			t.Fatalf("missing plan for conv-%d", i)
		}
	}
}
