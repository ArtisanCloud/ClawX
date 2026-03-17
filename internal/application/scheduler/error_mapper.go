package scheduler

func mapNextAction(code string) string {
	switch code {
	case "invalid_schedule_command":
		return "请检查命令参数，示例：/schedule add image-cleanup --cron \"0 3 * * 0\" --task image.cleanup"
	case "schedule_job_not_found":
		return "请先执行 /schedule list 确认任务名称或 ID"
	case "schedule_job_conflict":
		return "请更换任务名，或先 /schedule remove <name>"
	case "schedule_job_running":
		return "请等待当前执行结束后重试 /schedule run"
	case "rejected_scope":
		return "请确认当前项目和 agent 是否正确，可先执行 /project current"
	case "schedule_run_failed":
		return "请查看 /schedule status <name> 与日志后修正任务参数"
	default:
		return "请执行 /schedule list 或 /schedule status <name> 进一步排查"
	}
}
