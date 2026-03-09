package intent

import (
	"context"
	"strings"
	"time"

	skilldomain "synapsex/internal/domain/skill"
	chatiface "synapsex/internal/interfaces/chat"
)

type SkillRegistry interface {
	Snapshot() skilldomain.RegistrySnapshot
	FindEntry(name string) (skilldomain.CatalogEntry, bool)
	FindActiveDefinition(name string) (skilldomain.Definition, error)
}

type Pipeline struct {
	registry          SkillRegistry
	permissionChecker *PermissionChecker
	fallback          LLMFallback
	threshold         float64
}

func NewPipeline(registry SkillRegistry, permissionChecker *PermissionChecker, fallback LLMFallback, threshold float64) *Pipeline {
	if fallback == nil {
		fallback = DefaultLLMFallback{}
	}
	if threshold <= 0 || threshold > 1 {
		threshold = 0.72
	}
	return &Pipeline{
		registry:          registry,
		permissionChecker: permissionChecker,
		fallback:          fallback,
		threshold:         threshold,
	}
}

type Result struct {
	Decision   skilldomain.IntentDecision
	Skill      *skilldomain.Definition
	SkillInput string
	DurationMS int64
}

func (p *Pipeline) Decide(ctx context.Context, message chatiface.Message) (Result, error) {
	started := time.Now()
	text := strings.TrimSpace(message.Text)
	if text == "" {
		decision, _ := (skilldomain.IntentDecision{Kind: skilldomain.IntentTask, Reason: "empty_message"}).Normalize()
		return Result{Decision: decision, DurationMS: 0}, nil
	}
	if isControlCommand(text) {
		decision, _ := (skilldomain.IntentDecision{Kind: skilldomain.IntentControl, Reason: "control_command"}).Normalize()
		return Result{Decision: decision, DurationMS: time.Since(started).Milliseconds()}, nil
	}

	snapshot := p.registry.Snapshot()

	explicit := ParseExplicitSkill(text)
	if explicit.Valid {
		result, err := p.resolveSkill(ctx, snapshot, message, explicit.Name, "explicit_skill", 1.0, explicit.Input)
		result.DurationMS = time.Since(started).Milliseconds()
		return result, err
	}

	ruleCandidates := MatchByRule(text, snapshot)
	if selected, ok := SelectCandidate(ruleCandidates); ok {
		result, err := p.resolveSkill(ctx, snapshot, message, selected.SkillName, selected.Reason, selected.Confidence, text)
		result.DurationMS = time.Since(started).Milliseconds()
		return result, err
	}

	llmCandidate, matched, err := p.fallback.Match(ctx, text, snapshot)
	if err != nil {
		return Result{}, err
	}
	if matched {
		if llmCandidate.Confidence >= p.threshold {
			result, err := p.resolveSkill(ctx, snapshot, message, llmCandidate.SkillName, llmCandidate.Reason, llmCandidate.Confidence, text)
			result.DurationMS = time.Since(started).Milliseconds()
			return result, err
		}
		decision, _ := (skilldomain.IntentDecision{
			Kind:       skilldomain.IntentTask,
			Reason:     "llm_below_threshold",
			Confidence: llmCandidate.Confidence,
		}).Normalize()
		return Result{Decision: decision, DurationMS: time.Since(started).Milliseconds()}, nil
	}

	decision, _ := (skilldomain.IntentDecision{Kind: skilldomain.IntentTask, Reason: "task_fallback"}).Normalize()
	return Result{Decision: decision, DurationMS: time.Since(started).Milliseconds()}, nil
}

func (p *Pipeline) resolveSkill(
	ctx context.Context,
	snapshot skilldomain.RegistrySnapshot,
	message chatiface.Message,
	skillName string,
	reason string,
	confidence float64,
	input string,
) (Result, error) {
	entry, ok := snapshot.FindEntryByName(skillName)
	if !ok {
		return Result{}, RoutedError{
			Category: ErrorSkillNotFound,
			Message:  "requested skill does not exist",
		}
	}
	switch entry.Status {
	case skilldomain.StatusDisabled:
		return Result{}, RoutedError{
			Category: ErrorSkillDisabled,
			Message:  "requested skill is disabled",
		}
	case skilldomain.StatusInvalid:
		return Result{}, RoutedError{
			Category: ErrorSkillInvalid,
			Message:  "requested skill is invalid",
		}
	}

	def, err := p.registry.FindActiveDefinition(skillName)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), "not found"):
			return Result{}, RoutedError{Category: ErrorSkillNotFound, Message: "requested skill does not exist"}
		case strings.Contains(err.Error(), "disabled"):
			return Result{}, RoutedError{Category: ErrorSkillDisabled, Message: "requested skill is disabled"}
		default:
			return Result{}, RoutedError{Category: ErrorSkillInvalid, Message: "requested skill is not available"}
		}
	}

	if p.permissionChecker != nil {
		if err := p.permissionChecker.Allow(ctx, message, def.Name); err != nil {
			return Result{}, err
		}
	}

	decision, _ := (skilldomain.IntentDecision{
		Kind:       skilldomain.IntentSkill,
		SkillName:  def.Name,
		Reason:     reason,
		Confidence: confidence,
	}).Normalize()
	return Result{
		Decision:   decision,
		Skill:      &def,
		SkillInput: strings.TrimSpace(input),
	}, nil
}

func isControlCommand(text string) bool {
	fields := strings.Fields(strings.TrimSpace(text))
	if len(fields) == 0 {
		return false
	}
	switch normalizeCommand(fields[0]) {
	case "new", "resume", "switch", "list", "cancel", "current":
		return true
	default:
		return false
	}
}
