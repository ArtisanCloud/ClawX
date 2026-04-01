package main

import (
	"context"
	"errors"
	"strings"
	"testing"

	"clawx/internal/application/autonomy"
	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestTryRecoverSessionFlowSuccessAfterRetry(t *testing.T) {
	classification := autonomy.FailureClassification{
		Class:       autonomy.FailureClassTool,
		Recoverable: true,
		Reason:      "tool_or_command_failed",
	}
	runtime := agentRuntime{agentID: "main"}
	decision := service.Decision{
		ConversationID: "conv-recover-success",
		Message:        chatiface.Message{Text: "继续执行"},
	}
	attempts := 0
	recovered, result, ok := tryRecoverSessionFlow(
		context.Background(),
		"discord",
		"default",
		runtime,
		decision,
		classification,
		func(_ context.Context, attempt int) (service.SessionFlowResult, error) {
			attempts = attempt
			if attempt == 1 {
				return service.SessionFlowResult{}, errors.New("exit status 1")
			}
			return service.SessionFlowResult{}, nil
		},
	)
	if !ok || !result.Recovered {
		t.Fatalf("expected recovery success, got ok=%v recovered=%v", ok, result.Recovered)
	}
	if result.Attempts != 2 || attempts != 2 {
		t.Fatalf("expected 2 attempts, got result=%d attempts=%d", result.Attempts, attempts)
	}
	_ = recovered
}

func TestTryRecoverSessionFlowFailureConverges(t *testing.T) {
	classification := autonomy.FailureClassification{
		Class:       autonomy.FailureClassTool,
		Recoverable: true,
		Reason:      "tool_or_command_failed",
	}
	runtime := agentRuntime{agentID: "main"}
	decision := service.Decision{
		ConversationID: "conv-recover-failed",
		Message:        chatiface.Message{Text: "继续执行"},
	}
	_, result, ok := tryRecoverSessionFlow(
		context.Background(),
		"discord",
		"default",
		runtime,
		decision,
		classification,
		func(_ context.Context, _ int) (service.SessionFlowResult, error) {
			return service.SessionFlowResult{}, errors.New("exit status 2")
		},
	)
	if ok || result.Recovered {
		t.Fatalf("expected recovery failure convergence, got ok=%v recovered=%v", ok, result.Recovered)
	}
	if result.Attempts != 2 {
		t.Fatalf("expected max retries exhausted at 2 attempts for tool class, got %d", result.Attempts)
	}
	if strings.TrimSpace(result.StopReason) != "max_retries_exhausted" {
		t.Fatalf("unexpected stop reason: %q", result.StopReason)
	}
}

func TestHandleExecutionFailureEscalationPrompt(t *testing.T) {
	runtime := agentRuntime{agentID: "main"}
	decision := service.Decision{
		ConversationID: "conv-escalate",
		Message:        chatiface.Message{Text: "继续执行"},
	}
	response := handleExecutionFailure("discord", "default", runtime, decision, errors.New("permission denied"))
	if !strings.Contains(response, "处理失败") {
		t.Fatalf("expected failed receipt, got: %s", response)
	}
	if !strings.Contains(response, "已尝试:") {
		t.Fatalf("expected escalation evidence section, got: %s", response)
	}
	if !strings.Contains(response, "推荐操作:") {
		t.Fatalf("expected recommendation section, got: %s", response)
	}
}
