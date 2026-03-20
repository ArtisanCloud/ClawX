package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"clawx/internal/application/skillorchestrator"
	"clawx/internal/domain/skill"
	chatiface "clawx/internal/interfaces/chat"
)

func handleSkillChatCommand(message chatiface.Message, runtime agentRuntime) (bool, string, error) {
	if runtime.skillControl == nil {
		return false, "", nil
	}
	if isSkillHelpCommand(message.Text) {
		return true, skillorchestrator.HelpText(), nil
	}

	if action, matched, err := skillorchestrator.MapCommandToAction(message.Text); matched || err != nil {
		if err != nil {
			return true, "", err
		}
		if err := action.Validate(); err != nil {
			return true, "", err
		}
		if response, executed, execErr := executeSkillAction(message, runtime, action); executed || execErr != nil {
			if execErr != nil {
				return true, "", execErr
			}
			return true, response, nil
		}
		decision := skillorchestrator.RouteDecision{
			Matched: true,
			Reason:  "command_skill_route",
			Action:  action,
		}
		log.Print(skillorchestrator.FormatRouteAuditEvent(message.ConversationID, message.UserID, decision).FormatForLog())
		return true, fmt.Sprintf("已识别技能动作：\n- %s", skillorchestrator.FormatActionSummary(action)), nil
	}

	// Natural language skill routing should be handled by the primary LLM pipeline.
	// This handler only processes explicit /skill commands.
	return false, "", nil
}

func isSkillHelpCommand(raw string) bool {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return false
	}
	if strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/") != "skill" {
		return false
	}
	if len(fields) == 1 {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(fields[1])) {
	case "help":
		return true
	default:
		return false
	}
}

func buildSkillRouteInput(ctx context.Context, runtime agentRuntime, message chatiface.Message) (skillorchestrator.RouteInput, error) {
	input := skillorchestrator.RouteInput{Message: message.Text}
	if runtime.skillControl == nil {
		return input, nil
	}
	projector := skillorchestrator.NewContextDigestProjector(50)
	if runtime.skillControl.Audit != nil {
		records := runtime.skillControl.Audit.ListByConversation(message.ConversationID, 50)
		input.ContextDigest = projector.Project(message.ConversationID, records)
	}
	if runtime.skillControl.Registry != nil {
		metadata, err := runtime.skillControl.Registry.List(ctx)
		if err != nil {
			return input, err
		}
		var bindings []skill.SkillBinding
		if runtime.skillControl.Binder != nil {
			bindings, err = runtime.skillControl.Binder.ListAll(ctx)
			if err != nil {
				return input, err
			}
		}
		builder := skillorchestrator.NewCatalogDigestBuilder()
		input.SkillCatalogHash = builder.Build(metadata, bindings).Hash
	}
	return input, nil
}

