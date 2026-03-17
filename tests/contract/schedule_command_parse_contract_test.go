package contract

import (
	"testing"

	"clawx/internal/application/command"
)

func TestScheduleCommandParseContract(t *testing.T) {
	cmd, err := command.ParseScheduleControlCommand(`/schedule add image-cleanup --cron "0 3 * * 0" --task image.cleanup --arg retention_days=30`)
	if err != nil {
		t.Fatalf("parse add: %v", err)
	}
	if cmd.Kind != command.ScheduleControlAdd || cmd.NameOrID != "image-cleanup" {
		t.Fatalf("unexpected add command: %+v", cmd)
	}
	if cmd.CronExpr != "0 3 * * 0" || cmd.TaskType != "image.cleanup" {
		t.Fatalf("unexpected add payload: %+v", cmd)
	}
	if cmd.TaskArgs["retention_days"] != "30" {
		t.Fatalf("unexpected task args: %+v", cmd.TaskArgs)
	}
	if _, err := command.ParseScheduleControlCommand(`/schedule add image-cleanup --task image.cleanup`); err == nil {
		t.Fatalf("expected parse error for missing cron")
	}
}
