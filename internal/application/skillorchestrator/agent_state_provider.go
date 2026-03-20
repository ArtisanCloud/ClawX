package skillorchestrator

import "strings"

type AgentState struct {
	AgentID   string
	Workspace string
	ProfileID string
	IsDefault bool
}

type AgentStateProvider interface {
	Get(agentID string) (AgentState, bool)
}

type MapAgentStateProvider struct {
	values map[string]AgentState
}

func NewMapAgentStateProvider(values map[string]AgentState) *MapAgentStateProvider {
	copied := make(map[string]AgentState, len(values))
	for key, value := range values {
		id := strings.TrimSpace(key)
		if id == "" {
			continue
		}
		value.AgentID = strings.TrimSpace(value.AgentID)
		if value.AgentID == "" {
			value.AgentID = id
		}
		copied[id] = value
	}
	return &MapAgentStateProvider{values: copied}
}

func (p *MapAgentStateProvider) Get(agentID string) (AgentState, bool) {
	if p == nil {
		return AgentState{}, false
	}
	target := strings.TrimSpace(agentID)
	if target == "" {
		return AgentState{}, false
	}
	value, ok := p.values[target]
	return value, ok
}
