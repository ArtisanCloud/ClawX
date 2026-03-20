package skillorchestrator

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

const (
	defaultConfidenceThreshold = 0.70
	confidenceThresholdEnvKey  = "CLAWX_SKILL_ROUTER_CONFIDENCE_THRESHOLD"
)

var skillIDPattern = regexp.MustCompile(`([A-Za-z0-9._-]+)`)

type RouteDecision struct {
	Matched       bool
	LowConfidence bool
	Action        skilldomain.SkillAction
	Reason        string
}

type RouteInput struct {
	Message          string
	ContextDigest    skilldomain.ContextDigest
	SkillCatalogHash string
}

type Router struct {
	threshold float64
}

func NewRouterFromEnv() *Router {
	threshold := defaultConfidenceThreshold
	raw := strings.TrimSpace(os.Getenv(confidenceThresholdEnvKey))
	if raw != "" {
		if parsed, err := strconv.ParseFloat(raw, 64); err == nil {
			if parsed >= 0 && parsed <= 1 {
				threshold = parsed
			}
		}
	}
	return &Router{threshold: threshold}
}

func (r *Router) Threshold() float64 {
	if r == nil || r.threshold <= 0 {
		return defaultConfidenceThreshold
	}
	return r.threshold
}

func (r *Router) RouteText(raw string) (RouteDecision, error) {
	return r.Route(RouteInput{Message: raw})
}

func (r *Router) Route(input RouteInput) (RouteDecision, error) {
	text := strings.TrimSpace(input.Message)
	if text == "" {
		return RouteDecision{}, nil
	}
	if strings.HasPrefix(text, "/") {
		return RouteDecision{}, nil
	}
	lower := strings.ToLower(text)
	hasSkillWord := strings.Contains(lower, "skill") || strings.Contains(text, "技能")
	if !hasSkillWord {
		return RouteDecision{}, nil
	}

	intent := ""
	switch {
	case strings.Contains(lower, "install") || strings.Contains(text, "安装"):
		intent = "install_skill"
	case strings.Contains(lower, "bind") || strings.Contains(text, "绑定"):
		intent = "bind_skill"
	case strings.Contains(lower, "disable") || strings.Contains(text, "禁用"):
		intent = "disable_skill"
	}
	if intent == "" {
		return RouteDecision{
			Matched:       true,
			LowConfidence: true,
			Reason:        "skill_intent_unclear",
			Action: skilldomain.SkillAction{
				Intent:               "clarify_skill_intent",
				SkillID:              "",
				Confidence:           0.30,
				RiskLevel:            skilldomain.RiskLow,
				RequiresConfirmation: false,
				Source:               "nl",
			}.Normalize(),
		}, nil
	}

	skillID := extractSkillID(text)
	confidence := 0.62
	if skillID != "" {
		confidence = 0.92
	}
	action := skilldomain.SkillAction{
		Intent:  intent,
		SkillID: skillID,
		Arguments: map[string]any{
			"raw_text":             text,
			"context_digest":       input.ContextDigest.Hash,
			"context_digest_size":  len(input.ContextDigest.Entries),
			"skill_catalog_digest": strings.TrimSpace(input.SkillCatalogHash),
		},
		Confidence:           confidence,
		RiskLevel:            skilldomain.RiskLow,
		RequiresConfirmation: false,
		Source:               "nl",
	}.Normalize()
	if skillID == "" || confidence < r.Threshold() {
		return RouteDecision{
			Matched:       true,
			LowConfidence: true,
			Reason:        "low_confidence",
			Action:        action,
		}, nil
	}
	return RouteDecision{
		Matched: true,
		Reason:  "nl_skill_route",
		Action:  action,
	}, nil
}

func extractSkillID(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	// Prefer explicit markers.
	for _, marker := range []string{"技能", "skill"} {
		if idx := strings.Index(strings.ToLower(text), strings.ToLower(marker)); idx >= 0 {
			tail := strings.TrimSpace(text[idx+len(marker):])
			for _, skip := range []string{"是", "为", "叫", ":", "："} {
				tail = strings.TrimSpace(strings.TrimPrefix(tail, skip))
			}
			if tail != "" {
				if m := skillIDPattern.FindStringSubmatch(tail); len(m) == 2 {
					candidate := strings.TrimSpace(m[1])
					if !isReservedSkillWord(candidate) {
						return candidate
					}
				}
			}
		}
	}
	return ""
}

func isReservedSkillWord(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "skill", "skills", "技能", "install", "bind", "disable":
		return true
	default:
		return false
	}
}
