package integration

import (
	"strings"
	"testing"
)

func TestChannelRuntimeRetryLogFieldsContract(t *testing.T) {
	source := loadMainSource(t)

	required := "channel adapter stopped: channel=%s instance=%s component=%s retry_count=%d last_error=%q retry_in=%s"
	if !strings.Contains(source, required) {
		t.Fatalf("retry log fields changed or missing required structured fields")
	}
}
