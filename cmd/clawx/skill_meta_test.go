package main

import (
	"strings"
	"testing"

	"clawx/internal/application/service"
)

func TestHandleClawXSkillMetaCommand(t *testing.T) {
	runtime := agentRuntime{}

	handled, response := handleClawXSkillMetaCommand(runtime, "/sx-skills")
	if !handled {
		t.Fatalf("expected sx-skills to be handled")
	}
	if response == "" {
		t.Fatalf("expected sx-skills response")
	}

	handled, _ = handleClawXSkillMetaCommand(runtime, "hello")
	if handled {
		t.Fatalf("unexpected handled for non sx command")
	}
}

func TestApplyExecutionSourceLabel(t *testing.T) {
	skillText := applyExecutionSourceLabel(service.Decision{Kind: service.DecisionSkill}, "result")
	if !strings.HasPrefix(skillText, "[ClawX Skill]") {
		t.Fatalf("expected skill prefix, got %q", skillText)
	}

	executeText := applyExecutionSourceLabel(service.Decision{Kind: service.DecisionExecute}, "result")
	if !strings.HasPrefix(executeText, "[Agent Direct]") {
		t.Fatalf("expected execute prefix, got %q", executeText)
	}
}

func TestApplyExecutionCompletionGateBlocksClaimWithoutEvidence(t *testing.T) {
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: service.Decision{}.Message,
	}
	decision.Message.Text = "请实现图片工具的清理定时脚本，并支持执行后自动通知我报告"

	output := "已实现，包含定时清理和自动通知。"
	gated := applyExecutionCompletionGate(decision, output)
	if !strings.Contains(gated, "未通过平台验收门禁") {
		t.Fatalf("expected gate message, got %q", gated)
	}
}

func TestApplyExecutionCompletionGateAllowsClaimWithEvidence(t *testing.T) {
	decision := service.Decision{}
	decision.Kind = service.DecisionExecute
	decision.Message.Text = "请实现图片工具的清理定时脚本，并支持执行后自动通知我报告"

	output := "已实现。\n命令:\ngo test ./... -count=1\n结果：ok\n文件：[cleanup.go](/tmp/cleanup.go)"
	gated := applyExecutionCompletionGate(decision, output)
	if gated != output {
		t.Fatalf("expected output pass gate, got %q", gated)
	}
}
