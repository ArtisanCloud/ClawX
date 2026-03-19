package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/configplan"
	"clawx/internal/infrastructure/config"
	chatiface "clawx/internal/interfaces/chat"
)

func TestParseConfigChatCommand(t *testing.T) {
	cmd, ok, err := parseConfigChatCommand("/config plan 创建 agent review 使用 claude")
	if err != nil {
		t.Fatalf("parse command: %v", err)
	}
	if !ok {
		t.Fatalf("expected config command")
	}
	if cmd.Action != "plan" {
		t.Fatalf("unexpected action: %s", cmd.Action)
	}
	if cmd.Instruction == "" {
		t.Fatalf("expected instruction")
	}
}

func TestHandleConfigChatCommandPlanAndApply(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	planMessage := chatiface.Message{
		ConversationID: "conv-1",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 claude",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(planMessage)
	if err != nil {
		t.Fatalf("plan command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if response == "" {
		t.Fatalf("expected response")
	}

	applyMessage := chatiface.Message{
		ConversationID: "conv-1",
		UserID:         "user-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err = handleConfigChatCommand(applyMessage)
	if err != nil {
		t.Fatalf("apply command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	agent, ok := cfg.Agents["review"]
	if !ok {
		t.Fatalf("expected review agent")
	}
	if agent.ProfileID != "claude" {
		t.Fatalf("unexpected profile: %q", agent.ProfileID)
	}
}

func TestHandleConfigChatCommandNaturalLanguagePlan(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	nlMessage := chatiface.Message{
		ConversationID: "conv-nl",
		UserID:         "user-1",
		Text:           "请帮我创建一个 bid-all 智能体",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(nlMessage)
	if err != nil {
		t.Fatalf("nl command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if !containsAll(response, "已生成配置计划", "bid-all") {
		t.Fatalf("unexpected response: %s", response)
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-nl",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(showMessage)
	if err != nil {
		t.Fatalf("show command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled show")
	}
	if !containsAll(response, "来源: nl", "版本: v1") {
		t.Fatalf("unexpected show response: %s", response)
	}
}

func TestHandleConfigChatCommandNaturalLanguagePlanKeepsExplicitAgentID(t *testing.T) {
	clearPendingConfigPlans()
	message := chatiface.Message{
		ConversationID: "conv-nl-sanitize",
		UserID:         "user-1",
		Text:           "请创建 agent bid-all智能体",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(message)
	if err != nil {
		t.Fatalf("nl sanitize command failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if !containsAll(response, "bid-all智能体", "workspaces/bid-all智能体") {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestHandleConfigChatCommandNaturalLanguageNonConfigFallsBack(t *testing.T) {
	clearPendingConfigPlans()
	message := chatiface.Message{
		ConversationID: "conv-2",
		UserID:         "user-1",
		Text:           "帮我查看当前目录",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err := handleConfigChatCommand(message)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handled {
		t.Fatalf("expected non config message to fall through")
	}
}

func TestHandleConfigChatCommandNaturalLanguagePatchWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	planMessage := chatiface.Message{
		ConversationID: "conv-patch",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(planMessage); err != nil || !handled {
		t.Fatalf("plan create failed: handled=%v err=%v", handled, err)
	}

	patchMessage := chatiface.Message{
		ConversationID: "conv-patch",
		UserID:         "user-1",
		Text:           "把 workspace 改成 /tmp/review-work",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(patchMessage)
	if err != nil {
		t.Fatalf("patch failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected patch handled")
	}
	if !containsAll(response, "已更新待确认计划", "v2") {
		t.Fatalf("unexpected patch response: %s", response)
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-patch",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(showMessage)
	if err != nil || !handled {
		t.Fatalf("show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "/tmp/review-work", "版本: v2") {
		t.Fatalf("unexpected show response after patch: %s", response)
	}
}

func TestHandleConfigChatCommandNaturalLanguagePatchTimeout(t *testing.T) {
	clearPendingConfigPlans()
	planMessage := chatiface.Message{
		ConversationID: "conv-timeout",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(planMessage); err != nil || !handled {
		t.Fatalf("plan create failed: handled=%v err=%v", handled, err)
	}

	patchMessage := chatiface.Message{
		ConversationID: "conv-timeout",
		UserID:         "user-1",
		Text:           "把 timeout 改成 900",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(patchMessage)
	if err != nil || !handled {
		t.Fatalf("timeout patch failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "v2", "timeout=`900`") && !containsAll(response, "v2", "timeout=900s") {
		// Keep assertion tolerant of summary formatting.
		t.Fatalf("unexpected timeout patch response: %s", response)
	}
}

func TestHandleConfigChatCommandNaturalLanguagePatchAgentIDCorrection(t *testing.T) {
	clearPendingConfigPlans()
	planMessage := chatiface.Message{
		ConversationID: "conv-agent-id-correct",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent bid-all智能体 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(planMessage); err != nil || !handled {
		t.Fatalf("plan create failed: handled=%v err=%v", handled, err)
	}

	patchMessage := chatiface.Message{
		ConversationID: "conv-agent-id-correct",
		UserID:         "user-1",
		Text:           "你项目应该是bid-all，而不是bid-all智能体",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(patchMessage)
	if err != nil || !handled {
		t.Fatalf("agent id patch failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "已更新待确认计划", "bid-all") || strings.Contains(response, "bid-all智能体") {
		t.Fatalf("unexpected patch response: %s", response)
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-agent-id-correct",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(showMessage)
	if err != nil || !handled {
		t.Fatalf("show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "bid-all", "版本: v2") || strings.Contains(response, "bid-all智能体") {
		t.Fatalf("unexpected show response after patch: %s", response)
	}
}

func TestHandleConfigChatCommandShowContainsSummaryAndRecentTrails(t *testing.T) {
	clearPendingConfigPlans()
	planMessage := chatiface.Message{
		ConversationID: "conv-show-summary",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(planMessage); err != nil || !handled {
		t.Fatalf("plan create failed: handled=%v err=%v", handled, err)
	}

	patches := []string{
		"把 workspace 改成 /tmp/review-work",
		"把 timeout 改成 900",
	}
	for _, patchText := range patches {
		patchMessage := chatiface.Message{
			ConversationID: "conv-show-summary",
			UserID:         "user-1",
			Text:           patchText,
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		}
		if handled, _, err := handleConfigChatCommand(patchMessage); err != nil || !handled {
			t.Fatalf("patch failed: handled=%v err=%v text=%s", handled, err, patchText)
		}
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-show-summary",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(showMessage)
	if err != nil || !handled {
		t.Fatalf("show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "摘要版本: sv", "摘要字段:", "最近变更:", "set_workspace", "set_timeout") {
		t.Fatalf("show should include summary and trails: %s", response)
	}
}

func TestHandleConfigChatCommandMultiRoundPatchContextContinuity(t *testing.T) {
	clearPendingConfigPlans()
	planMessage := chatiface.Message{
		ConversationID: "conv-context-continuity",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(planMessage); err != nil || !handled {
		t.Fatalf("plan create failed: handled=%v err=%v", handled, err)
	}

	steps := []string{
		"把 workspace 改成 /tmp/review-work",
		"继续把 timeout 改成 1200",
		"再把 profile 改成 claude",
	}
	for _, text := range steps {
		msg := chatiface.Message{
			ConversationID: "conv-context-continuity",
			UserID:         "user-1",
			Text:           text,
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		}
		handled, _, err := handleConfigChatCommand(msg)
		if err != nil || !handled {
			t.Fatalf("multi-round patch failed: handled=%v err=%v text=%s", handled, err, text)
		}
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-context-continuity",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(showMessage)
	if err != nil || !handled {
		t.Fatalf("show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "workspace=`/tmp/review-work`", "timeout=1200s", "profile=`claude`", "版本: v4", "patch=3") {
		t.Fatalf("unexpected continuity result: %s", response)
	}
}

func TestHandleConfigChatCommandNaturalLanguagePatchWithoutPendingPlanFallsBack(t *testing.T) {
	clearPendingConfigPlans()
	message := chatiface.Message{
		ConversationID: "conv-no-plan-patch",
		UserID:         "user-1",
		Text:           "把 workspace 改成 /tmp/no-plan",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(message)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled no-pending-plan patch")
	}
	if !strings.Contains(response, "当前没有待确认的配置计划") {
		t.Fatalf("unexpected response: %s", response)
	}
}

func TestHandleConfigChatCommandNonAdminCanManagePlanButCannotApply(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "admin-1")

	clearPendingConfigPlans()

	user := "user-2"
	planMessage := chatiface.Message{
		ConversationID: "conv-perm",
		UserID:         user,
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err := handleConfigChatCommand(planMessage)
	if err != nil || !handled {
		t.Fatalf("non-admin plan failed: handled=%v err=%v", handled, err)
	}

	showMessage := chatiface.Message{
		ConversationID: "conv-perm",
		UserID:         user,
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(showMessage)
	if err != nil || !handled {
		t.Fatalf("non-admin show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "待确认计划", "review") {
		t.Fatalf("unexpected show response: %s", response)
	}

	applyMessage := chatiface.Message{
		ConversationID: "conv-perm",
		UserID:         user,
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(applyMessage)
	if err != nil || !handled {
		t.Fatalf("non-admin apply failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "没有配置权限") {
		t.Fatalf("unexpected apply response: %s", response)
	}

	cancelMessage := chatiface.Message{
		ConversationID: "conv-perm",
		UserID:         user,
		Text:           "/config cancel",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(cancelMessage)
	if err != nil || !handled {
		t.Fatalf("non-admin cancel failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "已取消") {
		t.Fatalf("unexpected cancel response: %s", response)
	}
}

func TestHandleConfigChatCommandApplyIsOnlyWritePath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	planMessage := chatiface.Message{
		ConversationID: "conv-write",
		UserID:         "admin-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err := handleConfigChatCommand(planMessage)
	if err != nil || !handled {
		t.Fatalf("plan failed: handled=%v err=%v", handled, err)
	}

	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		t.Fatalf("config file should not be written before apply, got err=%v", statErr)
	}

	applyMessage := chatiface.Message{
		ConversationID: "conv-write",
		UserID:         "admin-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err = handleConfigChatCommand(applyMessage)
	if err != nil || !handled {
		t.Fatalf("apply failed: handled=%v err=%v", handled, err)
	}

	if _, statErr := os.Stat(configPath); statErr != nil {
		t.Fatalf("config file should be written after apply: %v", statErr)
	}
}

func TestHandleConfigChatCommandApplyCreatesWorkspaceDir(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")
	targetWorkspace := filepath.Join(tempDir, "workspaces", "bid-all")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	planMessage := chatiface.Message{
		ConversationID: "conv-workspace-create",
		UserID:         "admin-1",
		Text:           "/config plan add agent bid-all profile codex workspace " + targetWorkspace + " timeout 600",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err := handleConfigChatCommand(planMessage)
	if err != nil || !handled {
		t.Fatalf("plan failed: handled=%v err=%v", handled, err)
	}

	if _, statErr := os.Stat(targetWorkspace); !os.IsNotExist(statErr) {
		t.Fatalf("workspace should not exist before apply, got err=%v", statErr)
	}

	applyMessage := chatiface.Message{
		ConversationID: "conv-workspace-create",
		UserID:         "admin-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(applyMessage)
	if err != nil || !handled {
		t.Fatalf("apply failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "workspace 已就绪", targetWorkspace) {
		t.Fatalf("unexpected apply response: %s", response)
	}

	info, statErr := os.Stat(targetWorkspace)
	if statErr != nil {
		t.Fatalf("workspace should be created after apply: %v", statErr)
	}
	if !info.IsDir() {
		t.Fatalf("workspace path must be directory")
	}
}

func TestHandleConfigChatCommandMixedIntentRequiresClarification(t *testing.T) {
	clearPendingConfigPlans()
	message := chatiface.Message{
		ConversationID: "conv-mixed",
		UserID:         "user-1",
		Text:           "请创建 agent bid-all，同时 go test ./...",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(message)
	if err != nil {
		t.Fatalf("mixed intent failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if !containsAll(response, "同时包含配置和任务意图", "回复“配置”", "回复“任务”") {
		t.Fatalf("unexpected response: %s", response)
	}

	state, ok := getPendingConfigClarification("conv-mixed")
	if !ok {
		t.Fatalf("expected clarification state")
	}
	if state.Action != "create" {
		t.Fatalf("unexpected clarification action: %s", state.Action)
	}
}

func TestHandleConfigChatCommandMixedIntentConfirmConfigAppliesConfigPath(t *testing.T) {
	clearPendingConfigPlans()
	first := chatiface.Message{
		ConversationID: "conv-mixed-confirm",
		UserID:         "user-1",
		Text:           "请创建 agent bid-all，同时 go test ./...",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(first); err != nil || !handled {
		t.Fatalf("mixed request failed: handled=%v err=%v", handled, err)
	}

	confirm := chatiface.Message{
		ConversationID: "conv-mixed-confirm",
		UserID:         "user-1",
		Text:           "配置",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(confirm)
	if err != nil || !handled {
		t.Fatalf("confirm failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "已生成配置计划", "bid-all") {
		t.Fatalf("unexpected response: %s", response)
	}
	if _, ok := getPendingConfigPlan("conv-mixed-confirm"); !ok {
		t.Fatalf("expected pending plan after confirm")
	}
	if _, ok := getPendingConfigClarification("conv-mixed-confirm"); ok {
		t.Fatalf("clarification should be cleared")
	}
}

func TestHandleConfigChatCommandMixedIntentConfirmTaskStopsConfigFlow(t *testing.T) {
	clearPendingConfigPlans()
	first := chatiface.Message{
		ConversationID: "conv-mixed-task",
		UserID:         "user-1",
		Text:           "请创建 agent bid-all，同时 go test ./...",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(first); err != nil || !handled {
		t.Fatalf("mixed request failed: handled=%v err=%v", handled, err)
	}

	confirm := chatiface.Message{
		ConversationID: "conv-mixed-task",
		UserID:         "user-1",
		Text:           "任务",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(confirm)
	if err != nil || !handled {
		t.Fatalf("task confirm failed: handled=%v err=%v", handled, err)
	}
	if !strings.Contains(response, "已切换到任务通道") {
		t.Fatalf("unexpected response: %s", response)
	}
	if _, ok := getPendingConfigPlan("conv-mixed-task"); ok {
		t.Fatalf("did not expect pending config plan")
	}
}

func TestHandleConfigChatCommandLowConfidenceSuggestOnly(t *testing.T) {
	clearPendingConfigPlans()
	message := chatiface.Message{
		ConversationID: "conv-low-confidence",
		UserID:         "user-1",
		Text:           "agent 怎么配比较好？",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(message)
	if err != nil {
		t.Fatalf("low confidence failed: %v", err)
	}
	if !handled {
		t.Fatalf("expected handled")
	}
	if !strings.Contains(response, "可能想改配置") {
		t.Fatalf("unexpected response: %s", response)
	}
	if _, ok := getPendingConfigPlan("conv-low-confidence"); ok {
		t.Fatalf("low confidence must not create pending plan")
	}
}

func TestHandleConfigChatCommandInteractionStepMetric(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	flow := []chatiface.Message{
		{
			ConversationID: "conv-steps",
			UserID:         "admin-1",
			Text:           "/config plan 创建 agent review 使用 codex",
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		},
		{
			ConversationID: "conv-steps",
			UserID:         "admin-1",
			Text:           "把 timeout 改成 900",
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		},
		{
			ConversationID: "conv-steps",
			UserID:         "admin-1",
			Text:           "/config show",
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		},
		{
			ConversationID: "conv-steps",
			UserID:         "admin-1",
			Text:           "/config apply",
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		},
	}
	for _, msg := range flow {
		handled, _, err := handleConfigChatCommand(msg)
		if err != nil || !handled {
			t.Fatalf("step failed: handled=%v err=%v text=%s", handled, err, msg.Text)
		}
	}
	if steps := getLastCompletedConfigStepCountForTests("conv-steps"); steps != 4 {
		t.Fatalf("expected step count 4, got %d", steps)
	}
}

func TestHandleConfigChatCommandSummaryApplyConsistency(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")
	targetWorkspace := filepath.Join(tempDir, "workspaces", "consistency")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")

	clearPendingConfigPlans()

	flow := []string{
		"/config plan add agent review profile codex workspace " + targetWorkspace + " timeout 600",
		"把 timeout 改成 1200",
		"把 profile 改成 claude",
	}
	for _, text := range flow {
		msg := chatiface.Message{
			ConversationID: "conv-summary-apply",
			UserID:         "admin-1",
			Text:           text,
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		}
		handled, _, err := handleConfigChatCommand(msg)
		if err != nil || !handled {
			t.Fatalf("flow step failed: handled=%v err=%v text=%s", handled, err, text)
		}
	}

	plan, ok := getPendingConfigPlan("conv-summary-apply")
	if !ok || plan.CompressedSummary == nil {
		t.Fatalf("expected pending plan summary before apply")
	}
	wantProfile := configplan.FieldValueFromPlan(plan, configplan.SummaryFieldProfile)
	wantWorkspace := configplan.FieldValueFromPlan(plan, configplan.SummaryFieldWorkspace)
	wantTimeout := configplan.FieldValueFromPlan(plan, configplan.SummaryFieldTimeout)

	apply := chatiface.Message{
		ConversationID: "conv-summary-apply",
		UserID:         "admin-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, _, err := handleConfigChatCommand(apply)
	if err != nil || !handled {
		t.Fatalf("apply failed: handled=%v err=%v", handled, err)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	agent, ok := cfg.Agents["review"]
	if !ok {
		t.Fatalf("expected review agent")
	}
	gotTimeout := strconv.Itoa(int(agent.Timeout.Seconds()))
	if agent.ProfileID != wantProfile || agent.Workspace != wantWorkspace || gotTimeout != wantTimeout {
		t.Fatalf("summary/apply mismatch: profile=%q/%q workspace=%q/%q timeout=%q/%q", agent.ProfileID, wantProfile, agent.Workspace, wantWorkspace, gotTimeout, wantTimeout)
	}
}

func TestHandleConfigChatCommandShowPerformanceWithHighPatchCount(t *testing.T) {
	clearPendingConfigPlans()

	plan := chatiface.Message{
		ConversationID: "conv-show-perf",
		UserID:         "user-1",
		Text:           "/config plan 创建 agent review 使用 codex",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	if handled, _, err := handleConfigChatCommand(plan); err != nil || !handled {
		t.Fatalf("plan failed: handled=%v err=%v", handled, err)
	}

	for i := 0; i < 30; i++ {
		msg := chatiface.Message{
			ConversationID: "conv-show-perf",
			UserID:         "user-1",
			Text:           "把 timeout 改成 " + strconv.Itoa(900+i),
			Channel:        "discord",
			ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
		}
		if handled, _, err := handleConfigChatCommand(msg); err != nil || !handled {
			t.Fatalf("patch failed: handled=%v err=%v idx=%d", handled, err, i)
		}
	}

	show := chatiface.Message{
		ConversationID: "conv-show-perf",
		UserID:         "user-1",
		Text:           "/config show",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	started := time.Now()
	handled, response, err := handleConfigChatCommand(show)
	cost := time.Since(started)
	if err != nil || !handled {
		t.Fatalf("show failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "摘要版本: sv", "最近变更:") {
		t.Fatalf("expected summary response: %s", response)
	}
	if cost > 500*time.Millisecond {
		t.Fatalf("show too slow with high patch count: %s", cost)
	}
}

func TestHandleConfigChatCommandPlanDeleteAgentWithWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")
	workspace := filepath.Join(tempDir, "workspaces", "bid-all")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")
	clearPendingConfigPlans()

	if err := ensureWorkspacePath(workspace); err != nil {
		t.Fatalf("prepare workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed workspace file: %v", err)
	}
	if _, err := config.UpsertAgent(config.AgentUpsertOptions{
		ID:             "bid-all",
		ProfileID:      "codex",
		Workspace:      workspace,
		TimeoutSeconds: 600,
		SetAsDefault:   true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	plan := chatiface.Message{
		ConversationID: "conv-delete-agent",
		UserID:         "admin-1",
		Text:           "/config plan 删除 agent bid-all 删除目录",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(plan)
	if err != nil || !handled {
		t.Fatalf("plan failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "已生成配置计划", "delete_workspace=true") {
		t.Fatalf("unexpected plan response: %s", response)
	}

	apply := chatiface.Message{
		ConversationID: "conv-delete-agent",
		UserID:         "admin-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(apply)
	if err != nil || !handled {
		t.Fatalf("apply failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "workspace 已删除") {
		t.Fatalf("unexpected apply response: %s", response)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, ok := cfg.Agents["bid-all"]; ok {
		t.Fatalf("agent should be removed")
	}
	if _, err := os.Stat(workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace should be removed, got err=%v", err)
	}
}

func TestHandleConfigChatCommandPlanRenameAgentMigrateWorkspace(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.json")
	homeDir := filepath.Join(tempDir, "home")
	oldWorkspace := filepath.Join(tempDir, "workspaces", "bid-all智能体")
	newWorkspace := filepath.Join(tempDir, "workspaces", "bid-all")

	t.Setenv("CLAWX_CONFIG", configPath)
	t.Setenv("HOME", homeDir)
	t.Setenv("CLAWX_CONFIG_ADMIN_USERS", "")
	clearPendingConfigPlans()

	if err := ensureWorkspacePath(oldWorkspace); err != nil {
		t.Fatalf("prepare old workspace: %v", err)
	}
	if err := os.WriteFile(filepath.Join(oldWorkspace, "README.md"), []byte("x"), 0o644); err != nil {
		t.Fatalf("seed old workspace file: %v", err)
	}
	if _, err := config.UpsertAgent(config.AgentUpsertOptions{
		ID:             "bid-all智能体",
		ProfileID:      "codex",
		Workspace:      oldWorkspace,
		TimeoutSeconds: 600,
		SetAsDefault:   true,
	}); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	plan := chatiface.Message{
		ConversationID: "conv-rename-agent",
		UserID:         "admin-1",
		Text:           "/config plan 重命名 agent bid-all智能体 为 bid-all",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err := handleConfigChatCommand(plan)
	if err != nil || !handled {
		t.Fatalf("plan failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "已生成配置计划", "bid-all智能体", "bid-all") {
		t.Fatalf("unexpected plan response: %s", response)
	}

	apply := chatiface.Message{
		ConversationID: "conv-rename-agent",
		UserID:         "admin-1",
		Text:           "/config apply",
		Channel:        "discord",
		ContextFlags:   chatiface.ContextFlags{IsAllowed: true},
	}
	handled, response, err = handleConfigChatCommand(apply)
	if err != nil || !handled {
		t.Fatalf("apply failed: handled=%v err=%v", handled, err)
	}
	if !containsAll(response, "workspace", oldWorkspace, newWorkspace) {
		t.Fatalf("unexpected apply response: %s", response)
	}

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if _, ok := cfg.Agents["bid-all智能体"]; ok {
		t.Fatalf("old agent should be removed")
	}
	newAgent, ok := cfg.Agents["bid-all"]
	if !ok {
		t.Fatalf("new agent should exist")
	}
	if newAgent.Workspace != newWorkspace {
		t.Fatalf("unexpected workspace: %s", newAgent.Workspace)
	}
	if cfg.DefaultAgentID != "bid-all" {
		t.Fatalf("default agent should follow rename, got %q", cfg.DefaultAgentID)
	}
	if _, err := os.Stat(oldWorkspace); !os.IsNotExist(err) {
		t.Fatalf("old workspace should be moved, got err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(newWorkspace, "README.md")); err != nil {
		t.Fatalf("workspace content should be migrated: %v", err)
	}
}

func clearPendingConfigPlans() {
	resetPendingConfigPlansForTests()
}

func containsAll(text string, values ...string) bool {
	for _, item := range values {
		if !strings.Contains(text, item) {
			return false
		}
	}
	return true
}
