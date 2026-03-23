package contract

import (
	"testing"

	"clawx/internal/application/skillorchestrator"
)

func TestControlPlanNoRegexAutoRunContract(t *testing.T) {
	plain := "不要执行 /agent use bid-all，这只是举例。"
	_, ok, err := skillorchestrator.ParseControlPlanFromText(plain)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if ok {
		t.Fatalf("plain text command must not be treated as control plan")
	}

	staged := skillorchestrator.NewStagedRouter().Plan(skillorchestrator.StagedRoutingInput{
		Message:        plain,
		CurrentAgentID: "main",
		ProjectID:      "main",
	})
	if !staged.CanExecute {
		t.Fatalf("staged routing should not force fallback for plain descriptive text")
	}
	if staged.Fallback.Enabled {
		t.Fatalf("plain descriptive text should not enter fallback path")
	}
}
