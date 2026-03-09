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
	SwitchedSessionID  string
	CancelledSessionID string
	CancelNoop         bool
	Sessions           []ControlSessionSummary
	CurrentSessionID   string
	CurrentStatus      string
	CurrentChecked     bool
}

func FormatControlResponse(result ControlResponse) string {
	switch {
	case result.CreatedSessionID != "":
		return fmt.Sprintf("已创建新会话: %s", result.CreatedSessionID)
	case result.ResumedSessionID != "":
		return fmt.Sprintf("已恢复会话: %s", result.ResumedSessionID)
	case result.SwitchedSessionID != "":
		return fmt.Sprintf("已切换当前会话: %s", result.SwitchedSessionID)
	case result.CancelledSessionID != "":
		if result.CancelNoop {
			return fmt.Sprintf("当前会话未在执行，无需取消: %s", result.CancelledSessionID)
		}
		return fmt.Sprintf("已取消会话执行: %s", result.CancelledSessionID)
	case len(result.Sessions) > 0:
		lines := make([]string, 0, len(result.Sessions)+1)
		lines = append(lines, "可用会话:")
		for _, summary := range result.Sessions {
			line := fmt.Sprintf("- %s [%s]", summary.ID, summary.Status)
			if summary.ID == result.CurrentSessionID && strings.TrimSpace(result.CurrentSessionID) != "" {
				line += " (current)"
			}
			lines = append(lines, line)
		}
		return strings.Join(lines, "\n")
	case result.CurrentChecked:
		if strings.TrimSpace(result.CurrentSessionID) == "" {
			return "当前没有会话，请先发送 /new"
		}
		return fmt.Sprintf("当前会话: %s [%s]", result.CurrentSessionID, result.CurrentStatus)
	default:
		return "当前没有可返回的控制结果"
	}
}
