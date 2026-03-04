package chat

import (
	"fmt"
	"strings"
)

type ControlSessionSummary struct {
	ID     string
	Status string
}

type ControlResponse struct {
	CreatedSessionID   string
	ResumedSessionID   string
	CancelledSessionID string
	Sessions           []ControlSessionSummary
}

func FormatControlResponse(result ControlResponse) string {
	switch {
	case result.CreatedSessionID != "":
		return fmt.Sprintf("已创建新会话: %s", result.CreatedSessionID)
	case result.ResumedSessionID != "":
		return fmt.Sprintf("已恢复会话: %s", result.ResumedSessionID)
	case result.CancelledSessionID != "":
		return fmt.Sprintf("已取消会话执行: %s", result.CancelledSessionID)
	case len(result.Sessions) > 0:
		lines := make([]string, 0, len(result.Sessions)+1)
		lines = append(lines, "可用会话:")
		for _, summary := range result.Sessions {
			lines = append(lines, fmt.Sprintf("- %s [%s]", summary.ID, summary.Status))
		}
		return strings.Join(lines, "\n")
	default:
		return "当前没有可返回的控制结果"
	}
}
