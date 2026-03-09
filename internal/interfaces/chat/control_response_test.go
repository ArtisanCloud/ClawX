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

func TestFormatControlResponseSwitch(t *testing.T) {
	got := FormatControlResponse(ControlResponse{
		SwitchedSessionID: "sess-2",
	})
	want := "已切换当前会话: sess-2"
	if got != want {
		t.Fatalf("unexpected switch message: got=%q want=%q", got, want)
	}
}
