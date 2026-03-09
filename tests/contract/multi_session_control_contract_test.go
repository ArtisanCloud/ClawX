package contract

import "testing"

func TestMultiSessionControlCommandContractSkeleton(t *testing.T) {
	t.Run("command_semantics_placeholder", func(t *testing.T) {
		t.Skip("TODO(Phase2/US2): verify /new /resume /switch /list /current /cancel semantics")
	})

	t.Run("error_contract_placeholder", func(t *testing.T) {
		t.Skip("TODO(Phase2/US2): verify missing/invalid/invisible session_id error contracts")
	})
}
