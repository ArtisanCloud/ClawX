package chat

import (
	"errors"
	"fmt"

	"synapsex/internal/application/command"
	"synapsex/internal/domain/session"
)

func FormatError(err error) string {
	switch {
	case err == nil:
		return ""
	case err.Error() == "channel context is not allowed":
		return "当前上下文未被授权，无法执行请求"
	case err.Error() == "message text is empty":
		return "请求内容为空，请提供有效输入"
	case errors.Is(err, session.ErrSessionBusy):
		return "当前会话正在执行中，请稍后重试或先取消当前执行"
	case errors.Is(err, session.ErrSessionNotFound):
		return "未找到可用会话，请先创建新会话"
	case errors.Is(err, session.ErrInvalidLock):
		return "会话锁状态异常，请稍后重试"
	case errors.Is(err, command.ErrInvalidControlCommand):
		return "控制命令格式无效，请检查命令参数"
	case errors.Is(err, command.ErrInvalidSessionCommand):
		return "会话命令参数无效，请检查输入"
	default:
		return fmt.Sprintf("执行失败: %v", err)
	}
}