func executeSkillAction(message chatiface.Message, runtime agentRuntime, action skill.SkillAction) (string, bool, error) {
	ctx := context.Background()
	switch strings.TrimSpace(action.Intent) {
	case "install_skill":
		if runtime.skillControl.Installer == nil {
			return "", false, nil
		}
		source := parseRegistrySource(action)
		version := parseStringArg(action, "version", "v1.0.0")
		agentID := parseStringArg(action, "agent", runtime.agentID)
		packageURI := parseStringArg(action, "package", "")
		inputSchema := map[string]any{"type": "object"}
		if packageURI != "" {
			inputSchema["package_uri"] = packageURI
		}
		meta := skill.SkillMetadata{
			SkillID:      strings.TrimSpace(action.SkillID),
			Version:      version,
			Source:       source,
			InputSchema:  inputSchema,
			RiskLevel:    action.RiskLevel,
			Enabled:      true,
			Capabilities: []string{"execute"},
		}
		result, err := runtime.skillControl.Installer.Install(ctx, meta, agentID)
		if err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		if result.AgentState != nil {
			if strings.TrimSpace(result.InstalledDir) != "" {
				return fmt.Sprintf("技能安装成功：%s@%s（source=%s）\n安装目录: %s\n目标 Agent: %s workspace=%s profile=%s default=%t",
					result.SkillID, result.Version, result.Source, result.InstalledDir, result.AgentState.AgentID, result.AgentState.Workspace, result.AgentState.ProfileID, result.AgentState.IsDefault), true, nil
			}
			return fmt.Sprintf("技能安装成功：%s@%s（source=%s）\n目标 Agent: %s workspace=%s profile=%s default=%t",
				result.SkillID, result.Version, result.Source, result.AgentState.AgentID, result.AgentState.Workspace, result.AgentState.ProfileID, result.AgentState.IsDefault), true, nil
		}
		if strings.TrimSpace(result.InstalledDir) != "" {
			return fmt.Sprintf("技能安装成功：%s@%s（source=%s）\n安装目录: %s",
				result.SkillID, result.Version, result.Source, result.InstalledDir), true, nil
		}
		return fmt.Sprintf("技能安装成功：%s@%s（source=%s）", result.SkillID, result.Version, result.Source), true, nil
	case "disable_skill":
		if runtime.skillControl.Toggle == nil {
			return "", false, nil
		}
		if err := runtime.skillControl.Toggle.Disable(ctx, action.SkillID); err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		return fmt.Sprintf("技能已禁用：%s", strings.TrimSpace(action.SkillID)), true, nil
	case "bind_skill":
		if runtime.skillControl.Binder == nil {
			return "", false, nil
		}
		scope, projectID, agentID := parseBindingScope(action, runtime.agentID)
		version := parseStringArg(action, "version", "")
		binding, err := runtime.skillControl.Binder.Bind(ctx, skillorchestrator.BindRequest{
			SkillID:   strings.TrimSpace(action.SkillID),
			Version:   version,
			Scope:     scope,
			ProjectID: projectID,
			AgentID:   agentID,
		})
		if err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		return fmt.Sprintf("技能绑定成功：%s@%s（scope=%s project=%s agent=%s）",
			binding.SkillID, binding.Version, binding.Scope, binding.ProjectID, binding.AgentID), true, nil
	case "upgrade_skill":
		if runtime.skillControl.Upgrader == nil {
			return "", false, nil
		}
		version := parseStringArg(action, "version", "")
		if version == "" {
			return "", true, fmt.Errorf("缺少升级目标版本。示例：`/skill upgrade <skill_id> --version v2.0.0`")
		}
		updated, err := runtime.skillControl.Upgrader.Upgrade(ctx, action.SkillID, version)
		if err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		return fmt.Sprintf("技能升级成功：%s@%s", updated.SkillID, updated.Version), true, nil
	case "run_skill":
		if runtime.skillControl.Executor == nil {
			return "", false, nil
		}
		metadata := skill.SkillMetadata{
			SkillID:      strings.TrimSpace(action.SkillID),
			Version:      parseStringArg(action, "version", "v1.0.0"),
			Source:       parseRegistrySource(action),
			InputSchema:  map[string]any{"type": "object"},
			RiskLevel:    action.RiskLevel,
			Enabled:      true,
			Capabilities: []string{"execute"},
		}
		if runtime.skillControl.Registry != nil {
			if existing, err := runtime.skillControl.Registry.Get(ctx, action.SkillID); err == nil {
				metadata = existing
			}
		}
		if version := parseStringArg(action, "version", ""); version != "" {
			metadata.Version = version
		}
		if risk := parseRiskLevel(action); risk != "" {
			action.RiskLevel = risk
			action.RequiresConfirmation = risk == skill.RiskHigh
			metadata.RiskLevel = risk
		}
		confirmationID, decision := parseConfirmation(action)
		projectID := parseStringArg(action, "project", "")
		agentID := parseStringArg(action, "agent", runtime.agentID)
		result, err := runtime.skillControl.Executor.Execute(ctx, skillorchestrator.ExecuteRequest{
			Action:               action,
			Metadata:             metadata,
			ProjectID:            projectID,
			AgentID:              agentID,
			Actor:                strings.TrimSpace(message.UserID),
			ConversationID:       strings.TrimSpace(message.ConversationID),
			ConfirmationID:       confirmationID,
			ConfirmationDecision: decision,
		})
		if err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		return fmt.Sprintf("技能执行成功：%s（trace=%s）", result.SkillID, result.TraceID), true, nil
	case "replay_skill_audit":
		if runtime.skillControl.Replay == nil {
			return "", false, nil
		}
		records, err := runtime.skillControl.Replay.ReplayTrace(action.SkillID)
		if err != nil {
			return "", true, fmt.Errorf(skillorchestrator.UserFacingError(err))
		}
		var lines []string
		lines = append(lines, fmt.Sprintf("审计回放：trace=%s events=%d", action.SkillID, len(records)))
		for _, record := range records {
			lines = append(lines, fmt.Sprintf("- %s result=%s error=%q skill=%s source=%s intent=%s args=%v",
				record.EventType, record.Result, record.Error, record.SkillID, record.Source, record.Intent, record.Arguments))
		}
		return strings.Join(lines, "\n"), true, nil
	default:
		_ = message
		return "", false, nil
	}
}

func parseRegistrySource(action skill.SkillAction) skill.RegistrySource {
	raw := parseStringArg(action, "source", "builtin")
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "builtin":
		return skill.RegistrySourceBuiltin
	case "clawhub":
		return skill.RegistrySourceClawHub
	case "local":
		return skill.RegistrySourceLocal
	case "git":
		return skill.RegistrySourceGit
	default:
		return skill.RegistrySourceBuiltin
	}
}

func parseStringArg(action skill.SkillAction, key, fallback string) string {
	if action.Arguments != nil {
		if raw, ok := action.Arguments[key]; ok {
			if value, ok := raw.(string); ok {
				value = strings.TrimSpace(value)
				if value != "" {
					return value
				}
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func parseBindingScope(action skill.SkillAction, defaultAgentID string) (skill.BindingScope, string, string) {
	scope := strings.ToLower(parseStringArg(action, "scope", ""))
	projectID := parseStringArg(action, "project", "")
	agentID := parseStringArg(action, "agent", defaultAgentID)
	switch scope {
	case "global":
		return skill.ScopeGlobal, "", ""
	case "project":
		return skill.ScopeProject, projectID, ""
	case "agent-local", "agent":
		return skill.ScopeAgentLocal, "", agentID
	default:
		if projectID != "" {
			return skill.ScopeProject, projectID, ""
		}
		return skill.ScopeAgentLocal, "", agentID
	}
}

func parseRiskLevel(action skill.SkillAction) skill.RiskLevel {
	raw := strings.ToLower(parseStringArg(action, "risk", ""))
	switch raw {
	case string(skill.RiskHigh):
		return skill.RiskHigh
	case string(skill.RiskMedium):
		return skill.RiskMedium
	case string(skill.RiskLow):
		return skill.RiskLow
	default:
		return ""
	}
}

func parseConfirmation(action skill.SkillAction) (string, string) {
	if id := parseStringArg(action, "reject", ""); id != "" {
		return id, "reject"
	}
	if id := parseStringArg(action, "confirm", ""); id != "" {
		return id, "approve"
	}
	return "", ""
}
