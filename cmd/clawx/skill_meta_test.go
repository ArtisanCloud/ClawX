package main

import (
	"strings"
	"testing"

	"clawx/internal/application/service"
)

func TestHandleClawXSkillMetaCommand(t *testing.T) {
	runtime := agentRuntime{}

	handled, response := handleClawXSkillMetaCommand(runtime, "/clawx-skills")
	if !handled {
		t.Fatalf("expected clawx-skills to be handled")
	}
	if response == "" {
		t.Fatalf("expected clawx-skills response")
	}

	handled, _ = handleClawXSkillMetaCommand(runtime, "/sx-skills")
	if handled {
		t.Fatalf("did not expect sx-skills to be handled")
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
	if executeText != "result" {
		t.Fatalf("expected execute output unchanged, got %q", executeText)
	}
}

func TestApplyRuntimeAgentLabel(t *testing.T) {
	mainText := applyRuntimeAgentLabel("result", "main")
	if mainText != "result" {
		t.Fatalf("expected main agent no label, got %q", mainText)
	}
	bidText := applyRuntimeAgentLabel("result", "bid-all")
	if !strings.HasPrefix(bidText, "[bid-all-agent]:") {
		t.Fatalf("expected bid-all label, got %q", bidText)
	}
}

func TestApplyExecutionCompletionGateBlocksClaimWithoutEvidence(t *testing.T) {
	decision := service.Decision{
		Kind:    service.DecisionExecute,
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
