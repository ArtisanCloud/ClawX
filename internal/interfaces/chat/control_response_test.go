package chat

import "testing"

func TestFormatControlResponseCancelNoop(t *testing.T) {
	got := FormatControlResponse(ControlResponse{
		CancelledSessionID: "sess-1",
		CancelNoop:         true,
	})
	want := "当前会话未在执行，无需取消: sess-1"
	if got != want {
		t.Fatalf("unexpected cancel noop message: got=%q want=%q", got, want)
	}
}
