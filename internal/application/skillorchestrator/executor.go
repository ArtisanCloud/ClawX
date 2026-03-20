package skillorchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	skilldomain "clawx/internal/domain/skill"
)

type ExecuteRequest struct {
	Action               skilldomain.SkillAction
	Metadata             skilldomain.SkillMetadata
	ProjectID            string
	AgentID              string
	Actor                string
	ConversationID       string
	ConfirmationID       string
	ConfirmationDecision string
	TraceID              string
}

type ExecuteResult struct {
	Executed bool
	Binding  *skilldomain.SkillBinding
	SkillID  string
	Output   string
	TraceID  string
}

type Executor struct {
	bindingRepo skilldomain.BindingRepository
	resolver    *BindingResolver
	policy      *PolicyEngine
	riskGuard   *RiskGuard
	audit       *AuditService
}

func NewExecutor(bindingRepo skilldomain.BindingRepository, policy *PolicyEngine, riskGuard *RiskGuard, audit *AuditService) *Executor {
	return &Executor{
		bindingRepo: bindingRepo,
		resolver:    NewBindingResolver(),
		policy:      policy,
		riskGuard:   riskGuard,
		audit:       audit,
	}
}

func (e *Executor) Execute(ctx context.Context, request ExecuteRequest) (ExecuteResult, error) {
	traceID := strings.TrimSpace(request.TraceID)
	if traceID == "" {
		if strings.TrimSpace(request.ConfirmationID) != "" {
			traceID = strings.TrimSpace(request.ConfirmationID)
		} else {
			traceID = fmt.Sprintf("trace-%d", time.Now().UTC().UnixNano())
		}
	}
	action := request.Action.Normalize()
	if err := action.Validate(); err != nil {
		e.recordAudit(traceID, request, action, "validation_rejected", err)
		return ExecuteResult{}, err
	}
	metadata := request.Metadata.Normalize()
	if err := metadata.Validate(); err != nil {
		e.recordAudit(traceID, request, action, "metadata_rejected", err)
		return ExecuteResult{}, err
	}
	if e.policy != nil {
		if err := e.policy.Evaluate(ctx, metadata); err != nil {
			e.recordAudit(traceID, request, action, "policy_rejected", err)
			return ExecuteResult{}, err
		}
	}

	var binding *skilldomain.SkillBinding
	if e.bindingRepo != nil && e.resolver != nil {
		bindings, err := e.bindingRepo.List(ctx)
		if err != nil {
			e.recordAudit(traceID, request, action, "binding_list_failed", err)
			return ExecuteResult{}, err
		}
		if resolved, ok := e.resolver.Resolve(bindings, ResolveRequest{
			SkillID:   action.SkillID,
			ProjectID: request.ProjectID,
			AgentID:   request.AgentID,
		}); ok {
			copyBinding := resolved
			binding = &copyBinding
		}
	}
	if binding == nil {
		err := fmt.Errorf("skill %q is not bound for current scope", action.SkillID)
		e.recordAudit(traceID, request, action, "binding_rejected", err)
		return ExecuteResult{}, err
	}
	if action.SkillID != metadata.SkillID {
		err := fmt.Errorf("action skill %q does not match metadata skill %q", action.SkillID, metadata.SkillID)
		e.recordAudit(traceID, request, action, "metadata_rejected", err)
		return ExecuteResult{}, err
	}
	if strings.TrimSpace(binding.Version) != strings.TrimSpace(metadata.Version) {
		err := fmt.Errorf("bound version %q does not match metadata version %q for skill %q", binding.Version, metadata.Version, action.SkillID)
		e.recordAudit(traceID, request, action, "version_rejected", err)
		return ExecuteResult{}, err
	}
	if e.riskGuard != nil {
		signature := buildConfirmationSignature(action, metadata, request.ProjectID, request.AgentID)
		err := e.riskGuard.Authorize(RiskAuthorizeRequest{
			ConversationID: strings.TrimSpace(request.ConversationID),
			Actor:          strings.TrimSpace(request.Actor),
			SkillID:        strings.TrimSpace(action.SkillID),
			Signature:      signature,
			RiskLevel:      action.RiskLevel,
			ConfirmationID: strings.TrimSpace(request.ConfirmationID),
			Decision:       strings.TrimSpace(request.ConfirmationDecision),
		})
		if err != nil {
			result := "confirm_rejected"
			if requiredErr, ok := err.(*ConfirmationRequiredError); ok {
				result = "confirm_required"
				if requiredTrace := strings.TrimSpace(requiredErr.ConfirmationID); requiredTrace != "" {
					traceID = requiredTrace
				}
			}
			e.recordAudit(traceID, request, action, result, err)
			return ExecuteResult{}, err
		}
	}
	if err := validateRequiredPermissions(request.Actor, metadata.RequiredPermissions, action.Arguments); err != nil {
		e.recordAudit(traceID, request, action, "permission_rejected", err)
		return ExecuteResult{}, err
	}
	e.recordAudit(traceID, request, action, "success", nil)
	return ExecuteResult{
		Executed: true,
		Binding:  binding,
		SkillID:  action.SkillID,
		Output:   "skill execution accepted",
		TraceID:  traceID,
	}, nil
}

