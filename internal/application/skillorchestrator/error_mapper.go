package skillorchestrator

import (
	"errors"
	"strings"
)

func UserFacingError(err error) string {
	if err == nil {
		return ""
	}
	var confirmationRequired *ConfirmationRequiredError
	if errors.As(err, &confirmationRequired) {
		return "高风险操作需确认：请使用 `--confirm " + strings.TrimSpace(confirmationRequired.ConfirmationID) + "` 重试，或使用 `--reject " + strings.TrimSpace(confirmationRequired.ConfirmationID) + "` 拒绝。"
	}
	var confirmationRejected *ConfirmationRejectedError
	if errors.As(err, &confirmationRejected) {
		return "已拒绝高风险操作：未执行任何副作用。"
	}
	text := strings.TrimSpace(err.Error())
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "not allowed"), strings.Contains(lower, "rejected by policy"):
		return "策略拒绝：当前来源或能力不在允许范围。"
	case strings.Contains(lower, "permission"):
		return "权限不足：当前操作者缺少执行该技能所需权限。"
	case strings.Contains(lower, "disabled"):
		return "技能已禁用，无法执行。"
	case strings.Contains(lower, "required"):
		return "参数不完整，请补充必要字段。"
	case strings.Contains(lower, "not found"):
		return "未找到对应技能或配置项。"
	case strings.Contains(lower, "confidence"):
		return "语义置信度不足，请改用更明确表达或命令。"
	default:
		return "技能操作失败：" + text
	}
}
