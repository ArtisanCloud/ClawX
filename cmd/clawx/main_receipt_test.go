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
	if got != "处理中：正在思考并执行。\n完成状态：进行中。" {
		t.Fatalf("unexpected progress receipt: %s", got)
	}
}

func TestFormatTaskReceiptCompleteDefault(t *testing.T) {
	got := formatTaskReceipt(taskReceiptComplete, "")
	if got != "处理完成。\n完成状态：已完成。" {
		t.Fatalf("unexpected complete receipt: %s", got)
	}
}

func TestFormatTaskReceiptReceivedWithStatus(t *testing.T) {
	got := formatTaskReceipt(taskReceiptReceived, "已识别为执行请求。")
	if got != "已接收，开始处理：已识别为执行请求。\n完成状态：已接收。" {
		t.Fatalf("unexpected received receipt: %s", got)
	}
}

func TestFormatExecutionFailureResponseNoEscalation(t *testing.T) {
	got := formatExecutionFailureResponse(errors.New("dial tcp: connection refused"), nil, false)
	if !strings.Contains(got, "处理失败") {
		t.Fatalf("expected failed receipt prefix, got: %s", got)
	}
	if !strings.Contains(got, "完成状态：失败。") {
		t.Fatalf("expected non-escalation failed status line, got: %s", got)
	}
	if !strings.Contains(got, "下一步：可直接发送“继续”或“重试”按当前进度重跑") {
		t.Fatalf("expected non-escalation next-step guidance, got: %s", got)
	}
	if strings.Contains(got, "已尝试动作：") {
		t.Fatalf("unexpected escalation prompt for recoverable non-exhausted error: %s", got)
	}
}

