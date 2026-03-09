package chat

import (
	"errors"
	"fmt"
	"strings"

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
	case errors.Is(err, ErrInvalidNormalizeInput), errors.Is(err, ErrInvalidWindowContext):
		return "请求上下文无效，请检查窗口信息后重试"
	case errors.Is(err, session.ErrSessionBusy):
		return "当前会话正在执行中，请稍后重试或先取消当前执行"
	case errors.Is(err, session.ErrSessionNotFound):
		return "未找到可用会话，请先创建新会话"
	case errors.Is(err, session.ErrWindowBindingNotFound):
		return "当前窗口尚未绑定会话，请先发送 /new 或 /resume"
	case errors.Is(err, session.ErrInvalidLock):
		return "会话锁状态异常，请稍后重试"
	case strings.Contains(err.Error(), "session does not belong to the active conversation"):
		return "目标会话不属于当前窗口上下文，请确认后重试"
	case errors.Is(err, command.ErrInvalidControlCommand):
		return "控制命令格式无效，请检查命令参数"
	case errors.Is(err, command.ErrInvalidSessionCommand):
		return "会话命令参数无效，请检查输入"
	case errorCategory(err) == "permission_denied":
		return "当前用户或频道没有该 Skill 权限，请联系管理员授权"
	case errorCategory(err) == "skill_not_found":
		return "未找到对应 Skill，请检查 Skill 名称或先执行 skill list"
	case errorCategory(err) == "skill_disabled":
		return "该 Skill 当前已被禁用，请联系管理员启用"
	case errorCategory(err) == "skill_invalid":
		return "该 Skill 配置无效，暂不可用"
	case errorCategory(err) == "pairing_expired":
		return "DM 配对已失效，请重新完成配对后再试"
	default:
		return fmt.Sprintf("执行失败: %v", err)
	}
}

type categorizedError interface {
	Code() string
}

func errorCategory(err error) string {
	if err == nil {
		return ""
	}
	var coded categorizedError
	if errors.As(err, &coded) {
		return coded.Code()
	}
	return ""
}