func buildConfirmationSignature(action skilldomain.SkillAction, metadata skilldomain.SkillMetadata, projectID, agentID string) string {
	return strings.Join([]string{
		strings.TrimSpace(action.SkillID),
		strings.TrimSpace(metadata.Version),
		strings.TrimSpace(projectID),
		strings.TrimSpace(agentID),
		strings.TrimSpace(action.Intent),
	}, "|")
}

func (e *Executor) recordAudit(traceID string, request ExecuteRequest, action skilldomain.SkillAction, result string, err error) {
	if e == nil || e.audit == nil {
		return
	}
	record := AuditRecord{
		TraceID:        strings.TrimSpace(traceID),
		EventType:      skilldomain.AuditEventExec,
		ConversationID: strings.TrimSpace(request.ConversationID),
		Actor:          strings.TrimSpace(request.Actor),
		SkillID:        strings.TrimSpace(action.SkillID),
		Source:         strings.TrimSpace(action.Source),
		Intent:         strings.TrimSpace(action.Intent),
		Confidence:     action.Confidence,
		Arguments:      action.Arguments,
		Result:         strings.TrimSpace(result),
		OccurredAt:     time.Now().UTC(),
	}
	if err != nil {
		record.Error = strings.TrimSpace(err.Error())
	}
	e.audit.Record(record)
}

func validateRequiredPermissions(actor string, required []string, arguments map[string]any) error {
	normalizedRequired := normalizePermissions(required)
	if len(normalizedRequired) == 0 {
		return nil
	}
	actor = strings.TrimSpace(actor)
	for _, item := range normalizedRequired {
		if actor != "" && item == actor {
			return nil
		}
	}
	granted := map[string]struct{}{}
	for _, item := range extractPermissions(arguments) {
		granted[item] = struct{}{}
	}
	for _, item := range normalizedRequired {
		if _, ok := granted[item]; !ok {
			return fmt.Errorf("permission %q is required", item)
		}
	}
	return nil
}

func extractPermissions(arguments map[string]any) []string {
	if len(arguments) == 0 {
		return nil
	}
	raw, ok := arguments["permissions"]
	if !ok {
		return nil
	}
	switch values := raw.(type) {
	case []string:
		return normalizePermissions(values)
	case []any:
		out := make([]string, 0, len(values))
		for _, item := range values {
			if text, ok := item.(string); ok {
				out = append(out, text)
			}
		}
		return normalizePermissions(out)
	case string:
		if strings.TrimSpace(values) == "" {
			return nil
		}
		return normalizePermissions(strings.Split(values, ","))
	default:
		return nil
	}
}

func normalizePermissions(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
