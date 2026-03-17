package main

import (
	"strings"
	"testing"

	"clawx/internal/application/service"
	chatiface "clawx/internal/interfaces/chat"
)

func TestBuildExecutionInputIncludesAttachmentsForDirectExecution(t *testing.T) {
	decision := service.Decision{
		Kind: service.DecisionExecute,
		Message: chatiface.Message{
			Text: "转换成长图",
			Attachments: []chatiface.Attachment{
				{
					Name:        "test.pdf",
					URL:         "https://cdn.discordapp.com/test.pdf",
					LocalPath:   "/home/ubuntu/.clawx/workspaces/image_tools/.agents/main/context/attachments/files/abc/test.pdf",
					ContentType: "application/pdf",
					SizeBytes:   9195520,
				},
			},
		},
	}

	input := buildExecutionInput(decision)
	if !strings.Contains(input, "[Attachments]") {
		t.Fatalf("expected attachments section, got: %q", input)
	}
	if !strings.Contains(input, "test.pdf") {
		t.Fatalf("expected attachment name in input, got: %q", input)
	}
	if !strings.Contains(input, "application/pdf") {
		t.Fatalf("expected content type in input, got: %q", input)
	}
	if !strings.Contains(input, "local_path=") {
		t.Fatalf("expected local path in input, got: %q", input)
	}
}
