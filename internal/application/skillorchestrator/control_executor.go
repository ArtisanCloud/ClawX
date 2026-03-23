package skillorchestrator

import (
	"fmt"
	"strings"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type ControlApplyFunc func(plan skilldomain.ControlPlan) (status, message string, err error)
type ControlVerifyFunc func(plan skilldomain.ControlPlan) (ok bool, detail string, err error)

type ControlExecuteRequest struct {
	TraceID        string
	ConversationID string
	Actor          string
	Plan           skilldomain.ControlPlan
	Apply          ControlApplyFunc
	Verify         ControlVerifyFunc
}

type ControlExecuteResult struct {
	TraceID      string
	Applied      bool
	Status       string
	Message      string
	VerifyOK     bool
	VerifyDetail string
}

type ControlExecutor struct {
	audit *AuditService
}

func NewControlExecutor(audit *AuditService) *ControlExecutor {
	return &ControlExecutor{audit: audit}
}

func (e *ControlExecutor) Execute(request ControlExecuteRequest) (ControlExecuteResult, error) {
	plan := request.Plan.Normalize()
	traceID := strings.TrimSpace(request.TraceID)
	if traceID == "" {
		traceID = fmt.Sprintf("control-%d", time.Now().UTC().UnixNano())
	}
	e.recordControlAudit("plan", traceID, request, plan, "received", nil, nil)

	if err := plan.Validate(); err != nil {
		e.recordControlAudit("validate", traceID, request, plan, "rejected", nil, err)
		return ControlExecuteResult{TraceID: traceID}, err
	}
	if plan.Type != "control_plan" || plan.Intent == "" {
		err := fmt.Errorf("control plan is invalid")
		e.recordControlAudit("validate", traceID, request, plan, "rejected", nil, err)
		return ControlExecuteResult{TraceID: traceID}, err
	}
	if strings.ToLower(strings.TrimSpace(plan.Mode)) != "execute" {
		err := fmt.Errorf("control plan mode %q is not executable", plan.Mode)
		e.recordControlAudit("validate", traceID, request, plan, "rejected", nil, err)
		return ControlExecuteResult{TraceID: traceID}, err
	}
	if request.Apply == nil {
		err := fmt.Errorf("control apply callback is required")
		e.recordControlAudit("execute", traceID, request, plan, "failed", nil, err)
		return ControlExecuteResult{TraceID: traceID}, err
	}
	e.recordControlAudit("validate", traceID, request, plan, "success", nil, nil)

	status, message, applyErr := request.Apply(plan)
	if applyErr != nil {
		e.recordControlAudit("execute", traceID, request, plan, "failed", map[string]any{
			"status":  strings.TrimSpace(status),
			"message": strings.TrimSpace(message),
		}, applyErr)
		return ControlExecuteResult{TraceID: traceID, Status: strings.TrimSpace(status), Message: strings.TrimSpace(message)}, applyErr
	}
	if strings.TrimSpace(status) == "" {
		status = "applied"
	}
	execResult := ControlExecuteResult{
		TraceID:  traceID,
		Applied:  status == "applied",
		Status:   strings.TrimSpace(status),
		Message:  strings.TrimSpace(message),
		VerifyOK: false,
	}
	e.recordControlAudit("execute", traceID, request, plan, "success", map[string]any{
		"status":  execResult.Status,
		"message": execResult.Message,
	}, nil)

	if request.Verify != nil {
		ok, detail, verifyErr := request.Verify(plan)
		execResult.VerifyOK = ok
		execResult.VerifyDetail = strings.TrimSpace(detail)
		if verifyErr != nil {
			e.recordControlAudit("verify", traceID, request, plan, "failed", map[string]any{
				"status":      execResult.Status,
				"verify_ok":   ok,
				"verify_info": execResult.VerifyDetail,
			}, verifyErr)
			return execResult, verifyErr
		}
		result := "success"
		if !ok {
			result = "rejected"
		}
		e.recordControlAudit("verify", traceID, request, plan, result, map[string]any{
			"status":      execResult.Status,
			"verify_ok":   ok,
			"verify_info": execResult.VerifyDetail,
		}, nil)
	}
	return execResult, nil
}

func (e *ControlExecutor) recordControlAudit(
	phase string,
	traceID string,
	request ControlExecuteRequest,
	plan skilldomain.ControlPlan,
	result string,
	arguments map[string]any,
	err error,
) {
	if e == nil || e.audit == nil {
		return
	}
	record := AuditRecord{
		TraceID:        strings.TrimSpace(traceID),
		EventType:      skilldomain.AuditEventExec,
		Phase:          strings.TrimSpace(phase),
		ConversationID: strings.TrimSpace(request.ConversationID),
		Actor:          strings.TrimSpace(request.Actor),
		SkillID:        "control",
		Source:         "control_plan",
		Intent:         strings.TrimSpace(plan.Intent),
		PlanType:       strings.TrimSpace(plan.Type),
		PlanMode:       strings.TrimSpace(plan.Mode),
		Target:         plan.Target,
		Arguments:      arguments,
		Result:         strings.TrimSpace(result),
		OccurredAt:     time.Now().UTC(),
	}
	if err != nil {
		record.Error = strings.TrimSpace(err.Error())
	}
	e.audit.Record(record)
}
