package skillorchestrator

import "testing"

func TestParseRequirementSyncPlanFromText_JSON(t *testing.T) {
	text := `{"type":"requirement_sync","intent":"requirement.update","mode":"execute","requirement":"新增招投标抓取去重规则"}`
	plan, ok, err := ParseRequirementSyncPlanFromText(text)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected plan parsed")
	}
	if plan.Requirement != "新增招投标抓取去重规则" {
		t.Fatalf("unexpected requirement: %q", plan.Requirement)
	}
}

func TestParseRequirementSyncPlanFromText_FencedJSON(t *testing.T) {
	text := "说明\n```json\n{\"type\":\"requirement_sync\",\"intent\":\"requirement.update\",\"mode\":\"execute\",\"requirement\":\"补充验收标准\"}\n```"
	_, ok, err := ParseRequirementSyncPlanFromText(text)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected fenced json to parse")
	}
}

func TestParseRequirementSyncPlanFromText_SuggestMode(t *testing.T) {
	text := `{"type":"requirement_sync","intent":"requirement.update","mode":"suggest","agent_id":"bid-all","requirement":"补充验收标准","reason":"建议先确认目标智能体"}`
	plan, ok, err := ParseRequirementSyncPlanFromText(text)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if !ok {
		t.Fatalf("expected suggest mode plan parsed")
	}
	if plan.Mode != "suggest" {
		t.Fatalf("expected suggest mode, got %q", plan.Mode)
	}
}

func TestParseRequirementSyncPlanFromText_Invalid(t *testing.T) {
	text := `{"type":"control_plan","intent":"agent.use","mode":"execute"}`
	_, ok, err := ParseRequirementSyncPlanFromText(text)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if ok {
		t.Fatalf("expected invalid requirement plan to be ignored")
	}
}
