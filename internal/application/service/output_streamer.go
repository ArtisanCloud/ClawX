package service

import (
	"strings"

	"synapsex/internal/domain/execution"
)

type OutputStreamer struct{}

func NewOutputStreamer() *OutputStreamer {
	return &OutputStreamer{}
}

func (s *OutputStreamer) Segment(sessionID, content string, maxLen int) []execution.OutputSegment {
	if maxLen <= 0 {
		maxLen = len(content)
	}
	if maxLen <= 0 {
		maxLen = 1
	}

	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}

	runes := []rune(content)
	segments := make([]execution.OutputSegment, 0, len(runes)/maxLen+1)
	sequence := 1

	for start := 0; start < len(runes); start += maxLen {
		end := start + maxLen
		if end > len(runes) {
			end = len(runes)
		}

		segment := execution.OutputSegment{
			SessionID:     sessionID,
			Sequence:      sequence,
			Content:       string(runes[start:end]),
			IsFinal:       end == len(runes),
			DeliveryState: execution.DeliveryPending,
		}
		segments = append(segments, segment)
		sequence++
	}

	return segments
}

