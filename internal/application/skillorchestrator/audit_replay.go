package skillorchestrator

import (
	"fmt"
	"strings"
)

type AuditReplayService struct {
	audit *AuditService
}

func NewAuditReplayService(audit *AuditService) *AuditReplayService {
	return &AuditReplayService{audit: audit}
}

func (s *AuditReplayService) ReplayTrace(traceID string) ([]AuditRecord, error) {
	if s == nil || s.audit == nil {
		return nil, fmt.Errorf("audit replay service is not configured")
	}
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("trace id is required")
	}
	records := s.audit.ListByTraceID(traceID)
	if len(records) == 0 {
		return nil, fmt.Errorf("trace %q not found", traceID)
	}
	return records, nil
}
