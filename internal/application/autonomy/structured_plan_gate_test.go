package autonomy

import "testing"

func TestEnforceStructuredPlanGate(t *testing.T) {
	validAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.use","agent_id":"bid-all"}]}`
	if err := EnforceStructuredPlanGate(validAction); err != nil {
		t.Fatalf("valid action_plan should pass: %v", err)
	}

	validBootstrap := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.bootstrap","cwd":"/tmp/work"}]}`
	if err := EnforceStructuredPlanGate(validBootstrap); err != nil {
		t.Fatalf("runtime.bootstrap action_plan should pass: %v", err)
	}

	validService := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.service","service":"clawx-bid-all","operation":"restart","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validService); err != nil {
		t.Fatalf("runtime.service action_plan should pass: %v", err)
	}

	validRelease := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.release","service":"clawx-bid-all","script":"./scripts/deploy_workers.sh","rollback_script":"./scripts/rollback_workers.sh","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validRelease); err != nil {
		t.Fatalf("runtime.release action_plan should pass: %v", err)
	}

	validSupervisor := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.supervisor","service":"clawx-bid-all","operation":"ensure","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validSupervisor); err != nil {
		t.Fatalf("runtime.supervisor action_plan should pass: %v", err)
	}

	validReleaseStatus := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.release.status","service":"clawx-bid-all","operation":"current","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validReleaseStatus); err != nil {
		t.Fatalf("runtime.release.status action_plan should pass: %v", err)
	}

	validTaskStatus := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.status","agent_id":"bid-all","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskStatus); err != nil {
		t.Fatalf("runtime.task.status action_plan should pass: %v", err)
	}

	validTaskDelegate := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.delegate","agent_id":"bid-all","cmd":"go test ./...","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskDelegate); err != nil {
		t.Fatalf("runtime.task.delegate action_plan should pass: %v", err)
	}

	validTaskDelegates := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.delegates","agent_id":"bid-all","parent_task_id":"task-1","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskDelegates); err != nil {
		t.Fatalf("runtime.task.delegates action_plan should pass: %v", err)
	}

	validTaskRetry := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.retry","agent_id":"bid-all","task_id":"t-1","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskRetry); err != nil {
		t.Fatalf("runtime.task.retry action_plan should pass: %v", err)
	}

	validTaskCancel := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.cancel","agent_id":"bid-all","task_id":"t-1","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskCancel); err != nil {
		t.Fatalf("runtime.task.cancel action_plan should pass: %v", err)
	}

	validTaskControl := `{"type":"action_plan","mode":"execute","actions":[{"kind":"runtime.task.control","agent_id":"bid-all","service":"clawx-bid-all","operation":"ensure_running","scope":"user"}]}`
	if err := EnforceStructuredPlanGate(validTaskControl); err != nil {
		t.Fatalf("runtime.task.control action_plan should pass: %v", err)
	}

	invalidAction := `{"type":"action_plan","mode":"execute","actions":[{"kind":"agent.delete","agent_id":"bid-all"}]}`
	if err := EnforceStructuredPlanGate(invalidAction); err == nil {
		t.Fatalf("invalid allowlist action_plan should be rejected")
	}

	validBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"],"evidence_exec_ids":["rexec-1"]}`
	if err := EnforceStructuredPlanGate(validBlocker); err != nil {
		t.Fatalf("valid execution_blocker should pass: %v", err)
	}
	invalidBlocker := `{"type":"execution_blocker","need_user_input":true,"attempted":["a"],"evidence":["b"]}`
	if err := EnforceStructuredPlanGate(invalidBlocker); err == nil {
		t.Fatalf("execution_blocker without evidence_exec_ids should be rejected")
	}

	deprecated := `{"type":"control_plan","intent":"agent.use","target":{"agent_id":"bid-all"},"mode":"execute","risk":"low"}`
	if err := EnforceStructuredPlanGate(deprecated); err == nil {
		t.Fatalf("deprecated control_plan should be rejected")
	}
}

func TestEnforceStructuredPlanGateAcceptsNarrativeWrappedJSON(t *testing.T) {
	text := "我先给出计划：\n```json\n{\"type\":\"action_plan\",\"mode\":\"execute\",\"actions\":[{\"kind\":\"runtime.exec\",\"cmd\":\"echo hi\"}]}\n```\n然后继续执行。"
	if err := EnforceStructuredPlanGate(text); err != nil {
		t.Fatalf("narrative wrapped action_plan should pass: %v", err)
	}
}
