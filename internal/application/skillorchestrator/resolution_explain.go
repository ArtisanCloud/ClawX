package skillorchestrator

import (
	"fmt"
	"strings"

	skilldomain "clawx/internal/domain/skill"
)

type ResolutionExplain struct {
	Resolved bool
	Binding  skilldomain.SkillBinding
	Message  string
}

func ExplainResolution(bindings []skilldomain.SkillBinding, request ResolveRequest) ResolutionExplain {
	resolver := NewBindingResolver()
	resolved, ok := resolver.Resolve(bindings, request)
	if !ok {
		return ResolutionExplain{
			Resolved: false,
			Message:  "未找到可用绑定。",
		}
	}

	message := ""
	switch resolved.Scope {
	case skilldomain.ScopeAgentLocal:
		message = fmt.Sprintf("命中 agent-local 绑定（agent=%s）。解析链路：agent-local > project > global。", strings.TrimSpace(resolved.AgentID))
	case skilldomain.ScopeProject:
		message = fmt.Sprintf("命中 project 绑定（project=%s），agent-local 未命中后回落到 project，再高于 global。", strings.TrimSpace(resolved.ProjectID))
	case skilldomain.ScopeGlobal:
		message = "命中 global 绑定：agent-local 与 project 均未命中，最终回落到 global。"
	default:
		message = "命中绑定。"
	}
	return ResolutionExplain{
		Resolved: true,
		Binding:  resolved,
		Message:  message,
	}
}
