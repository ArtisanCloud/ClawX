package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChannelAuditLogFieldsContract(t *testing.T) {
	source := loadMainSource(t)

	channels := []string{"telegram", "feishu", "wecom", "discord"}
	for _, channel := range channels {
		beginMarker := channel + " execute begin: channel=" + channel + " instance=%s event_id=%s"
		if !strings.Contains(source, beginMarker) {
			t.Fatalf("missing begin audit marker for %s", channel)
		}

		failedMarker := channel + " execute failed: channel=" + channel + " instance=%s event_id=%s"
		if !strings.Contains(source, failedMarker) {
			t.Fatalf("missing failed audit marker for %s", channel)
		}
		if !strings.Contains(source, failedMarker+" agent=%s backend=%s") {
			t.Fatalf("failed audit marker for %s lost structured fields", channel)
		}

		doneMarker := channel + " execute done: channel=" + channel + " instance=%s event_id=%s"
		if !strings.Contains(source, doneMarker) {
			t.Fatalf("missing done audit marker for %s", channel)
		}
	}

	if !strings.Contains(source, "intent.kind=%s") {
		t.Fatalf("audit logs missing intent.kind field")
	}
	if !strings.Contains(source, "duration_ms=%d") {
		t.Fatalf("audit logs missing duration_ms field")
	}
}

func loadMainSource(t *testing.T) string {
	t.Helper()
	path := filepath.Join("..", "..", "cmd", "clawx", "main.go")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	return string(body)
}
