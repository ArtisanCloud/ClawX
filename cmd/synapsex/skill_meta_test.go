package main

import (
	"strings"
	"testing"

	"synapsex/internal/application/service"
)

func TestHandleSynapseXSkillMetaCommand(t *testing.T) {
	runtime := agentRuntime{}

	handled, response := handleSynapseXSkillMetaCommand(runtime, "/sx-skills")
	if !handled {
		t.Fatalf("expected sx-skills to be handled")
	}
	if response == "" {
		t.Fatalf("expected sx-skills response")
	}

	handled, _ = handleSynapseXSkillMetaCommand(runtime, "hello")
	if handled {
		t.Fatalf("unexpected handled for non sx command")
	}
}

func TestApplyExecutionSourceLabel(t *testing.T) {
	skillText := applyExecutionSourceLabel(service.Decision{Kind: service.DecisionSkill}, "result")
	if !strings.HasPrefix(skillText, "[SynapseX Skill]") {
		t.Fatalf("expected skill prefix, got %q", skillText)
	}

	executeText := applyExecutionSourceLabel(service.Decision{Kind: service.DecisionExecute}, "result")
	if !strings.HasPrefix(executeText, "[Agent Direct]") {
		t.Fatalf("expected execute prefix, got %q", executeText)
	}
}
