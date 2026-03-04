package service

import (
	"context"

	"synapsex/internal/domain/execution"
	"synapsex/internal/interfaces/chat"
)

type DeliveryReport struct {
	Attempts       int
	Delivered      int
	FailedSegments int
	State          execution.ResultState
}

type OutputDelivery struct {
	formatter *OutputFormatter
	streamer  *OutputStreamer
}

func NewOutputDelivery(formatter *OutputFormatter, streamer *OutputStreamer) *OutputDelivery {
	if formatter == nil {
		formatter = NewOutputFormatter()
	}
	if streamer == nil {
		streamer = NewOutputStreamer()
	}
	return &OutputDelivery{
		formatter: formatter,
		streamer:  streamer,
	}
}

func (d *OutputDelivery) Deliver(ctx context.Context, sender chat.Sender, sessionID, content string, maxLen, maxRetries int) DeliveryReport {
	formatted := d.formatter.Format(content)
	segments := d.streamer.Segment(sessionID, formatted, maxLen)

	report := DeliveryReport{
		State: execution.ResultSuccess,
	}

	for _, segment := range segments {
		delivered := false
		for attempt := 1; attempt <= maxRetries+1; attempt++ {
			report.Attempts++
			if err := sender.SendText(ctx, sessionID, segment.Content, segment.IsFinal); err == nil {
				report.Delivered++
				delivered = true
				break
			}
		}
		if !delivered {
			report.FailedSegments++
		}
	}

	if report.FailedSegments > 0 {
		report.State = execution.ResultPartialDelivery
		_ = sender.SendError(ctx, sessionID, "部分输出发送失败，结果可能不完整")
	}

	return report
}

