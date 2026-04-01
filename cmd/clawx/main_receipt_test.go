package main

import (
	"errors"
	"strings"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestFormatTaskReceiptProgress(t *testing.T) {
	got := formatTaskReceipt(taskReceiptProgress, "正在思考并执行。")
	if got != "处理中：正在思考并执行。" {
		t.Fatalf("unexpected progress receipt: %s", got)
	}
}

func TestFormatTaskReceiptCompleteDefault(t *testing.T) {
	got := formatTaskReceipt(taskReceiptComplete, "")
	if got != "处理完成。" {
		t.Fatalf("unexpected complete receipt: %s", got)
	}
}

func TestFormatExecutionFailureResponseNoEscalation(t *testing.T) {
	got := formatExecutionFailureResponse(errors.New("dial tcp: connection refused"), nil, false)
	if !strings.Contains(got, "处理失败") {
		t.Fatalf("expected failed receipt prefix, got: %s", got)
	}
	if strings.Contains(got, "已尝试:") {
		t.Fatalf("unexpected escalation prompt for recoverable non-exhausted error: %s", got)
	}
}

func TestFormatExecutionFailureResponseEscalation(t *testing.T) {
	got := formatExecutionFailureResponse(errors.New("permission denied"), []string{"检查路径"}, false)
	if !strings.Contains(got, "处理失败") {
		t.Fatalf("expected failed receipt prefix, got: %s", got)
	}
	if !strings.Contains(got, "已尝试:") {
		t.Fatalf("expected escalation prompt details, got: %s", got)
	}
}

func TestApplyAutonomySelfHealGateBlocksNoEvidenceDependencyAsk(t *testing.T) {
	decision := serviceDecisionExecuteForTest("继续安装依赖")
	raw := `{
  "type":"execution_blocker",
  "need_user_input":true,
  "blocker_class":"network",
  "attempted":[],
  "evidence":[],
  "recommendation":"请提供内网镜像",
  "question":"是否提供镜像地址？"
}`
	got := applyAutonomySelfHealGate(decision, raw)
	if got == raw {
		t.Fatalf("expected self-heal gate to block ask-without-evidence")
	}
	if !strings.Contains(got, "缺少结构化执行证据") {
		t.Fatalf("expected self-heal guard message, got: %s", got)
	}
}

func TestApplyAutonomySelfHealGateAllowsWithEvidence(t *testing.T) {
	decision := serviceDecisionExecuteForTest("继续安装依赖")
	execID := newRuntimeExecID()
	_ = appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:         execID,
		ConversationID: decision.ConversationID,
		AgentID:        "main",
		Command:        "pip install -e . --index-url https://pypi.org/simple",
		Success:        false,
		ExitCode:       1,
		OutputDigest:   digestOutput("Name or service not known"),
		OutputPreview:  "Name or service not known",
	})
	raw := "```json\n{\n  \"type\":\"execution_blocker\",\n  \"need_user_input\":true,\n  \"blocker_class\":\"network\",\n  \"attempted\":[\"pip install -r requirements.txt -i https://pypi.org/simple\"],\n  \"evidence\":[\"结果: Name or service not known\"],\n  \"evidence_exec_ids\":[\"" + execID + "\"],\n  \"recommendation\":\"提供内网镜像\",\n  \"question\":\"是否提供镜像地址？\"\n}\n```"
	got := applyAutonomySelfHealGate(decision, raw)
	if got != raw {
		t.Fatalf("expected pass-through when evidence exists, got: %s", got)
	}
}

func TestApplyAutonomySelfHealGateBlocksWithoutExecIDs(t *testing.T) {
	decision := serviceDecisionExecuteForTest("继续安装依赖")
	raw := `{
  "type":"execution_blocker",
  "need_user_input":true,
  "blocker_class":"network",
  "attempted":["pip install -e . --index-url https://pypi.org/simple"],
  "evidence":["Name or service not known"],
  "recommendation":"请提供镜像",
  "question":"是否提供镜像地址？"
}`
	got := applyAutonomySelfHealGate(decision, raw)
	if !strings.Contains(got, "没有附上本轮执行证据") {
		t.Fatalf("expected exec id guard, got: %s", got)
	}
}

func TestApplyAutonomySelfHealGateBlocksInvalidExecIDs(t *testing.T) {
	decision := serviceDecisionExecuteForTest("继续安装依赖")
	raw := `{
  "type":"execution_blocker",
  "need_user_input":true,
  "blocker_class":"network",
  "attempted":["pip install -e . --index-url https://pypi.org/simple"],
  "evidence":["Name or service not known"],
  "evidence_exec_ids":["rexec-missing"],
  "recommendation":"请提供镜像",
  "question":"是否提供镜像地址？"
}`
	got := applyAutonomySelfHealGate(decision, raw)
	if !strings.Contains(got, "不是当前会话生成的") {
		t.Fatalf("expected invalid exec id guard, got: %s", got)
	}
}

func serviceDecisionExecuteForTest(text string) service.Decision {
	return service.Decision{
		ConversationID: "conv-main-receipt",
		Kind:           service.DecisionExecute,
		Message: chatiface.Message{
			Text: text,
		},
	}
}