func TestFormatExecutionFailureResponseEscalation(t *testing.T) {
	got := formatExecutionFailureResponse(errors.New("permission denied"), []string{"检查路径"}, false)
	if !strings.Contains(got, "处理失败") {
		t.Fatalf("expected failed receipt prefix, got: %s", got)
	}
	if !strings.Contains(got, "完成状态：失败（待确认）。") {
		t.Fatalf("expected escalation status line, got: %s", got)
	}
	if !strings.Contains(got, "已尝试动作：") {
		t.Fatalf("expected escalation prompt details, got: %s", got)
	}
	if !strings.Contains(got, "恢复动作：") {
		t.Fatalf("expected escalation recovery action field, got: %s", got)
	}
	if !strings.Contains(got, "下一步：请确认是否按恢复动作继续，我会基于你的选择续跑。") {
		t.Fatalf("expected escalation next-step guidance, got: %s", got)
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
	if strings.Contains(got, `"type":"execution_blocker"`) {
		t.Fatalf("expected blocker payload stripped from user-facing output, got: %s", got)
	}
	if !strings.Contains(got, "自动恢复已穷尽，需要你确认后继续") || !strings.Contains(got, "关键证据") || !strings.Contains(got, "确认问题：") {
		t.Fatalf("expected readable blocker prompt, got: %s", got)
	}
	if !strings.Contains(got, "完成状态：失败（待确认）。") {
		t.Fatalf("expected blocker status line, got: %s", got)
	}
	if !strings.Contains(got, "下一步：请直接确认上面的决策问题，我会按你的选择继续执行。") {
		t.Fatalf("expected blocker next-step guidance, got: %s", got)
	}
}

func TestApplyAutonomySelfHealGateIncludesExecutionSourceContext(t *testing.T) {
	decision := serviceDecisionExecuteForTest("继续安装依赖")
	execID := newRuntimeExecID()
	_ = appendRuntimeExecAttestation(runtimeExecAttestationRecord{
		ExecID:                            execID,
		ConversationID:                    decision.ConversationID,
		AgentID:                           "main",
		Command:                           "pip install -r requirements.txt",
		Success:                           false,
		ExitCode:                          1,
		OutputDigest:                      digestOutput("Name or service not known"),
		OutputPreview:                     "Name or service not known",
		RuntimeExecDecisionMode:           "build.continue",
		RuntimeExecDecisionApplySource:    "persisted_state",
		RuntimeExecDecisionLockSource:     "user_phrase",
		RuntimeExecDecisionFallbackSource: "workspace",
	})
	raw := "```json\n{\n  \"type\":\"execution_blocker\",\n  \"need_user_input\":true,\n  \"blocker_class\":\"network\",\n  \"attempted\":[\"pip install -r requirements.txt -i https://pypi.org/simple\"],\n  \"evidence\":[\"结果: Name or service not known\"],\n  \"evidence_exec_ids\":[\"" + execID + "\"],\n  \"recommendation\":\"提供内网镜像\",\n  \"question\":\"是否提供镜像地址？\"\n}\n```"
	got := applyAutonomySelfHealGate(decision, raw)
	if !strings.Contains(got, "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=workspace") {
		t.Fatalf("expected execution source context in blocker prompt, got: %s", got)
	}
}

func TestBuildRuntimeExecDecisionContextLineFromAttestations(t *testing.T) {
	records := []runtimeExecAttestationRecord{
		{
			RuntimeExecDecisionMode:           "build.continue",
			RuntimeExecDecisionApplySource:    "persisted_state",
			RuntimeExecDecisionLockSource:     "user_phrase",
			RuntimeExecDecisionFallbackSource: "workspace",
		},
		{
			RuntimeExecDecisionMode:           "build.continue",
			RuntimeExecDecisionApplySource:    "persisted_state",
			RuntimeExecDecisionLockSource:     "user_phrase",
			RuntimeExecDecisionFallbackSource: "env",
		},
		{
			RuntimeExecDecisionMode:        "build.continue",
			RuntimeExecDecisionApplySource: "persisted_state",
			RuntimeExecDecisionSource:      "user_phrase",
		},
	}
	got := buildRuntimeExecDecisionContextLineFromAttestations(records)
	want := "执行来源：mode=build.continue apply_source=persisted_state lock_source=user_phrase fallback_source=env,workspace"
	if got != want {
		t.Fatalf("unexpected decision context line: got=%q want=%q", got, want)
	}
}

func TestParseExecutionBlockerPlanNarrativeWrapped(t *testing.T) {
	raw := "我需要一个决策：\n```json\n{\n  \"type\":\"execution_blocker\",\n  \"need_user_input\":true,\n  \"blocker_class\":\"network\",\n  \"attempted\":[\"pip install -r requirements.txt\"],\n  \"evidence\":[\"Name or service not known\"],\n  \"evidence_exec_ids\":[\"rexec-1\"],\n  \"recommendation\":\"提供镜像\",\n  \"question\":\"是否提供镜像地址？\"\n}\n```\n请确认。"
	plan, ok := parseExecutionBlockerPlan(raw)
	if !ok {
		t.Fatalf("expected parse success for narrative wrapped execution_blocker")
	}
	if !plan.NeedUserInput || len(plan.EvidenceExecIDs) != 1 || plan.EvidenceExecIDs[0] != "rexec-1" {
		t.Fatalf("unexpected parsed blocker: %+v", plan)
	}
}

func TestStripExecutionBlockerPayloadInlineJSON(t *testing.T) {
	raw := "诊断结果：{\"type\":\"execution_blocker\",\"need_user_input\":true,\"blocker_class\":\"network\",\"attempted\":[\"pip install\"],\"evidence\":[\"Name or service not known\"],\"evidence_exec_ids\":[\"rexec-1\"],\"question\":\"是否提供镜像？\"}，请确认。"
	cleaned := stripExecutionBlockerPayload(raw)
	if strings.Contains(cleaned, "\"type\":\"execution_blocker\"") {
		t.Fatalf("expected execution_blocker payload stripped, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, "请确认") {
		t.Fatalf("expected narrative preserved, got: %s", cleaned)
	}
}

func TestParseAutonomyProgressReportNarrativeWrapped(t *testing.T) {
	raw := "我先汇报进度：\n```json\n{\n  \"type\":\"progress_report\",\n  \"goal\":\"修复测试\",\n  \"done\":false,\n  \"remaining_steps\":[\"补齐回归测试\"],\n  \"evidence\":[\"已修复构建错误\"],\n  \"summary\":\"主体修复完成，待验证\",\n  \"next_action\":\"执行 go test ./...\"\n}\n```\n然后继续执行。"
	report, ok := parseAutonomyProgressReport(raw)
	if !ok {
		t.Fatalf("expected parse success for narrative wrapped progress_report")
	}
	if report.Goal != "修复测试" || report.Done || len(report.RemainingSteps) != 1 {
		t.Fatalf("unexpected parsed report: %+v", report)
	}
}

func TestStripProgressReportPayloadInlineJSON(t *testing.T) {
	raw := "阶段汇报：{\"type\":\"progress_report\",\"goal\":\"修复测试\",\"done\":false,\"remaining_steps\":[\"执行回归测试\"]}，我会继续。"
	cleaned := stripProgressReportPayload(raw)
	if strings.Contains(cleaned, "\"type\":\"progress_report\"") {
		t.Fatalf("expected progress_report payload stripped, got: %s", cleaned)
	}
	if !strings.Contains(cleaned, "我会继续") {
		t.Fatalf("expected narrative preserved, got: %s", cleaned)
	}
}

func TestParseAutonomyProgressReportDoneString(t *testing.T) {
	raw := `{"type":"progress_report","goal":"修复测试","done":"true","evidence":["go test ./... 通过"]}`
	report, ok := parseAutonomyProgressReport(raw)
	if !ok {
		t.Fatalf("expected parse success for done string")
	}
	if !report.Done {
		t.Fatalf("expected done parsed as true from string, got: %+v", report)
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
