package contract

import (
	"reflect"
	"testing"

	"clawx/internal/application/command"
)

func TestServiceControlCommandParseContract(t *testing.T) {
	t.Run("start", func(t *testing.T) {
		cmd, err := command.ParseServiceControlCommand("/service start img -- sleep 30")
		if err != nil {
			t.Fatalf("parse start: %v", err)
		}
		if cmd.Kind != command.ServiceControlStart {
			t.Fatalf("unexpected kind: %s", cmd.Kind)
		}
		if cmd.Name != "img" {
			t.Fatalf("unexpected name: %q", cmd.Name)
		}
		want := []string{"sleep", "30"}
		if !reflect.DeepEqual(cmd.Command, want) {
			t.Fatalf("unexpected command: got=%v want=%v", cmd.Command, want)
		}
	})

	t.Run("status", func(t *testing.T) {
		cmd, err := command.ParseServiceControlCommand("/service status worker")
		if err != nil {
			t.Fatalf("parse status: %v", err)
		}
		if cmd.Kind != command.ServiceControlStatus || cmd.Name != "worker" {
			t.Fatalf("unexpected status cmd: %#v", cmd)
		}
	})

	t.Run("logs", func(t *testing.T) {
		cmd, err := command.ParseServiceControlCommand("/service logs worker --tail=20")
		if err != nil {
			t.Fatalf("parse logs: %v", err)
		}
		if cmd.Kind != command.ServiceControlLogs || cmd.Name != "worker" || cmd.Tail != 20 {
			t.Fatalf("unexpected logs cmd: %#v", cmd)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := command.ParseServiceControlCommand("/service start worker sleep 10")
		if err == nil {
			t.Fatalf("expected invalid command error")
		}
	})
}
