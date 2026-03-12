package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode"
)

var (
	ErrInvalidConfig       = errors.New("invalid config")
	ErrForbiddenCWD        = errors.New("cwd is outside allowed roots")
	ErrUnknownAgent        = errors.New("default agent is not defined")
	ErrUnknownAgentProfile = errors.New("agent references an unknown provider profile")
	ErrConfigKeyNotFound   = errors.New("config key not found")
)

type ProviderProfile struct {
	ID         string
	Kind       string
	Command    string
	Args       []string
	HealthArgs []string
	Model      string
}

type Agent struct {
	ID        string
	ProfileID string
	Workspace string
	Timeout   time.Duration
	IsDefault bool
}

type DatabaseConfig struct {
	Enabled    bool
	Driver     string
	Host       string
	Port       int
	Name       string
	User       string
	Password   string
	SSLMode    string
	AutoCreate bool
}

type DiscordInstance struct {
	ID                string
	Enabled           bool
	BotToken          string
	APIBaseURL        string
	GatewayURL        string
	AllowedChannelIDs []string
	RequireMention    bool
	DefaultAgentID    string
	AgentBindings     map[string]string
}

type TelegramInstance struct {
	ID                      string
	Enabled                 bool
	Mode                    string
	Token                   string
	BotUsername             string
	AllowedChatIDs          []string
	RequireCommandOrMention bool
	PollingTimeout          time.Duration
	WebhookURL              string
	WebhookPath             string
	WebhookSecret           string
	DefaultAgentID          string
	AgentBindings           map[string]string
}

type FeishuInstance struct {
	ID                string
	Enabled           bool
	Mode              string
	AppID             string
	AppSecret         string
	VerificationToken string
	EncryptKey        string
	DefaultAgentID    string
	AgentBindings     map[string]string
}

type WeComInstance struct {
	ID             string
	Enabled        bool
	Mode           string
	CorpID         string
	AgentID        string
	Secret         string
	Token          string
	EncodingAESKey string
	DefaultAgentID string
	AgentBindings  map[string]string
}

type SkillSources struct {
	UserDir        string
	WorkspaceDir   string
	BuiltinEnabled bool
	BuiltinDir     string
}

type SkillAllowlist struct {
	Users    []string
	Channels []string
}

type SkillConfig struct {
	Enabled       bool
	Sources       SkillSources
	DisabledNames []string
	Allowlist     SkillAllowlist
	DefaultMode   string
	PairingTTL    time.Duration
}

type LLMFallbackConfig struct {
	Enabled             bool
	ConfidenceThreshold float64
}

type IntentRouterConfig struct {
	Mode        string
	LLMFallback LLMFallbackConfig
}

type Snapshot struct {
	AllowedRoots                  []string
	DefaultCWD                    string
	Timeout                       time.Duration
	ExecCommand                   string
	ExecArgs                      []string
	ExecHealthArgs                []string
	DiscordEnabled                bool
	DiscordBotToken               string
	DiscordAPIBaseURL             string
	DiscordGatewayURL             string
	DiscordAllowedChannelIDs      []string
	DiscordRequireMention         bool
	TelegramEnabled               bool
	TelegramMode                  string
	TelegramToken                 string
	TelegramBotUsername           string
	TelegramAllowedChatIDs        []string
	TelegramRequireCommandMention bool
	TelegramPollingTimeout        time.Duration
	TelegramWebhookURL            string
	TelegramWebhookPath           string
	TelegramWebhookSecret         string
	FeishuEnabled                 bool
	FeishuMode                    string
	FeishuAppID                   string
	FeishuAppSecret               string
	FeishuVerificationToken       string
	FeishuEncryptKey              string
	WeComEnabled                  bool
	WeComMode                     string
	WeComCorpID                   string
	WeComAgentID                  string
	WeComSecret                   string
	WeComToken                    string
	WeComEncodingAESKey           string
	HealthProbeEnabled            bool
	HTTPListenAddr                string
	HealthProbePath               string
	Database                      DatabaseConfig

	ProviderProfiles map[string]ProviderProfile
	Agents           map[string]Agent
	DefaultAgentID   string
	ActiveProfile    *ProviderProfile
	ActiveAgent      *Agent

	DiscordInstances       []DiscordInstance
	TelegramInstances      []TelegramInstance
	DiscordDefaultAgentID  string
	DiscordAgentBindings   map[string]string
	TelegramDefaultAgentID string
	TelegramAgentBindings  map[string]string
	FeishuInstances        []FeishuInstance
	FeishuDefaultAgentID   string
	FeishuAgentBindings    map[string]string
	WeComInstances         []WeComInstance
	WeComDefaultAgentID    string
	WeComAgentBindings     map[string]string
	Skills                 SkillConfig
	IntentRouter           IntentRouterConfig
}

type jsonSnapshot struct {
	Runtime      *jsonRuntime      `json:"runtime"`
	Execution    *jsonExecution    `json:"execution"`
	Providers    *jsonProviders    `json:"providers"`
	Agents       *jsonAgents       `json:"agents"`
	Channels     *jsonChannels     `json:"channels"`
	Gateway      *jsonGateway      `json:"gateway"`
	Database     *jsonDatabase     `json:"database"`
	Skills       *jsonSkills       `json:"skills"`
	IntentRouter *jsonIntentRouter `json:"intentRouter"`

	AllowedRoots                  []string                   `json:"allowed_roots"`
	DefaultCWD                    string                     `json:"default_cwd"`
	TimeoutSeconds                int                        `json:"timeout_seconds"`
	ExecCommand                   string                     `json:"exec_command"`
	ExecArgs                      []string                   `json:"exec_args"`
	ExecHealthArgs                []string                   `json:"exec_health_args"`
	DiscordEnabled                *bool                      `json:"discord_enabled"`
	DiscordBotToken               string                     `json:"discord_bot_token"`
	DiscordAPIBaseURL             string                     `json:"discord_api_base_url"`
	DiscordGatewayURL             string                     `json:"discord_gateway_url"`
	DiscordAllowedChannelIDs      []string                   `json:"discord_allowed_channel_ids"`
	DiscordRequireMention         *bool                      `json:"discord_require_mention"`
	TelegramEnabled               *bool                      `json:"telegram_enabled"`
	TelegramMode                  string                     `json:"telegram_mode"`
	TelegramToken                 string                     `json:"telegram_token"`
	TelegramBotUsername           string                     `json:"telegram_bot_username"`
	TelegramAllowedChatIDs        []string                   `json:"telegram_allowed_chat_ids"`
	TelegramRequireCommandMention *bool                      `json:"telegram_require_command_or_mention"`
	TelegramPollingSeconds        int                        `json:"telegram_polling_seconds"`
	TelegramWebhookURL            string                     `json:"telegram_webhook_url"`
	TelegramWebhookPath           string                     `json:"telegram_webhook_path"`
	TelegramWebhookSecret         string                     `json:"telegram_webhook_secret"`
	HealthProbeEnabled            *bool                      `json:"health_probe_enabled"`
	HTTPListenAddr                string                     `json:"http_listen_addr"`
	HealthProbePath               string                     `json:"health_path"`
	LegacyProviders               *jsonLegacyProviderSection `json:"legacy_providers"`
}

type jsonRuntime struct {
	AllowedRoots   []string `json:"allowedRoots"`
	DefaultCWD     string   `json:"defaultCwd"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type jsonExecution struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	HealthArgs []string `json:"healthArgs"`
}

type jsonProviders struct {
	Profiles map[string]jsonProviderProfile `json:"profiles"`
}

type jsonProviderProfile struct {
	Kind       string   `json:"kind"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	HealthArgs []string `json:"healthArgs"`
	Model      string   `json:"model"`
}

type jsonLegacyProviderSection struct {
	Profiles map[string]jsonProviderProfile `json:"profiles"`
}

type jsonAgents struct {
	Default string      `json:"default"`
	List    []jsonAgent `json:"list"`
}

type jsonAgent struct {
	ID             string `json:"id"`
	Profile        string `json:"profile"`
	Workspace      string `json:"workspace"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Default        bool   `json:"default"`
}

type jsonChannels struct {
	Discord  *jsonDiscordChannel  `json:"discord"`
	Telegram *jsonTelegramChannel `json:"telegram"`
	Feishu   *jsonFeishuChannel   `json:"feishu"`
	WeCom    *jsonWeComChannel    `json:"wecom"`
}

type jsonDiscordChannel struct {
	Enabled           *bool                 `json:"enabled"`
	BotToken          string                `json:"botToken"`
	APIBaseURL        string                `json:"apiBaseUrl"`
	GatewayURL        string                `json:"gatewayUrl"`
	AllowedChannelIDs []string              `json:"allowedChannelIds"`
	RequireMention    *bool                 `json:"requireMention"`
	DefaultAgent      string                `json:"defaultAgent"`
	AgentBindings     map[string]string     `json:"agentBindings"`
	Instances         []jsonDiscordInstance `json:"instances"`
}

type jsonDiscordInstance struct {
	ID                string            `json:"id"`
	Enabled           *bool             `json:"enabled"`
	BotToken          string            `json:"botToken"`
	APIBaseURL        string            `json:"apiBaseUrl"`
	GatewayURL        string            `json:"gatewayUrl"`
	AllowedChannelIDs []string          `json:"allowedChannelIds"`
	RequireMention    *bool             `json:"requireMention"`
	DefaultAgent      string            `json:"defaultAgent"`
	AgentBindings     map[string]string `json:"agentBindings"`
}

type jsonTelegramChannel struct {
	Enabled                 *bool                  `json:"enabled"`
	Mode                    string                 `json:"mode"`
	Token                   string                 `json:"token"`
	BotUsername             string                 `json:"botUsername"`
	AllowedChatIDs          []string               `json:"allowedChatIds"`
	RequireCommandOrMention *bool                  `json:"requireCommandOrMention"`
	PollingSeconds          int                    `json:"pollingSeconds"`
	WebhookURL              string                 `json:"webhookUrl"`
	WebhookPath             string                 `json:"webhookPath"`
	WebhookSecret           string                 `json:"webhookSecret"`
	DefaultAgent            string                 `json:"defaultAgent"`
	AgentBindings           map[string]string      `json:"agentBindings"`
	Instances               []jsonTelegramInstance `json:"instances"`
}

type jsonTelegramInstance struct {
	ID                      string            `json:"id"`
	Enabled                 *bool             `json:"enabled"`
	Mode                    string            `json:"mode"`
	Token                   string            `json:"token"`
	BotUsername             string            `json:"botUsername"`
	AllowedChatIDs          []string          `json:"allowedChatIds"`
	RequireCommandOrMention *bool             `json:"requireCommandOrMention"`
	PollingSeconds          int               `json:"pollingSeconds"`
	WebhookURL              string            `json:"webhookUrl"`
	WebhookPath             string            `json:"webhookPath"`
	WebhookSecret           string            `json:"webhookSecret"`
	DefaultAgent            string            `json:"defaultAgent"`
	AgentBindings           map[string]string `json:"agentBindings"`
}

type jsonFeishuChannel struct {
	Enabled           *bool                `json:"enabled"`
	Mode              string               `json:"mode"`
	AppID             string               `json:"appId"`
	AppSecret         string               `json:"appSecret"`
	VerificationToken string               `json:"verificationToken"`
	EncryptKey        string               `json:"encryptKey"`
	DefaultAgent      string               `json:"defaultAgent"`
	AgentBindings     map[string]string    `json:"agentBindings"`
	Instances         []jsonFeishuInstance `json:"instances"`
}

type jsonFeishuInstance struct {
	ID                string            `json:"id"`
	Enabled           *bool             `json:"enabled"`
	Mode              string            `json:"mode"`
	AppID             string            `json:"appId"`
	AppSecret         string            `json:"appSecret"`
	VerificationToken string            `json:"verificationToken"`
	EncryptKey        string            `json:"encryptKey"`
	DefaultAgent      string            `json:"defaultAgent"`
	AgentBindings     map[string]string `json:"agentBindings"`
}

type jsonWeComChannel struct {
	Enabled        *bool               `json:"enabled"`
	Mode           string              `json:"mode"`
	CorpID         string              `json:"corpId"`
	AgentID        string              `json:"agentId"`
	Secret         string              `json:"secret"`
	Token          string              `json:"token"`
	EncodingAESKey string              `json:"encodingAesKey"`
	DefaultAgent   string              `json:"defaultAgent"`
	AgentBindings  map[string]string   `json:"agentBindings"`
	Instances      []jsonWeComInstance `json:"instances"`
}

type jsonWeComInstance struct {
	ID             string            `json:"id"`
	Enabled        *bool             `json:"enabled"`
	Mode           string            `json:"mode"`
	CorpID         string            `json:"corpId"`
	AgentID        string            `json:"agentId"`
	Secret         string            `json:"secret"`
	Token          string            `json:"token"`
	EncodingAESKey string            `json:"encodingAesKey"`
	DefaultAgent   string            `json:"defaultAgent"`
	AgentBindings  map[string]string `json:"agentBindings"`
}

type jsonGateway struct {
	ListenAddr string          `json:"listenAddr"`
	Health     *jsonGatewayRef `json:"health"`
}

type jsonGatewayRef struct {
	Enabled *bool  `json:"enabled"`
	Path    string `json:"path"`
}

type jsonDatabase struct {
	Enabled    *bool  `json:"enabled"`
	Driver     string `json:"driver"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Name       string `json:"name"`
	User       string `json:"user"`
	Password   string `json:"password"`
	SSLMode    string `json:"sslMode"`
	AutoCreate *bool  `json:"autoCreate"`
}

type jsonSkills struct {
	Enabled           *bool               `json:"enabled"`
	Sources           *jsonSkillSources   `json:"sources"`
	DisabledNames     []string            `json:"disabledNames"`
	Allowlist         *jsonSkillAllowlist `json:"allowlist"`
	DefaultMode       string              `json:"defaultMode"`
	PairingTTLSeconds int                 `json:"pairingTTLSeconds"`
}

type jsonSkillSources struct {
	UserDir        string `json:"userDir"`
	WorkspaceDir   string `json:"workspaceDir"`
	BuiltinEnabled *bool  `json:"builtinEnabled"`
	BuiltinDir     string `json:"builtinDir"`
}

type jsonSkillAllowlist struct {
	Users    []string `json:"users"`
	Channels []string `json:"channels"`
}

type jsonIntentRouter struct {
	Mode        string                 `json:"mode"`
	LLMFallback *jsonIntentLLMFallback `json:"llmFallback"`
}

type jsonIntentLLMFallback struct {
	Enabled             *bool   `json:"enabled"`
	ConfidenceThreshold float64 `json:"confidenceThreshold"`
}

type BootstrapOptions struct {
	BaseProfileID         string
	DefaultProfileID      string
	MainWorkspace         string
	TelegramEnabled       bool
	TelegramToken         string
	TelegramBotUsername   string
	DiscordEnabled        bool
	DiscordBotToken       string
	DiscordRequireMention bool
	DatabaseEnabled       bool
	DatabaseDriver        string
	DatabaseHost          string
	DatabasePort          int
	DatabaseName          string
	DatabaseUser          string
	DatabasePassword      string
	DatabaseSSLMode       string
	DatabaseAutoCreate    bool
}

type AgentUpsertOptions struct {
	ID             string
	ProfileID      string
	Workspace      string
	TimeoutSeconds int
	SetAsDefault   bool
}

type fileSnapshot struct {
	Runtime      fileRuntime      `json:"runtime"`
	Providers    fileProviders    `json:"providers"`
	Agents       fileAgents       `json:"agents"`
	Execution    fileExecution    `json:"execution"`
	Channels     fileChannels     `json:"channels"`
	Gateway      fileGateway      `json:"gateway"`
	Database     fileDatabase     `json:"database"`
	Skills       fileSkills       `json:"skills"`
	IntentRouter fileIntentRouter `json:"intentRouter"`
}

type fileRuntime struct {
	AllowedRoots   []string `json:"allowedRoots"`
	DefaultCWD     string   `json:"defaultCwd"`
	TimeoutSeconds int      `json:"timeoutSeconds"`
}

type fileProviders struct {
	Profiles map[string]fileProviderProfile `json:"profiles"`
}

type fileProviderProfile struct {
	Kind       string   `json:"kind"`
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	HealthArgs []string `json:"healthArgs"`
	Model      string   `json:"model,omitempty"`
}

type fileAgents struct {
	Default string      `json:"default"`
	List    []fileAgent `json:"list"`
}

type fileAgent struct {
	ID             string `json:"id"`
	Profile        string `json:"profile"`
	Workspace      string `json:"workspace"`
	TimeoutSeconds int    `json:"timeoutSeconds"`
	Default        bool   `json:"default,omitempty"`
}

type fileExecution struct {
	Command    string   `json:"command"`
	Args       []string `json:"args"`
	HealthArgs []string `json:"healthArgs"`
}

type fileChannels struct {
	Discord  fileDiscordChannel  `json:"discord"`
	Telegram fileTelegramChannel `json:"telegram"`
	Feishu   fileFeishuChannel   `json:"feishu"`
	WeCom    fileWeComChannel    `json:"wecom"`
}

type fileDiscordChannel struct {
	Enabled           bool                  `json:"enabled"`
	BotToken          string                `json:"botToken"`
	APIBaseURL        string                `json:"apiBaseUrl"`
	GatewayURL        string                `json:"gatewayUrl"`
	AllowedChannelIDs []string              `json:"allowedChannelIds"`
	RequireMention    bool                  `json:"requireMention"`
	DefaultAgent      string                `json:"defaultAgent,omitempty"`
	AgentBindings     map[string]string     `json:"agentBindings,omitempty"`
	Instances         []fileDiscordInstance `json:"instances,omitempty"`
}

type fileDiscordInstance struct {
	ID                string            `json:"id"`
	Enabled           bool              `json:"enabled"`
	BotToken          string            `json:"botToken"`
	APIBaseURL        string            `json:"apiBaseUrl"`
	GatewayURL        string            `json:"gatewayUrl"`
	AllowedChannelIDs []string          `json:"allowedChannelIds,omitempty"`
	RequireMention    bool              `json:"requireMention"`
	DefaultAgent      string            `json:"defaultAgent,omitempty"`
	AgentBindings     map[string]string `json:"agentBindings,omitempty"`
}

type fileTelegramChannel struct {
	Enabled                 bool                   `json:"enabled"`
	Mode                    string                 `json:"mode"`
	Token                   string                 `json:"token"`
	BotUsername             string                 `json:"botUsername"`
	AllowedChatIDs          []string               `json:"allowedChatIds"`
	RequireCommandOrMention bool                   `json:"requireCommandOrMention"`
	PollingSeconds          int                    `json:"pollingSeconds"`
	WebhookURL              string                 `json:"webhookUrl,omitempty"`
	WebhookPath             string                 `json:"webhookPath,omitempty"`
	WebhookSecret           string                 `json:"webhookSecret,omitempty"`
	DefaultAgent            string                 `json:"defaultAgent,omitempty"`
	AgentBindings           map[string]string      `json:"agentBindings,omitempty"`
	Instances               []fileTelegramInstance `json:"instances,omitempty"`
}

type fileTelegramInstance struct {
	ID                      string            `json:"id"`
	Enabled                 bool              `json:"enabled"`
	Mode                    string            `json:"mode"`
	Token                   string            `json:"token"`
	BotUsername             string            `json:"botUsername"`
	AllowedChatIDs          []string          `json:"allowedChatIds,omitempty"`
	RequireCommandOrMention bool              `json:"requireCommandOrMention"`
	PollingSeconds          int               `json:"pollingSeconds"`
	WebhookURL              string            `json:"webhookUrl,omitempty"`
	WebhookPath             string            `json:"webhookPath,omitempty"`
	WebhookSecret           string            `json:"webhookSecret,omitempty"`
	DefaultAgent            string            `json:"defaultAgent,omitempty"`
	AgentBindings           map[string]string `json:"agentBindings,omitempty"`
}

type fileFeishuChannel struct {
	Enabled           bool                 `json:"enabled"`
	Mode              string               `json:"mode"`
	AppID             string               `json:"appId"`
	AppSecret         string               `json:"appSecret"`
	VerificationToken string               `json:"verificationToken"`
	EncryptKey        string               `json:"encryptKey,omitempty"`
	DefaultAgent      string               `json:"defaultAgent,omitempty"`
	AgentBindings     map[string]string    `json:"agentBindings,omitempty"`
	Instances         []fileFeishuInstance `json:"instances,omitempty"`
}

type fileFeishuInstance struct {
	ID                string            `json:"id"`
	Enabled           bool              `json:"enabled"`
	Mode              string            `json:"mode"`
	AppID             string            `json:"appId"`
	AppSecret         string            `json:"appSecret"`
	VerificationToken string            `json:"verificationToken"`
	EncryptKey        string            `json:"encryptKey,omitempty"`
	DefaultAgent      string            `json:"defaultAgent,omitempty"`
	AgentBindings     map[string]string `json:"agentBindings,omitempty"`
}

type fileWeComChannel struct {
	Enabled        bool                `json:"enabled"`
	Mode           string              `json:"mode"`
	CorpID         string              `json:"corpId"`
	AgentID        string              `json:"agentId"`
	Secret         string              `json:"secret"`
	Token          string              `json:"token"`
	EncodingAESKey string              `json:"encodingAesKey"`
	DefaultAgent   string              `json:"defaultAgent,omitempty"`
	AgentBindings  map[string]string   `json:"agentBindings,omitempty"`
	Instances      []fileWeComInstance `json:"instances,omitempty"`
}

type fileWeComInstance struct {
	ID             string            `json:"id"`
	Enabled        bool              `json:"enabled"`
	Mode           string            `json:"mode"`
	CorpID         string            `json:"corpId"`
	AgentID        string            `json:"agentId"`
	Secret         string            `json:"secret"`
	Token          string            `json:"token"`
	EncodingAESKey string            `json:"encodingAesKey"`
	DefaultAgent   string            `json:"defaultAgent,omitempty"`
	AgentBindings  map[string]string `json:"agentBindings,omitempty"`
}

type fileGateway struct {
	ListenAddr string            `json:"listenAddr"`
	Health     fileGatewayHealth `json:"health"`
}

type fileGatewayHealth struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type fileDatabase struct {
	Enabled    bool   `json:"enabled"`
	Driver     string `json:"driver"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Name       string `json:"name"`
	User       string `json:"user"`
	Password   string `json:"password"`
	SSLMode    string `json:"sslMode"`
	AutoCreate bool   `json:"autoCreate"`
}

type fileSkills struct {
	Enabled           bool               `json:"enabled"`
	Sources           fileSkillSources   `json:"sources"`
	DisabledNames     []string           `json:"disabledNames"`
	Allowlist         fileSkillAllowlist `json:"allowlist"`
	DefaultMode       string             `json:"defaultMode"`
	PairingTTLSeconds int                `json:"pairingTTLSeconds"`
}

type fileSkillSources struct {
	UserDir        string `json:"userDir"`
	WorkspaceDir   string `json:"workspaceDir"`
	BuiltinEnabled bool   `json:"builtinEnabled"`
	BuiltinDir     string `json:"builtinDir"`
}

type fileSkillAllowlist struct {
	Users    []string `json:"users"`
	Channels []string `json:"channels"`
}

type fileIntentRouter struct {
	Mode        string                `json:"mode"`
	LLMFallback fileIntentLLMFallback `json:"llmFallback"`
}

type fileIntentLLMFallback struct {
	Enabled             bool    `json:"enabled"`
	ConfidenceThreshold float64 `json:"confidenceThreshold"`
}

func Load() (Snapshot, error) {
	if err := EnsureStateLayout(); err != nil {
		return Snapshot{}, err
	}

	cfg := defaultSnapshot()
	path := configPath()
	dotEnvPath := configDotEnvPath(path)
	if shouldLoadDotEnv(path) {
		if err := loadDotEnvIfPresent(dotEnvPath); err != nil {
			return Snapshot{}, err
		}
	}
	if err := applyJSONConfigIfPresent(&cfg, path); err != nil {
		return Snapshot{}, err
	}
	if err := applyEnvOverrides(&cfg); err != nil {
		return Snapshot{}, err
	}
	if len(cfg.AllowedRoots) == 0 {
		cfg.AllowedRoots = []string{cfg.DefaultCWD}
	}
	if err := cfg.resolveActiveAgent(); err != nil {
		return Snapshot{}, err
	}
	cfg.normalizeChannelInstances()
	cfg.normalizeDatabase()
	cfg.normalizeSkills()
	cfg.normalizeIntentRouter()
	if err := cfg.Validate(); err != nil {
		return Snapshot{}, err
	}
	return cfg, nil
}

func Path() string {
	return configPath()
}

func StateDir() string {
	return stateDir()
}

func EnsureStateLayout() error {
	state := stateDir()
	if strings.TrimSpace(state) == "" {
		return fmt.Errorf("state directory is empty")
	}
	if err := os.MkdirAll(state, 0o755); err != nil {
		return fmt.Errorf("create state directory %q: %w", state, err)
	}
	if err := migrateLegacyWorkspaceLayout(); err != nil {
		return err
	}
	workspaceRoot := defaultWorkspaceRoot()
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		return fmt.Errorf("create workspace root %q: %w", workspaceRoot, err)
	}
	skillsRoot := defaultSkillsRoot()
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return fmt.Errorf("create skills root %q: %w", skillsRoot, err)
	}
	if err := ensureStarterSkill(skillsRoot); err != nil {
		return fmt.Errorf("create starter skill: %w", err)
	}
	if err := os.MkdirAll(SkillStateDir(), 0o755); err != nil {
		return fmt.Errorf("create state cache root %q: %w", SkillStateDir(), err)
	}
	return nil
}

func ensureStarterSkill(skillsRoot string) error {
	manifestPath := filepath.Join(skillsRoot, "echo", "SKILL.md")
	if _, err := os.Stat(manifestPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return err
	}
	content := strings.TrimSpace(`---
name: echo
description: 回显输入内容
aliases:
  - repeat
---
请回显用户输入。`) + "\n"
	return os.WriteFile(manifestPath, []byte(content), 0o644)
}

func migrateLegacyWorkspaceLayout() error {
	legacyRoot := filepath.Join(stateDir(), "workworkspace")
	legacyInfo, err := os.Stat(legacyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat legacy workspace root %q: %w", legacyRoot, err)
	}
	if !legacyInfo.IsDir() {
		return fmt.Errorf("legacy workspace root %q is not a directory", legacyRoot)
	}

	currentRoot := defaultWorkspaceRoot()
	if err := os.MkdirAll(currentRoot, 0o755); err != nil {
		return fmt.Errorf("create workspace root %q: %w", currentRoot, err)
	}
	if err := moveDirContents(legacyRoot, currentRoot); err != nil {
		return fmt.Errorf("migrate legacy workspace root %q -> %q: %w", legacyRoot, currentRoot, err)
	}
	if err := rewriteLegacyWorkspacePathsInConfig(legacyRoot, currentRoot); err != nil {
		return fmt.Errorf("rewrite workspace paths in config: %w", err)
	}
	if empty, err := dirEmpty(legacyRoot); err == nil && empty {
		_ = os.Remove(legacyRoot)
	}
	return nil
}

func moveDirContents(srcRoot, dstRoot string) error {
	entries, err := os.ReadDir(srcRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(srcRoot, entry.Name())
		dstPath := filepath.Join(dstRoot, entry.Name())
		if err := movePath(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func movePath(srcPath, dstPath string) error {
	if _, err := os.Stat(dstPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.Rename(srcPath, dstPath); err != nil {
				if errors.Is(err, syscall.EXDEV) {
					return copyAndRemovePath(srcPath, dstPath)
				}
				return err
			}
			return nil
		}
		return err
	}

	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	dstInfo, err := os.Stat(dstPath)
	if err != nil {
		return err
	}

	if srcInfo.IsDir() && dstInfo.IsDir() {
		if err := moveDirContents(srcPath, dstPath); err != nil {
			return err
		}
		if empty, err := dirEmpty(srcPath); err == nil && empty {
			_ = os.Remove(srcPath)
		}
		return nil
	}

	// Keep destination when file/object already exists; preserve source as legacy backup.
	return nil
}

func copyAndRemovePath(srcPath, dstPath string) error {
	srcInfo, err := os.Stat(srcPath)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		if err := copyDir(srcPath, dstPath, srcInfo.Mode()); err != nil {
			return err
		}
		return os.RemoveAll(srcPath)
	}
	if err := copyFile(srcPath, dstPath, srcInfo.Mode()); err != nil {
		return err
	}
	return os.Remove(srcPath)
}

func copyDir(srcPath, dstPath string, mode os.FileMode) error {
	if err := os.MkdirAll(dstPath, mode.Perm()); err != nil {
		return err
	}
	entries, err := os.ReadDir(srcPath)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		childSrc := filepath.Join(srcPath, entry.Name())
		childDst := filepath.Join(dstPath, entry.Name())
		if err := copyAndRemovePath(childSrc, childDst); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(srcPath, dstPath string, mode os.FileMode) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	return nil
}

func dirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}
	return len(entries) == 0, nil
}

func rewriteLegacyWorkspacePathsInConfig(legacyRoot, currentRoot string) error {
	path := configPath()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat config file %q: %w", path, err)
	}

	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return err
	}

	changed := false
	if updated, ok := rewriteWorkspacePath(file.Runtime.DefaultCWD, legacyRoot, currentRoot); ok {
		file.Runtime.DefaultCWD = updated
		changed = true
	}
	for idx := range file.Runtime.AllowedRoots {
		if updated, ok := rewriteWorkspacePath(file.Runtime.AllowedRoots[idx], legacyRoot, currentRoot); ok {
			file.Runtime.AllowedRoots[idx] = updated
			changed = true
		}
	}
	for idx := range file.Agents.List {
		if updated, ok := rewriteWorkspacePath(file.Agents.List[idx].Workspace, legacyRoot, currentRoot); ok {
			file.Agents.List[idx].Workspace = updated
			changed = true
		}
	}

	if !changed {
		return nil
	}
	if err := writeFileSnapshot(path, file); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func rewriteWorkspacePath(raw, legacyRoot, currentRoot string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return raw, false
	}

	cleanValue := filepath.Clean(trimmed)
	cleanLegacy := filepath.Clean(legacyRoot)
	cleanCurrent := filepath.Clean(currentRoot)
	if cleanValue == cleanLegacy {
		return cleanCurrent, true
	}

	legacyPrefix := cleanLegacy + string(os.PathSeparator)
	if strings.HasPrefix(cleanValue, legacyPrefix) {
		return filepath.Join(cleanCurrent, strings.TrimPrefix(cleanValue, legacyPrefix)), true
	}
	return raw, false
}

func Exists() (string, bool, error) {
	path := configPath()
	_, err := os.Stat(path)
	if err == nil {
		return path, true, nil
	}
	if os.IsNotExist(err) {
		return path, false, nil
	}
	return path, false, fmt.Errorf("stat config file: %w", err)
}

func EnsureDefaultFile() (string, bool, error) {
	path, exists, err := Exists()
	if err != nil {
		return "", false, err
	}
	if exists {
		return path, false, nil
	}

	if err := writeFileSnapshot(path, defaultFileSnapshot()); err != nil {
		return "", false, fmt.Errorf("write default config: %w", err)
	}
	return path, true, nil
}

func WorkspaceRoot() string {
	return defaultWorkspaceRoot()
}

func SkillStateDir() string {
	return filepath.Join(stateDir(), "state")
}

func SkillIndexPath(agentID string) string {
	segment := normalizeWorkspaceSegment(agentID)
	return filepath.Join(SkillStateDir(), "skills_index_"+segment+".json")
}

func PairingStorePath(agentID string) string {
	segment := normalizeWorkspaceSegment(agentID)
	return filepath.Join(SkillStateDir(), "pairing_"+segment+".json")
}

func SuggestedAgentWorkspace(agentID string) string {
	return defaultAgentWorkspace(agentID)
}

func UpsertAgent(opts AgentUpsertOptions) (string, error) {
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		return "", fmt.Errorf("agent id is required")
	}
	profileID := strings.TrimSpace(opts.ProfileID)
	if profileID == "" {
		return "", fmt.Errorf("agent profile is required")
	}

	workspace := strings.TrimSpace(opts.Workspace)
	if workspace == "" {
		workspace = SuggestedAgentWorkspace(id)
	}

	timeoutSeconds := opts.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 600
	}

	path := configPath()
	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return "", err
	}

	if _, ok := file.Providers.Profiles[profileID]; !ok {
		return "", fmt.Errorf("profile %q is not defined", profileID)
	}

	updated := false
	for idx := range file.Agents.List {
		if strings.TrimSpace(file.Agents.List[idx].ID) == id {
			file.Agents.List[idx].Profile = profileID
			file.Agents.List[idx].Workspace = workspace
			file.Agents.List[idx].TimeoutSeconds = timeoutSeconds
			updated = true
			break
		}
	}
	if !updated {
		file.Agents.List = append(file.Agents.List, fileAgent{
			ID:             id,
			Profile:        profileID,
			Workspace:      workspace,
			TimeoutSeconds: timeoutSeconds,
		})
	}

	if opts.SetAsDefault || strings.TrimSpace(file.Agents.Default) == "" {
		file.Agents.Default = id
	}
	setDefaultFlag(&file.Agents, file.Agents.Default)

	ensureAllowedRootContainsWorkspace(&file.Runtime, workspace)

	if err := writeFileSnapshot(path, file); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func SetDefaultAgent(agentID string) (string, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", fmt.Errorf("agent id is required")
	}

	path := configPath()
	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return "", err
	}

	found := false
	for _, item := range file.Agents.List {
		if strings.TrimSpace(item.ID) == agentID {
			found = true
			break
		}
	}
	if !found {
		return "", fmt.Errorf("agent %q not found", agentID)
	}

	file.Agents.Default = agentID
	setDefaultFlag(&file.Agents, agentID)

	if err := writeFileSnapshot(path, file); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func GetValueByDotKey(key string) (any, error) {
	path := configPath()
	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return nil, err
	}

	root, err := fileSnapshotToMap(file)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(key) == "" {
		return root, nil
	}

	value, ok, err := getNestedMapValue(root, key)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrConfigKeyNotFound
	}
	return value, nil
}

func SetValueByDotKey(key, rawValue string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("config key is required")
	}

	path := configPath()
	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return "", err
	}

	root, err := fileSnapshotToMap(file)
	if err != nil {
		return "", err
	}

	if err := setNestedMapValue(root, key, coerceDotKeyValue(rawValue)); err != nil {
		return "", err
	}

	updated, err := mapToFileSnapshot(root)
	if err != nil {
		return "", err
	}
	if err := writeFileSnapshot(path, updated); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func fileSnapshotToMap(file fileSnapshot) (map[string]any, error) {
	data, err := json.Marshal(file)
	if err != nil {
		return nil, fmt.Errorf("marshal config snapshot: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("decode config snapshot map: %w", err)
	}
	return root, nil
}

func mapToFileSnapshot(root map[string]any) (fileSnapshot, error) {
	data, err := json.Marshal(root)
	if err != nil {
		return fileSnapshot{}, fmt.Errorf("marshal config map: %w", err)
	}

	var file fileSnapshot
	if err := json.Unmarshal(data, &file); err != nil {
		return fileSnapshot{}, fmt.Errorf("invalid config mutation: %w", err)
	}
	return file, nil
}

func splitDotKeyPath(key string) ([]string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, fmt.Errorf("config key is required")
	}

	parts := strings.Split(key, ".")
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return nil, fmt.Errorf("invalid config key %q", key)
		}
	}
	return parts, nil
}

func getNestedMapValue(root map[string]any, key string) (any, bool, error) {
	parts, err := splitDotKeyPath(key)
	if err != nil {
		return nil, false, err
	}

	current := any(root)
	for _, part := range parts {
		node, ok := current.(map[string]any)
		if !ok {
			return nil, false, nil
		}
		next, ok := node[part]
		if !ok {
			return nil, false, nil
		}
		current = next
	}
	return current, true, nil
}

func setNestedMapValue(root map[string]any, key string, value any) error {
	parts, err := splitDotKeyPath(key)
	if err != nil {
		return err
	}

	current := root
	for _, part := range parts[:len(parts)-1] {
		existing, ok := current[part]
		if !ok {
			child := map[string]any{}
			current[part] = child
			current = child
			continue
		}

		child, ok := existing.(map[string]any)
		if !ok {
			return fmt.Errorf("config key %q has non-object parent at %q", key, part)
		}
		current = child
	}

	current[parts[len(parts)-1]] = value
	return nil
}

func coerceDotKeyValue(rawValue string) any {
	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" {
		return ""
	}

	// Accept JSON literals for objects/arrays/quoted strings.
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") || strings.HasPrefix(trimmed, "\"") {
		var decoded any
		if err := json.Unmarshal([]byte(trimmed), &decoded); err == nil {
			return decoded
		}
	}

	switch strings.ToLower(trimmed) {
	case "true":
		return true
	case "false":
		return false
	}

	if value, err := strconv.Atoi(trimmed); err == nil {
		return value
	}
	if strings.ContainsAny(trimmed, ".eE") {
		if value, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return value
		}
	}
	return rawValue
}

func SetSkillDisabledNames(names []string) (string, error) {
	path := configPath()
	file, err := readOrDefaultFileSnapshot(path)
	if err != nil {
		return "", err
	}

	normalized := make([]string, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		value := strings.ToLower(strings.TrimSpace(name))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	file.Skills.DisabledNames = normalized
	if err := writeFileSnapshot(path, file); err != nil {
		return "", fmt.Errorf("write config: %w", err)
	}
	return path, nil
}

func WriteBootstrapFile(opts BootstrapOptions) (string, error) {
	path := configPath()
	file := defaultFileSnapshot()

	profileID := strings.TrimSpace(opts.DefaultProfileID)
	if profileID == "" {
		profileID = "codex"
	}
	baseProfileID := strings.TrimSpace(opts.BaseProfileID)
	if baseProfileID == "" {
		baseProfileID = profileID
	}
	baseProfile, ok := file.Providers.Profiles[baseProfileID]
	if !ok {
		baseProfile = file.Providers.Profiles["codex"]
	}
	if _, exists := file.Providers.Profiles[profileID]; !exists {
		file.Providers.Profiles[profileID] = baseProfile
	}
	file.Agents.Default = "main"
	mainWorkspace := strings.TrimSpace(opts.MainWorkspace)
	if mainWorkspace == "" {
		mainWorkspace = defaultAgentWorkspace("main")
	}
	if len(file.Agents.List) > 0 {
		file.Agents.List[0].ID = "main"
		file.Agents.List[0].Profile = profileID
		file.Agents.List[0].Default = true
		file.Agents.List[0].Workspace = mainWorkspace
	}
	ensureAllowedRootContainsWorkspace(&file.Runtime, mainWorkspace)
	file.Runtime.DefaultCWD = mainWorkspace

	file.Channels.Telegram.Enabled = opts.TelegramEnabled
	file.Channels.Telegram.Token = strings.TrimSpace(opts.TelegramToken)
	file.Channels.Telegram.BotUsername = normalizeTelegramUsername(opts.TelegramBotUsername)
	file.Channels.Telegram.DefaultAgent = "main"

	file.Channels.Discord.Enabled = opts.DiscordEnabled
	file.Channels.Discord.BotToken = strings.TrimSpace(opts.DiscordBotToken)
	file.Channels.Discord.RequireMention = opts.DiscordRequireMention
	file.Channels.Discord.DefaultAgent = "main"

	dbDriver := strings.ToLower(strings.TrimSpace(opts.DatabaseDriver))
	if dbDriver == "" {
		dbDriver = "postgres"
	}
	dbHost := strings.TrimSpace(opts.DatabaseHost)
	if dbHost == "" {
		dbHost = "127.0.0.1"
	}
	dbPort := opts.DatabasePort
	if dbPort <= 0 {
		dbPort = 5432
	}
	dbName := strings.TrimSpace(opts.DatabaseName)
	if dbName == "" {
		dbName = "synapse_x"
	}
	dbUser := strings.TrimSpace(opts.DatabaseUser)
	if dbUser == "" {
		dbUser = "postgres"
	}
	dbSSLMode := strings.TrimSpace(opts.DatabaseSSLMode)
	if dbSSLMode == "" {
		dbSSLMode = "disable"
	}
	file.Database = fileDatabase{
		Enabled:    opts.DatabaseEnabled,
		Driver:     dbDriver,
		Host:       dbHost,
		Port:       dbPort,
		Name:       dbName,
		User:       dbUser,
		Password:   opts.DatabasePassword,
		SSLMode:    dbSSLMode,
		AutoCreate: opts.DatabaseAutoCreate,
	}

	if err := writeFileSnapshot(path, file); err != nil {
		return "", fmt.Errorf("write bootstrap config: %w", err)
	}
	return path, nil
}

func LoadFromEnv() (Snapshot, error) {
	if err := EnsureStateLayout(); err != nil {
		return Snapshot{}, err
	}
	if err := loadDotEnvIfPresent(configDotEnvPath(configPath())); err != nil {
		return Snapshot{}, err
	}

	cfg := defaultSnapshot()
	if err := applyEnvOverrides(&cfg); err != nil {
		return Snapshot{}, err
	}
	if len(cfg.AllowedRoots) == 0 {
		cfg.AllowedRoots = []string{cfg.DefaultCWD}
	}
	cfg.normalizeChannelInstances()
	cfg.normalizeDatabase()
	cfg.normalizeSkills()
	cfg.normalizeIntentRouter()
	if err := cfg.Validate(); err != nil {
		return Snapshot{}, err
	}
	return cfg, nil
}

func defaultFileSnapshot() fileSnapshot {
	mainWorkspace := defaultAgentWorkspace("main")
	localSmokeWorkspace := defaultAgentWorkspace("local-smoke")
	workspaceRoot := defaultWorkspaceRoot()

	return fileSnapshot{
		Runtime: fileRuntime{
			AllowedRoots:   []string{workspaceRoot},
			DefaultCWD:     mainWorkspace,
			TimeoutSeconds: 600,
		},
		Providers: fileProviders{
			Profiles: map[string]fileProviderProfile{
				"codex": {
					Kind:       "codex-cli",
					Command:    "codex",
					Args:       []string{},
					HealthArgs: []string{"--version"},
				},
				"claude": {
					Kind:       "claude-cli",
					Command:    "claude",
					Args:       []string{},
					HealthArgs: []string{"--version"},
				},
				"local-smoke": {
					Kind:       "generic-cli",
					Command:    "cat",
					Args:       []string{},
					HealthArgs: []string{},
				},
			},
		},
		Agents: fileAgents{
			Default: "main",
			List: []fileAgent{
				{
					ID:             "main",
					Profile:        "codex",
					Workspace:      mainWorkspace,
					TimeoutSeconds: 600,
					Default:        true,
				},
				{
					ID:             "local-smoke",
					Profile:        "local-smoke",
					Workspace:      localSmokeWorkspace,
					TimeoutSeconds: 600,
				},
			},
		},
		Execution: fileExecution{
			Command:    "cat",
			Args:       []string{},
			HealthArgs: []string{},
		},
		Channels: fileChannels{
			Discord: fileDiscordChannel{
				Enabled:           false,
				BotToken:          "",
				APIBaseURL:        "https://discord.com/api/v10",
				GatewayURL:        "wss://gateway.discord.gg/?v=10&encoding=json",
				AllowedChannelIDs: []string{},
				RequireMention:    true,
				DefaultAgent:      "main",
			},
			Telegram: fileTelegramChannel{
				Enabled:                 false,
				Mode:                    "polling",
				Token:                   "",
				BotUsername:             "",
				AllowedChatIDs:          []string{},
				RequireCommandOrMention: true,
				PollingSeconds:          30,
				WebhookURL:              "",
				WebhookPath:             "/webhooks/telegram",
				WebhookSecret:           "",
				DefaultAgent:            "main",
			},
			Feishu: fileFeishuChannel{
				Enabled:           false,
				Mode:              "webhook",
				AppID:             "",
				AppSecret:         "",
				VerificationToken: "",
				EncryptKey:        "",
				DefaultAgent:      "main",
			},
			WeCom: fileWeComChannel{
				Enabled:        false,
				Mode:           "webhook",
				CorpID:         "",
				AgentID:        "",
				Secret:         "",
				Token:          "",
				EncodingAESKey: "",
				DefaultAgent:   "main",
			},
		},
		Gateway: fileGateway{
			ListenAddr: ":8080",
			Health: fileGatewayHealth{
				Enabled: true,
				Path:    "/healthz",
			},
		},
		Database: fileDatabase{
			Enabled:    false,
			Driver:     "postgres",
			Host:       "127.0.0.1",
			Port:       5432,
			Name:       "synapse_x",
			User:       "postgres",
			Password:   "",
			SSLMode:    "disable",
			AutoCreate: true,
		},
		Skills: fileSkills{
			Enabled: true,
			Sources: fileSkillSources{
				UserDir:        defaultSkillsRoot(),
				WorkspaceDir:   ".synapsex/skills",
				BuiltinEnabled: true,
				BuiltinDir:     "internal/skills/builtin",
			},
			DisabledNames: nil,
			Allowlist: fileSkillAllowlist{
				Users:    nil,
				Channels: nil,
			},
			DefaultMode:       "channel_allowlist_dm_pairing",
			PairingTTLSeconds: 604800,
		},
		IntentRouter: fileIntentRouter{
			Mode: "rule_first_llm_fallback",
			LLMFallback: fileIntentLLMFallback{
				Enabled:             true,
				ConfidenceThreshold: 0.72,
			},
		},
	}
}

func writeFileSnapshot(path string, file fileSnapshot) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(file); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	dir := filepath.Dir(path)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create config directory: %w", err)
		}
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0o644); err != nil {
		return err
	}
	return nil
}

func readOrDefaultFileSnapshot(path string) (fileSnapshot, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return defaultFileSnapshot(), nil
		}
		return fileSnapshot{}, fmt.Errorf("open config file: %w", err)
	}
	defer file.Close()

	var raw fileSnapshot
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return fileSnapshot{}, fmt.Errorf("parse config file: %w", err)
	}
	if decoder.More() {
		return fileSnapshot{}, fmt.Errorf("parse config file: trailing data")
	}
	normalizeFileSnapshot(&raw)
	return raw, nil
}

func normalizeFileSnapshot(file *fileSnapshot) {
	if file == nil {
		return
	}

	file.Channels.Telegram.Mode = strings.ToLower(strings.TrimSpace(file.Channels.Telegram.Mode))
	if file.Channels.Telegram.Mode == "" {
		file.Channels.Telegram.Mode = "polling"
	}
	if file.Channels.Telegram.PollingSeconds <= 0 {
		file.Channels.Telegram.PollingSeconds = 30
	}
	file.Channels.Telegram.WebhookPath = normalizeWebhookPath(file.Channels.Telegram.WebhookPath)
	if file.Channels.Telegram.WebhookPath == "" {
		file.Channels.Telegram.WebhookPath = "/webhooks/telegram"
	}
	file.Channels.Telegram.DefaultAgent = strings.TrimSpace(file.Channels.Telegram.DefaultAgent)
	if file.Channels.Telegram.DefaultAgent == "" {
		file.Channels.Telegram.DefaultAgent = "main"
	}
	file.Channels.Telegram.AgentBindings = filterBindingMap(file.Channels.Telegram.AgentBindings)

	file.Channels.Feishu.Mode = strings.ToLower(strings.TrimSpace(file.Channels.Feishu.Mode))
	if file.Channels.Feishu.Mode == "" {
		file.Channels.Feishu.Mode = "webhook"
	}
	file.Channels.Feishu.DefaultAgent = strings.TrimSpace(file.Channels.Feishu.DefaultAgent)
	if file.Channels.Feishu.DefaultAgent == "" {
		file.Channels.Feishu.DefaultAgent = "main"
	}
	file.Channels.Feishu.AgentBindings = filterBindingMap(file.Channels.Feishu.AgentBindings)

	file.Channels.WeCom.Mode = strings.ToLower(strings.TrimSpace(file.Channels.WeCom.Mode))
	if file.Channels.WeCom.Mode == "" {
		file.Channels.WeCom.Mode = "webhook"
	}
	file.Channels.WeCom.DefaultAgent = strings.TrimSpace(file.Channels.WeCom.DefaultAgent)
	if file.Channels.WeCom.DefaultAgent == "" {
		file.Channels.WeCom.DefaultAgent = "main"
	}
	file.Channels.WeCom.AgentBindings = filterBindingMap(file.Channels.WeCom.AgentBindings)

	file.Database.Driver = strings.ToLower(strings.TrimSpace(file.Database.Driver))
	if file.Database.Driver == "" {
		file.Database.Driver = "postgres"
	}
	file.Database.Host = strings.TrimSpace(file.Database.Host)
	if file.Database.Host == "" {
		file.Database.Host = "127.0.0.1"
	}
	if file.Database.Port <= 0 {
		file.Database.Port = 5432
	}
	file.Database.Name = strings.TrimSpace(file.Database.Name)
	if file.Database.Name == "" {
		file.Database.Name = "synapse_x"
	}
	file.Database.User = strings.TrimSpace(file.Database.User)
	if file.Database.User == "" {
		file.Database.User = "postgres"
	}
	file.Database.SSLMode = strings.TrimSpace(file.Database.SSLMode)
	if file.Database.SSLMode == "" {
		file.Database.SSLMode = "disable"
	}

	file.Skills.Sources.UserDir = strings.TrimSpace(file.Skills.Sources.UserDir)
	if file.Skills.Sources.UserDir == "" {
		file.Skills.Sources.UserDir = defaultSkillsRoot()
	}
	file.Skills.Sources.WorkspaceDir = strings.TrimSpace(file.Skills.Sources.WorkspaceDir)
	if file.Skills.Sources.WorkspaceDir == "" {
		file.Skills.Sources.WorkspaceDir = ".synapsex/skills"
	}
	file.Skills.Sources.BuiltinDir = strings.TrimSpace(file.Skills.Sources.BuiltinDir)
	if file.Skills.Sources.BuiltinDir == "" {
		file.Skills.Sources.BuiltinDir = "internal/skills/builtin"
	}
	file.Skills.DisabledNames = filterEmpty(file.Skills.DisabledNames)
	file.Skills.Allowlist.Users = filterEmpty(file.Skills.Allowlist.Users)
	file.Skills.Allowlist.Channels = filterEmpty(file.Skills.Allowlist.Channels)
	file.Skills.DefaultMode = strings.TrimSpace(file.Skills.DefaultMode)
	if file.Skills.DefaultMode == "" {
		file.Skills.DefaultMode = "channel_allowlist_dm_pairing"
	}
	if file.Skills.PairingTTLSeconds <= 0 {
		file.Skills.PairingTTLSeconds = 604800
	}

	file.IntentRouter.Mode = strings.TrimSpace(file.IntentRouter.Mode)
	if file.IntentRouter.Mode == "" {
		file.IntentRouter.Mode = "rule_first_llm_fallback"
	}
	if file.IntentRouter.LLMFallback.ConfidenceThreshold <= 0 || file.IntentRouter.LLMFallback.ConfidenceThreshold > 1 {
		file.IntentRouter.LLMFallback.ConfidenceThreshold = 0.72
	}
}

func setDefaultFlag(agents *fileAgents, defaultID string) {
	defaultID = strings.TrimSpace(defaultID)
	for idx := range agents.List {
		agents.List[idx].Default = strings.TrimSpace(agents.List[idx].ID) == defaultID && defaultID != ""
	}
}

func ensureAllowedRootContainsWorkspace(runtime *fileRuntime, workspace string) {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return
	}
	cleanWorkspace := filepath.Clean(workspace)
	roots := append([]string(nil), runtime.AllowedRoots...)
	roots = append(roots, defaultWorkspaceRoot(), cleanWorkspace)
	runtime.AllowedRoots = dedupePathList(roots)
	if strings.TrimSpace(runtime.DefaultCWD) == "" {
		runtime.DefaultCWD = defaultAgentWorkspace("main")
	}
}

func dedupePathList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = filepath.Clean(value)
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func defaultSnapshot() Snapshot {
	mainWorkspace := defaultAgentWorkspace("main")
	workspaceRoot := defaultWorkspaceRoot()

	return Snapshot{
		AllowedRoots:                  []string{workspaceRoot},
		DefaultCWD:                    mainWorkspace,
		Timeout:                       600 * time.Second,
		ExecCommand:                   "cat",
		DiscordEnabled:                false,
		DiscordAPIBaseURL:             "https://discord.com/api/v10",
		DiscordGatewayURL:             "wss://gateway.discord.gg/?v=10&encoding=json",
		DiscordRequireMention:         true,
		TelegramEnabled:               false,
		TelegramMode:                  "polling",
		TelegramRequireCommandMention: true,
		TelegramPollingTimeout:        30 * time.Second,
		TelegramWebhookPath:           "/webhooks/telegram",
		FeishuEnabled:                 false,
		FeishuMode:                    "webhook",
		WeComEnabled:                  false,
		WeComMode:                     "webhook",
		HealthProbeEnabled:            true,
		HTTPListenAddr:                ":8080",
		HealthProbePath:               "/healthz",
		Database: DatabaseConfig{
			Enabled:    false,
			Driver:     "postgres",
			Host:       "127.0.0.1",
			Port:       5432,
			Name:       "synapse_x",
			User:       "postgres",
			Password:   "",
			SSLMode:    "disable",
			AutoCreate: true,
		},
		Skills: SkillConfig{
			Enabled: true,
			Sources: SkillSources{
				UserDir:        defaultSkillsRoot(),
				WorkspaceDir:   ".synapsex/skills",
				BuiltinEnabled: true,
				BuiltinDir:     "internal/skills/builtin",
			},
			DisabledNames: nil,
			Allowlist: SkillAllowlist{
				Users:    nil,
				Channels: nil,
			},
			DefaultMode: "channel_allowlist_dm_pairing",
			PairingTTL:  7 * 24 * time.Hour,
		},
		IntentRouter: IntentRouterConfig{
			Mode: "rule_first_llm_fallback",
			LLMFallback: LLMFallbackConfig{
				Enabled:             true,
				ConfidenceThreshold: 0.72,
			},
		},
		ProviderProfiles: make(map[string]ProviderProfile),
		Agents:           make(map[string]Agent),
	}
}

func configPath() string {
	if explicit := strings.TrimSpace(os.Getenv("SYNAPSEX_CONFIG")); explicit != "" {
		return explicit
	}
	return filepath.Join(stateDir(), "config.json")
}

func configDotEnvPath(configPath string) string {
	return filepath.Join(filepath.Dir(configPath), ".env")
}

func stateDir() string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Join(".synapsex")
	}
	return filepath.Join(home, ".synapsex")
}

func shouldLoadDotEnv(path string) bool {
	if parseBoolOrDefault(os.Getenv("SYNAPSEX_LOAD_DOTENV"), false) {
		return true
	}
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func applyJSONConfigIfPresent(cfg *Snapshot, path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open config file: %w", err)
	}
	defer file.Close()

	var raw jsonSnapshot
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	if decoder.More() {
		return fmt.Errorf("parse config file: trailing data")
	}

	applyJSONValues(cfg, raw)
	return nil
}

func applyJSONValues(cfg *Snapshot, raw jsonSnapshot) {
	applyLegacyJSONValues(cfg, raw)
	applyStructuredJSONValues(cfg, raw)
}

func applyLegacyJSONValues(cfg *Snapshot, raw jsonSnapshot) {
	if raw.AllowedRoots != nil {
		cfg.AllowedRoots = filterEmpty(raw.AllowedRoots)
	}
	if strings.TrimSpace(raw.DefaultCWD) != "" {
		cfg.DefaultCWD = strings.TrimSpace(raw.DefaultCWD)
	}
	if raw.TimeoutSeconds > 0 {
		cfg.Timeout = time.Duration(raw.TimeoutSeconds) * time.Second
	}
	if strings.TrimSpace(raw.ExecCommand) != "" {
		cfg.ExecCommand = strings.TrimSpace(raw.ExecCommand)
	}
	if raw.ExecArgs != nil {
		cfg.ExecArgs = filterEmpty(raw.ExecArgs)
	}
	if raw.ExecHealthArgs != nil {
		cfg.ExecHealthArgs = filterEmpty(raw.ExecHealthArgs)
	}
	if raw.DiscordEnabled != nil {
		cfg.DiscordEnabled = *raw.DiscordEnabled
	}
	if strings.TrimSpace(raw.DiscordBotToken) != "" {
		cfg.DiscordBotToken = strings.TrimSpace(raw.DiscordBotToken)
	}
	if strings.TrimSpace(raw.DiscordAPIBaseURL) != "" {
		cfg.DiscordAPIBaseURL = strings.TrimRight(strings.TrimSpace(raw.DiscordAPIBaseURL), "/")
	}
	if strings.TrimSpace(raw.DiscordGatewayURL) != "" {
		cfg.DiscordGatewayURL = strings.TrimSpace(raw.DiscordGatewayURL)
	}
	if raw.DiscordAllowedChannelIDs != nil {
		cfg.DiscordAllowedChannelIDs = filterEmpty(raw.DiscordAllowedChannelIDs)
	}
	if raw.DiscordRequireMention != nil {
		cfg.DiscordRequireMention = *raw.DiscordRequireMention
	}
	if raw.TelegramEnabled != nil {
		cfg.TelegramEnabled = *raw.TelegramEnabled
	}
	if strings.TrimSpace(raw.TelegramMode) != "" {
		cfg.TelegramMode = strings.ToLower(strings.TrimSpace(raw.TelegramMode))
	}
	if strings.TrimSpace(raw.TelegramToken) != "" {
		cfg.TelegramToken = strings.TrimSpace(raw.TelegramToken)
	}
	if strings.TrimSpace(raw.TelegramBotUsername) != "" {
		cfg.TelegramBotUsername = normalizeTelegramUsername(raw.TelegramBotUsername)
	}
	if raw.TelegramAllowedChatIDs != nil {
		cfg.TelegramAllowedChatIDs = filterEmpty(raw.TelegramAllowedChatIDs)
	}
	if raw.TelegramRequireCommandMention != nil {
		cfg.TelegramRequireCommandMention = *raw.TelegramRequireCommandMention
	}
	if raw.TelegramPollingSeconds > 0 {
		cfg.TelegramPollingTimeout = time.Duration(raw.TelegramPollingSeconds) * time.Second
	}
	if strings.TrimSpace(raw.TelegramWebhookURL) != "" {
		cfg.TelegramWebhookURL = strings.TrimSpace(raw.TelegramWebhookURL)
	}
	if strings.TrimSpace(raw.TelegramWebhookPath) != "" {
		cfg.TelegramWebhookPath = normalizeWebhookPath(raw.TelegramWebhookPath)
	}
	if strings.TrimSpace(raw.TelegramWebhookSecret) != "" {
		cfg.TelegramWebhookSecret = strings.TrimSpace(raw.TelegramWebhookSecret)
	}
	if raw.HealthProbeEnabled != nil {
		cfg.HealthProbeEnabled = *raw.HealthProbeEnabled
	}
	if strings.TrimSpace(raw.HTTPListenAddr) != "" {
		cfg.HTTPListenAddr = strings.TrimSpace(raw.HTTPListenAddr)
	}
	if strings.TrimSpace(raw.HealthProbePath) != "" {
		cfg.HealthProbePath = normalizeHealthPath(raw.HealthProbePath)
	}
	applyProfileSet(cfg, raw.LegacyProviders)
}

func applyStructuredJSONValues(cfg *Snapshot, raw jsonSnapshot) {
	if raw.Runtime != nil {
		if raw.Runtime.AllowedRoots != nil {
			cfg.AllowedRoots = filterEmpty(raw.Runtime.AllowedRoots)
		}
		if strings.TrimSpace(raw.Runtime.DefaultCWD) != "" {
			cfg.DefaultCWD = strings.TrimSpace(raw.Runtime.DefaultCWD)
		}
		if raw.Runtime.TimeoutSeconds > 0 {
			cfg.Timeout = time.Duration(raw.Runtime.TimeoutSeconds) * time.Second
		}
	}

	if raw.Execution != nil {
		if strings.TrimSpace(raw.Execution.Command) != "" {
			cfg.ExecCommand = strings.TrimSpace(raw.Execution.Command)
		}
		if raw.Execution.Args != nil {
			cfg.ExecArgs = filterEmpty(raw.Execution.Args)
		}
		if raw.Execution.HealthArgs != nil {
			cfg.ExecHealthArgs = filterEmpty(raw.Execution.HealthArgs)
		}
	}

	applyProfileSet(cfg, raw.Providers)
	applyAgents(cfg, raw.Agents)

	if raw.Channels != nil {
		if raw.Channels.Discord != nil {
			discord := raw.Channels.Discord
			if discord.Enabled != nil {
				cfg.DiscordEnabled = *discord.Enabled
			}
			if strings.TrimSpace(discord.BotToken) != "" {
				cfg.DiscordBotToken = strings.TrimSpace(discord.BotToken)
			}
			if strings.TrimSpace(discord.APIBaseURL) != "" {
				cfg.DiscordAPIBaseURL = strings.TrimRight(strings.TrimSpace(discord.APIBaseURL), "/")
			}
			if strings.TrimSpace(discord.GatewayURL) != "" {
				cfg.DiscordGatewayURL = strings.TrimSpace(discord.GatewayURL)
			}
			if discord.AllowedChannelIDs != nil {
				cfg.DiscordAllowedChannelIDs = filterEmpty(discord.AllowedChannelIDs)
			}
			if discord.RequireMention != nil {
				cfg.DiscordRequireMention = *discord.RequireMention
			}
			if strings.TrimSpace(discord.DefaultAgent) != "" {
				cfg.DiscordDefaultAgentID = strings.TrimSpace(discord.DefaultAgent)
			}
			if discord.AgentBindings != nil {
				cfg.DiscordAgentBindings = filterBindingMap(discord.AgentBindings)
			}
			if len(discord.Instances) > 0 {
				cfg.DiscordInstances = make([]DiscordInstance, 0, len(discord.Instances))
				for _, item := range discord.Instances {
					instance := DiscordInstance{
						ID:                strings.TrimSpace(item.ID),
						Enabled:           valueOrDefaultBool(item.Enabled, true),
						BotToken:          strings.TrimSpace(item.BotToken),
						APIBaseURL:        strings.TrimRight(strings.TrimSpace(item.APIBaseURL), "/"),
						GatewayURL:        strings.TrimSpace(item.GatewayURL),
						AllowedChannelIDs: filterEmpty(item.AllowedChannelIDs),
						RequireMention:    valueOrDefaultBool(item.RequireMention, true),
						DefaultAgentID:    strings.TrimSpace(item.DefaultAgent),
						AgentBindings:     filterBindingMap(item.AgentBindings),
					}
					if instance.ID == "" {
						instance.ID = fmt.Sprintf("discord-%d", len(cfg.DiscordInstances)+1)
					}
					if instance.APIBaseURL == "" {
						instance.APIBaseURL = defaultAPIBaseURL()
					}
					if instance.GatewayURL == "" {
						instance.GatewayURL = defaultDiscordGatewayURL()
					}
					cfg.DiscordInstances = append(cfg.DiscordInstances, instance)
				}
			}
		}
		if raw.Channels.Telegram != nil {
			telegram := raw.Channels.Telegram
			if telegram.Enabled != nil {
				cfg.TelegramEnabled = *telegram.Enabled
			}
			if strings.TrimSpace(telegram.Mode) != "" {
				cfg.TelegramMode = strings.ToLower(strings.TrimSpace(telegram.Mode))
			}
			if strings.TrimSpace(telegram.Token) != "" {
				cfg.TelegramToken = strings.TrimSpace(telegram.Token)
			}
			if strings.TrimSpace(telegram.BotUsername) != "" {
				cfg.TelegramBotUsername = normalizeTelegramUsername(telegram.BotUsername)
			}
			if telegram.AllowedChatIDs != nil {
				cfg.TelegramAllowedChatIDs = filterEmpty(telegram.AllowedChatIDs)
			}
			if telegram.RequireCommandOrMention != nil {
				cfg.TelegramRequireCommandMention = *telegram.RequireCommandOrMention
			}
			if telegram.PollingSeconds > 0 {
				cfg.TelegramPollingTimeout = time.Duration(telegram.PollingSeconds) * time.Second
			}
			if strings.TrimSpace(telegram.WebhookURL) != "" {
				cfg.TelegramWebhookURL = strings.TrimSpace(telegram.WebhookURL)
			}
			if strings.TrimSpace(telegram.WebhookPath) != "" {
				cfg.TelegramWebhookPath = normalizeWebhookPath(telegram.WebhookPath)
			}
			if strings.TrimSpace(telegram.WebhookSecret) != "" {
				cfg.TelegramWebhookSecret = strings.TrimSpace(telegram.WebhookSecret)
			}
			if strings.TrimSpace(telegram.DefaultAgent) != "" {
				cfg.TelegramDefaultAgentID = strings.TrimSpace(telegram.DefaultAgent)
			}
			if telegram.AgentBindings != nil {
				cfg.TelegramAgentBindings = filterBindingMap(telegram.AgentBindings)
			}
			if len(telegram.Instances) > 0 {
				cfg.TelegramInstances = make([]TelegramInstance, 0, len(telegram.Instances))
				for _, item := range telegram.Instances {
					timeout := cfg.TelegramPollingTimeout
					if item.PollingSeconds > 0 {
						timeout = time.Duration(item.PollingSeconds) * time.Second
					}
					instance := TelegramInstance{
						ID:                      strings.TrimSpace(item.ID),
						Enabled:                 valueOrDefaultBool(item.Enabled, true),
						Mode:                    strings.ToLower(strings.TrimSpace(item.Mode)),
						Token:                   strings.TrimSpace(item.Token),
						BotUsername:             normalizeTelegramUsername(item.BotUsername),
						AllowedChatIDs:          filterEmpty(item.AllowedChatIDs),
						RequireCommandOrMention: valueOrDefaultBool(item.RequireCommandOrMention, true),
						PollingTimeout:          timeout,
						WebhookURL:              strings.TrimSpace(item.WebhookURL),
						WebhookPath:             normalizeWebhookPath(item.WebhookPath),
						WebhookSecret:           strings.TrimSpace(item.WebhookSecret),
						DefaultAgentID:          strings.TrimSpace(item.DefaultAgent),
						AgentBindings:           filterBindingMap(item.AgentBindings),
					}
					if instance.ID == "" {
						instance.ID = fmt.Sprintf("telegram-%d", len(cfg.TelegramInstances)+1)
					}
					if instance.Mode == "" {
						instance.Mode = "polling"
					}
					cfg.TelegramInstances = append(cfg.TelegramInstances, instance)
				}
			}
		}
		if raw.Channels.Feishu != nil {
			feishu := raw.Channels.Feishu
			if feishu.Enabled != nil {
				cfg.FeishuEnabled = *feishu.Enabled
			}
			if strings.TrimSpace(feishu.Mode) != "" {
				cfg.FeishuMode = strings.ToLower(strings.TrimSpace(feishu.Mode))
			}
			if strings.TrimSpace(feishu.AppID) != "" {
				cfg.FeishuAppID = strings.TrimSpace(feishu.AppID)
			}
			if strings.TrimSpace(feishu.AppSecret) != "" {
				cfg.FeishuAppSecret = strings.TrimSpace(feishu.AppSecret)
			}
			if strings.TrimSpace(feishu.VerificationToken) != "" {
				cfg.FeishuVerificationToken = strings.TrimSpace(feishu.VerificationToken)
			}
			if strings.TrimSpace(feishu.EncryptKey) != "" {
				cfg.FeishuEncryptKey = strings.TrimSpace(feishu.EncryptKey)
			}
			if strings.TrimSpace(feishu.DefaultAgent) != "" {
				cfg.FeishuDefaultAgentID = strings.TrimSpace(feishu.DefaultAgent)
			}
			if feishu.AgentBindings != nil {
				cfg.FeishuAgentBindings = filterBindingMap(feishu.AgentBindings)
			}
			if len(feishu.Instances) > 0 {
				cfg.FeishuInstances = make([]FeishuInstance, 0, len(feishu.Instances))
				for _, item := range feishu.Instances {
					instance := FeishuInstance{
						ID:                strings.TrimSpace(item.ID),
						Enabled:           valueOrDefaultBool(item.Enabled, true),
						Mode:              strings.ToLower(strings.TrimSpace(item.Mode)),
						AppID:             strings.TrimSpace(item.AppID),
						AppSecret:         strings.TrimSpace(item.AppSecret),
						VerificationToken: strings.TrimSpace(item.VerificationToken),
						EncryptKey:        strings.TrimSpace(item.EncryptKey),
						DefaultAgentID:    strings.TrimSpace(item.DefaultAgent),
						AgentBindings:     filterBindingMap(item.AgentBindings),
					}
					if instance.ID == "" {
						instance.ID = fmt.Sprintf("feishu-%d", len(cfg.FeishuInstances)+1)
					}
					if instance.Mode == "" {
						instance.Mode = "webhook"
					}
					cfg.FeishuInstances = append(cfg.FeishuInstances, instance)
				}
			}
		}
		if raw.Channels.WeCom != nil {
			wecom := raw.Channels.WeCom
			if wecom.Enabled != nil {
				cfg.WeComEnabled = *wecom.Enabled
			}
			if strings.TrimSpace(wecom.Mode) != "" {
				cfg.WeComMode = strings.ToLower(strings.TrimSpace(wecom.Mode))
			}
			if strings.TrimSpace(wecom.CorpID) != "" {
				cfg.WeComCorpID = strings.TrimSpace(wecom.CorpID)
			}
			if strings.TrimSpace(wecom.AgentID) != "" {
				cfg.WeComAgentID = strings.TrimSpace(wecom.AgentID)
			}
			if strings.TrimSpace(wecom.Secret) != "" {
				cfg.WeComSecret = strings.TrimSpace(wecom.Secret)
			}
			if strings.TrimSpace(wecom.Token) != "" {
				cfg.WeComToken = strings.TrimSpace(wecom.Token)
			}
			if strings.TrimSpace(wecom.EncodingAESKey) != "" {
				cfg.WeComEncodingAESKey = strings.TrimSpace(wecom.EncodingAESKey)
			}
			if strings.TrimSpace(wecom.DefaultAgent) != "" {
				cfg.WeComDefaultAgentID = strings.TrimSpace(wecom.DefaultAgent)
			}
			if wecom.AgentBindings != nil {
				cfg.WeComAgentBindings = filterBindingMap(wecom.AgentBindings)
			}
			if len(wecom.Instances) > 0 {
				cfg.WeComInstances = make([]WeComInstance, 0, len(wecom.Instances))
				for _, item := range wecom.Instances {
					instance := WeComInstance{
						ID:             strings.TrimSpace(item.ID),
						Enabled:        valueOrDefaultBool(item.Enabled, true),
						Mode:           strings.ToLower(strings.TrimSpace(item.Mode)),
						CorpID:         strings.TrimSpace(item.CorpID),
						AgentID:        strings.TrimSpace(item.AgentID),
						Secret:         strings.TrimSpace(item.Secret),
						Token:          strings.TrimSpace(item.Token),
						EncodingAESKey: strings.TrimSpace(item.EncodingAESKey),
						DefaultAgentID: strings.TrimSpace(item.DefaultAgent),
						AgentBindings:  filterBindingMap(item.AgentBindings),
					}
					if instance.ID == "" {
						instance.ID = fmt.Sprintf("wecom-%d", len(cfg.WeComInstances)+1)
					}
					if instance.Mode == "" {
						instance.Mode = "webhook"
					}
					cfg.WeComInstances = append(cfg.WeComInstances, instance)
				}
			}
		}
	}

	if raw.Gateway != nil {
		if strings.TrimSpace(raw.Gateway.ListenAddr) != "" {
			cfg.HTTPListenAddr = strings.TrimSpace(raw.Gateway.ListenAddr)
		}
		if raw.Gateway.Health != nil {
			if raw.Gateway.Health.Enabled != nil {
				cfg.HealthProbeEnabled = *raw.Gateway.Health.Enabled
			}
			if strings.TrimSpace(raw.Gateway.Health.Path) != "" {
				cfg.HealthProbePath = normalizeHealthPath(raw.Gateway.Health.Path)
			}
		}
	}

	if raw.Database != nil {
		db := raw.Database
		if db.Enabled != nil {
			cfg.Database.Enabled = *db.Enabled
		}
		if strings.TrimSpace(db.Driver) != "" {
			cfg.Database.Driver = strings.ToLower(strings.TrimSpace(db.Driver))
		}
		if strings.TrimSpace(db.Host) != "" {
			cfg.Database.Host = strings.TrimSpace(db.Host)
		}
		if db.Port > 0 {
			cfg.Database.Port = db.Port
		}
		if strings.TrimSpace(db.Name) != "" {
			cfg.Database.Name = strings.TrimSpace(db.Name)
		}
		if strings.TrimSpace(db.User) != "" {
			cfg.Database.User = strings.TrimSpace(db.User)
		}
		if db.Password != "" {
			cfg.Database.Password = db.Password
		}
		if strings.TrimSpace(db.SSLMode) != "" {
			cfg.Database.SSLMode = strings.TrimSpace(db.SSLMode)
		}
		if db.AutoCreate != nil {
			cfg.Database.AutoCreate = *db.AutoCreate
		}
	}

	if raw.Skills != nil {
		if raw.Skills.Enabled != nil {
			cfg.Skills.Enabled = *raw.Skills.Enabled
		}
		if raw.Skills.Sources != nil {
			if strings.TrimSpace(raw.Skills.Sources.UserDir) != "" {
				cfg.Skills.Sources.UserDir = strings.TrimSpace(raw.Skills.Sources.UserDir)
			}
			if strings.TrimSpace(raw.Skills.Sources.WorkspaceDir) != "" {
				cfg.Skills.Sources.WorkspaceDir = strings.TrimSpace(raw.Skills.Sources.WorkspaceDir)
			}
			if raw.Skills.Sources.BuiltinEnabled != nil {
				cfg.Skills.Sources.BuiltinEnabled = *raw.Skills.Sources.BuiltinEnabled
			}
			if strings.TrimSpace(raw.Skills.Sources.BuiltinDir) != "" {
				cfg.Skills.Sources.BuiltinDir = strings.TrimSpace(raw.Skills.Sources.BuiltinDir)
			}
		}
		if raw.Skills.DisabledNames != nil {
			cfg.Skills.DisabledNames = filterEmpty(raw.Skills.DisabledNames)
		}
		if raw.Skills.Allowlist != nil {
			if raw.Skills.Allowlist.Users != nil {
				cfg.Skills.Allowlist.Users = filterEmpty(raw.Skills.Allowlist.Users)
			}
			if raw.Skills.Allowlist.Channels != nil {
				cfg.Skills.Allowlist.Channels = filterEmpty(raw.Skills.Allowlist.Channels)
			}
		}
		if strings.TrimSpace(raw.Skills.DefaultMode) != "" {
			cfg.Skills.DefaultMode = strings.TrimSpace(raw.Skills.DefaultMode)
		}
		if raw.Skills.PairingTTLSeconds > 0 {
			cfg.Skills.PairingTTL = time.Duration(raw.Skills.PairingTTLSeconds) * time.Second
		}
	}

	if raw.IntentRouter != nil {
		if strings.TrimSpace(raw.IntentRouter.Mode) != "" {
			cfg.IntentRouter.Mode = strings.TrimSpace(raw.IntentRouter.Mode)
		}
		if raw.IntentRouter.LLMFallback != nil {
			if raw.IntentRouter.LLMFallback.Enabled != nil {
				cfg.IntentRouter.LLMFallback.Enabled = *raw.IntentRouter.LLMFallback.Enabled
			}
			if raw.IntentRouter.LLMFallback.ConfidenceThreshold > 0 {
				cfg.IntentRouter.LLMFallback.ConfidenceThreshold = raw.IntentRouter.LLMFallback.ConfidenceThreshold
			}
		}
	}
}

func applyProfileSet(cfg *Snapshot, section interface{}) {
	var profiles map[string]jsonProviderProfile
	switch value := section.(type) {
	case *jsonProviders:
		if value != nil {
			profiles = value.Profiles
		}
	case *jsonLegacyProviderSection:
		if value != nil {
			profiles = value.Profiles
		}
	}
	if len(profiles) == 0 {
		return
	}
	for id, profile := range profiles {
		trimmedID := strings.TrimSpace(id)
		if trimmedID == "" {
			continue
		}
		cfg.ProviderProfiles[trimmedID] = ProviderProfile{
			ID:         trimmedID,
			Kind:       normalizeProfileKind(profile.Kind),
			Command:    strings.TrimSpace(profile.Command),
			Args:       filterEmpty(profile.Args),
			HealthArgs: filterEmpty(profile.HealthArgs),
			Model:      strings.TrimSpace(profile.Model),
		}
	}
}

func applyAgents(cfg *Snapshot, section *jsonAgents) {
	if section == nil {
		return
	}
	if strings.TrimSpace(section.Default) != "" {
		cfg.DefaultAgentID = strings.TrimSpace(section.Default)
	}
	for _, item := range section.List {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		timeout := cfg.Timeout
		if item.TimeoutSeconds > 0 {
			timeout = time.Duration(item.TimeoutSeconds) * time.Second
		}
		agent := Agent{
			ID:        id,
			ProfileID: strings.TrimSpace(item.Profile),
			Workspace: strings.TrimSpace(item.Workspace),
			Timeout:   timeout,
			IsDefault: item.Default,
		}
		cfg.Agents[id] = agent
		if item.Default && cfg.DefaultAgentID == "" {
			cfg.DefaultAgentID = id
		}
	}
}

func (s *Snapshot) resolveActiveAgent() error {
	if len(s.Agents) == 0 || len(s.ProviderProfiles) == 0 {
		return nil
	}

	defaultID := strings.TrimSpace(s.DefaultAgentID)
	if defaultID == "" {
		for _, agent := range s.Agents {
			if agent.IsDefault {
				defaultID = agent.ID
				break
			}
		}
	}
	if defaultID == "" {
		return ErrUnknownAgent
	}

	agent, ok := s.Agents[defaultID]
	if !ok {
		return ErrUnknownAgent
	}
	profile, ok := s.ProviderProfiles[agent.ProfileID]
	if !ok {
		return ErrUnknownAgentProfile
	}
	if strings.TrimSpace(agent.Workspace) != "" {
		s.DefaultCWD = strings.TrimSpace(agent.Workspace)
	}
	if agent.Timeout > 0 {
		s.Timeout = agent.Timeout
	}
	s.DefaultAgentID = agent.ID
	s.ActiveAgent = &agent
	s.ActiveProfile = &profile
	return nil
}

func applyEnvOverrides(cfg *Snapshot) error {
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_ALLOWED_ROOTS")); raw != "" {
		cfg.AllowedRoots = splitList(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DEFAULT_CWD")); raw != "" {
		cfg.DefaultCWD = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TIMEOUT_SECONDS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse SYNAPSEX_TIMEOUT_SECONDS: %w", err)
		}
		cfg.Timeout = time.Duration(parsed) * time.Second
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_EXEC_COMMAND")); raw != "" {
		cfg.ExecCommand = raw
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_EXEC_ARGS"); ok {
		cfg.ExecArgs = splitShellWords(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_EXEC_HEALTH_ARGS"); ok {
		cfg.ExecHealthArgs = splitShellWords(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DISCORD_ENABLED"); ok {
		cfg.DiscordEnabled = parseBoolOrDefault(raw, cfg.DiscordEnabled)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DISCORD_BOT_TOKEN")); raw != "" {
		cfg.DiscordBotToken = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DISCORD_API_BASE_URL")); raw != "" {
		cfg.DiscordAPIBaseURL = strings.TrimRight(raw, "/")
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DISCORD_GATEWAY_URL")); raw != "" {
		cfg.DiscordGatewayURL = raw
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DISCORD_ALLOWED_CHANNEL_IDS"); ok {
		cfg.DiscordAllowedChannelIDs = splitList(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DISCORD_REQUIRE_MENTION"); ok {
		cfg.DiscordRequireMention = parseBoolOrDefault(raw, cfg.DiscordRequireMention)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_TELEGRAM_ENABLED"); ok {
		cfg.TelegramEnabled = parseBoolOrDefault(raw, cfg.TelegramEnabled)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_MODE")); raw != "" {
		cfg.TelegramMode = strings.ToLower(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_TOKEN")); raw != "" {
		cfg.TelegramToken = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_BOT_USERNAME")); raw != "" {
		cfg.TelegramBotUsername = normalizeTelegramUsername(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_TELEGRAM_ALLOWED_CHAT_IDS"); ok {
		cfg.TelegramAllowedChatIDs = splitList(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_TELEGRAM_REQUIRE_COMMAND_OR_MENTION"); ok {
		cfg.TelegramRequireCommandMention = parseBoolOrDefault(raw, cfg.TelegramRequireCommandMention)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_POLLING_SECONDS")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse SYNAPSEX_TELEGRAM_POLLING_SECONDS: %w", err)
		}
		cfg.TelegramPollingTimeout = time.Duration(parsed) * time.Second
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_WEBHOOK_URL")); raw != "" {
		cfg.TelegramWebhookURL = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_WEBHOOK_PATH")); raw != "" {
		cfg.TelegramWebhookPath = normalizeWebhookPath(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_TELEGRAM_WEBHOOK_SECRET")); raw != "" {
		cfg.TelegramWebhookSecret = raw
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_HEALTH_PROBE_ENABLED"); ok {
		cfg.HealthProbeEnabled = parseBoolOrDefault(raw, cfg.HealthProbeEnabled)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_HTTP_LISTEN_ADDR")); raw != "" {
		cfg.HTTPListenAddr = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_HEALTH_PATH")); raw != "" {
		cfg.HealthProbePath = normalizeHealthPath(raw)
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DATABASE_ENABLED"); ok {
		cfg.Database.Enabled = parseBoolOrDefault(raw, cfg.Database.Enabled)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_DRIVER")); raw != "" {
		cfg.Database.Driver = strings.ToLower(raw)
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_HOST")); raw != "" {
		cfg.Database.Host = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_PORT")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return fmt.Errorf("parse SYNAPSEX_DATABASE_PORT: %w", err)
		}
		cfg.Database.Port = parsed
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_NAME")); raw != "" {
		cfg.Database.Name = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_USER")); raw != "" {
		cfg.Database.User = raw
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DATABASE_PASSWORD"); ok {
		cfg.Database.Password = raw
	}
	if raw := strings.TrimSpace(os.Getenv("SYNAPSEX_DATABASE_SSLMODE")); raw != "" {
		cfg.Database.SSLMode = raw
	}
	if raw, ok := os.LookupEnv("SYNAPSEX_DATABASE_AUTO_CREATE"); ok {
		cfg.Database.AutoCreate = parseBoolOrDefault(raw, cfg.Database.AutoCreate)
	}
	return nil
}

func (s *Snapshot) normalizeChannelInstances() {
	s.DiscordDefaultAgentID = strings.TrimSpace(s.DiscordDefaultAgentID)
	s.DiscordAgentBindings = filterBindingMap(s.DiscordAgentBindings)
	s.TelegramDefaultAgentID = strings.TrimSpace(s.TelegramDefaultAgentID)
	s.TelegramAgentBindings = filterBindingMap(s.TelegramAgentBindings)
	s.FeishuDefaultAgentID = strings.TrimSpace(s.FeishuDefaultAgentID)
	s.FeishuAgentBindings = filterBindingMap(s.FeishuAgentBindings)
	s.WeComDefaultAgentID = strings.TrimSpace(s.WeComDefaultAgentID)
	s.WeComAgentBindings = filterBindingMap(s.WeComAgentBindings)

	if len(s.DiscordInstances) == 0 && hasAnyDiscordLegacyConfig(*s) {
		s.DiscordInstances = []DiscordInstance{{
			ID:                "discord-default",
			Enabled:           s.DiscordEnabled,
			BotToken:          strings.TrimSpace(s.DiscordBotToken),
			APIBaseURL:        strings.TrimRight(strings.TrimSpace(s.DiscordAPIBaseURL), "/"),
			GatewayURL:        strings.TrimSpace(s.DiscordGatewayURL),
			AllowedChannelIDs: filterEmpty(s.DiscordAllowedChannelIDs),
			RequireMention:    s.DiscordRequireMention,
			DefaultAgentID:    strings.TrimSpace(s.DiscordDefaultAgentID),
			AgentBindings:     filterBindingMap(s.DiscordAgentBindings),
		}}
	}
	if len(s.TelegramInstances) == 0 && hasAnyTelegramLegacyConfig(*s) {
		s.TelegramInstances = []TelegramInstance{{
			ID:                      "telegram-default",
			Enabled:                 s.TelegramEnabled,
			Mode:                    strings.ToLower(strings.TrimSpace(s.TelegramMode)),
			Token:                   strings.TrimSpace(s.TelegramToken),
			BotUsername:             normalizeTelegramUsername(s.TelegramBotUsername),
			AllowedChatIDs:          filterEmpty(s.TelegramAllowedChatIDs),
			RequireCommandOrMention: s.TelegramRequireCommandMention,
			PollingTimeout:          s.TelegramPollingTimeout,
			WebhookURL:              strings.TrimSpace(s.TelegramWebhookURL),
			WebhookPath:             normalizeWebhookPath(s.TelegramWebhookPath),
			WebhookSecret:           strings.TrimSpace(s.TelegramWebhookSecret),
			DefaultAgentID:          strings.TrimSpace(s.TelegramDefaultAgentID),
			AgentBindings:           filterBindingMap(s.TelegramAgentBindings),
		}}
	}
	if len(s.FeishuInstances) == 0 && hasAnyFeishuLegacyConfig(*s) {
		s.FeishuInstances = []FeishuInstance{{
			ID:                "feishu-default",
			Enabled:           s.FeishuEnabled,
			Mode:              strings.ToLower(strings.TrimSpace(s.FeishuMode)),
			AppID:             strings.TrimSpace(s.FeishuAppID),
			AppSecret:         strings.TrimSpace(s.FeishuAppSecret),
			VerificationToken: strings.TrimSpace(s.FeishuVerificationToken),
			EncryptKey:        strings.TrimSpace(s.FeishuEncryptKey),
			DefaultAgentID:    strings.TrimSpace(s.FeishuDefaultAgentID),
			AgentBindings:     filterBindingMap(s.FeishuAgentBindings),
		}}
	}
	if len(s.WeComInstances) == 0 && hasAnyWeComLegacyConfig(*s) {
		s.WeComInstances = []WeComInstance{{
			ID:             "wecom-default",
			Enabled:        s.WeComEnabled,
			Mode:           strings.ToLower(strings.TrimSpace(s.WeComMode)),
			CorpID:         strings.TrimSpace(s.WeComCorpID),
			AgentID:        strings.TrimSpace(s.WeComAgentID),
			Secret:         strings.TrimSpace(s.WeComSecret),
			Token:          strings.TrimSpace(s.WeComToken),
			EncodingAESKey: strings.TrimSpace(s.WeComEncodingAESKey),
			DefaultAgentID: strings.TrimSpace(s.WeComDefaultAgentID),
			AgentBindings:  filterBindingMap(s.WeComAgentBindings),
		}}
	}

	s.DiscordInstances = normalizeDiscordInstances(s.DiscordInstances, s.DiscordDefaultAgentID)
	s.TelegramInstances = normalizeTelegramInstances(s.TelegramInstances, s.TelegramDefaultAgentID, s.TelegramPollingTimeout, s.TelegramWebhookPath)
	s.FeishuInstances = normalizeFeishuInstances(s.FeishuInstances, s.FeishuDefaultAgentID)
	s.WeComInstances = normalizeWeComInstances(s.WeComInstances, s.WeComDefaultAgentID)

	s.DiscordEnabled = hasEnabledDiscordInstance(s.DiscordInstances)
	s.TelegramEnabled = hasEnabledTelegramInstance(s.TelegramInstances)
	s.FeishuEnabled = hasEnabledFeishuInstance(s.FeishuInstances)
	s.WeComEnabled = hasEnabledWeComInstance(s.WeComInstances)

	if first, ok := firstDiscordInstance(s.DiscordInstances); ok {
		s.DiscordBotToken = strings.TrimSpace(first.BotToken)
		s.DiscordAPIBaseURL = strings.TrimRight(strings.TrimSpace(first.APIBaseURL), "/")
		s.DiscordGatewayURL = strings.TrimSpace(first.GatewayURL)
		s.DiscordAllowedChannelIDs = filterEmpty(first.AllowedChannelIDs)
		s.DiscordRequireMention = first.RequireMention
	}
	if first, ok := firstTelegramInstance(s.TelegramInstances); ok {
		s.TelegramMode = strings.ToLower(strings.TrimSpace(first.Mode))
		s.TelegramToken = strings.TrimSpace(first.Token)
		s.TelegramBotUsername = normalizeTelegramUsername(first.BotUsername)
		s.TelegramAllowedChatIDs = filterEmpty(first.AllowedChatIDs)
		s.TelegramRequireCommandMention = first.RequireCommandOrMention
		s.TelegramPollingTimeout = first.PollingTimeout
		s.TelegramWebhookURL = strings.TrimSpace(first.WebhookURL)
		s.TelegramWebhookPath = normalizeWebhookPath(first.WebhookPath)
		s.TelegramWebhookSecret = strings.TrimSpace(first.WebhookSecret)
	}
	if first, ok := firstFeishuInstance(s.FeishuInstances); ok {
		s.FeishuMode = strings.ToLower(strings.TrimSpace(first.Mode))
		s.FeishuAppID = strings.TrimSpace(first.AppID)
		s.FeishuAppSecret = strings.TrimSpace(first.AppSecret)
		s.FeishuVerificationToken = strings.TrimSpace(first.VerificationToken)
		s.FeishuEncryptKey = strings.TrimSpace(first.EncryptKey)
	}
	if first, ok := firstWeComInstance(s.WeComInstances); ok {
		s.WeComMode = strings.ToLower(strings.TrimSpace(first.Mode))
		s.WeComCorpID = strings.TrimSpace(first.CorpID)
		s.WeComAgentID = strings.TrimSpace(first.AgentID)
		s.WeComSecret = strings.TrimSpace(first.Secret)
		s.WeComToken = strings.TrimSpace(first.Token)
		s.WeComEncodingAESKey = strings.TrimSpace(first.EncodingAESKey)
	}
}

func (s *Snapshot) normalizeDatabase() {
	s.Database.Driver = strings.ToLower(strings.TrimSpace(s.Database.Driver))
	if s.Database.Driver == "" {
		s.Database.Driver = "postgres"
	}

	s.Database.Host = strings.TrimSpace(s.Database.Host)
	if s.Database.Host == "" {
		s.Database.Host = "127.0.0.1"
	}

	if s.Database.Port <= 0 {
		s.Database.Port = 5432
	}

	s.Database.Name = strings.TrimSpace(s.Database.Name)
	if s.Database.Name == "" {
		s.Database.Name = "synapse_x"
	}

	s.Database.User = strings.TrimSpace(s.Database.User)
	if s.Database.User == "" {
		s.Database.User = "postgres"
	}

	s.Database.SSLMode = strings.TrimSpace(s.Database.SSLMode)
	if s.Database.SSLMode == "" {
		s.Database.SSLMode = "disable"
	}
}

func (s *Snapshot) normalizeSkills() {
	s.Skills.Sources.UserDir = strings.TrimSpace(s.Skills.Sources.UserDir)
	if s.Skills.Sources.UserDir == "" {
		s.Skills.Sources.UserDir = defaultSkillsRoot()
	}
	s.Skills.Sources.WorkspaceDir = strings.TrimSpace(s.Skills.Sources.WorkspaceDir)
	if s.Skills.Sources.WorkspaceDir == "" {
		s.Skills.Sources.WorkspaceDir = ".synapsex/skills"
	}
	s.Skills.Sources.BuiltinDir = strings.TrimSpace(s.Skills.Sources.BuiltinDir)
	if s.Skills.Sources.BuiltinDir == "" {
		s.Skills.Sources.BuiltinDir = "internal/skills/builtin"
	}
	s.Skills.DisabledNames = filterEmpty(s.Skills.DisabledNames)
	s.Skills.Allowlist.Users = filterEmpty(s.Skills.Allowlist.Users)
	s.Skills.Allowlist.Channels = filterEmpty(s.Skills.Allowlist.Channels)
	s.Skills.DefaultMode = strings.TrimSpace(strings.ToLower(s.Skills.DefaultMode))
	if s.Skills.DefaultMode == "" {
		s.Skills.DefaultMode = "channel_allowlist_dm_pairing"
	}
	if s.Skills.PairingTTL <= 0 {
		s.Skills.PairingTTL = 7 * 24 * time.Hour
	}
}

func (s *Snapshot) normalizeIntentRouter() {
	s.IntentRouter.Mode = strings.TrimSpace(strings.ToLower(s.IntentRouter.Mode))
	if s.IntentRouter.Mode == "" {
		s.IntentRouter.Mode = "rule_first_llm_fallback"
	}
	if s.IntentRouter.LLMFallback.ConfidenceThreshold <= 0 || s.IntentRouter.LLMFallback.ConfidenceThreshold > 1 {
		s.IntentRouter.LLMFallback.ConfidenceThreshold = 0.72
	}
}

func normalizeDiscordInstances(values []DiscordInstance, fallbackAgent string) []DiscordInstance {
	if len(values) == 0 {
		return nil
	}
	result := make([]DiscordInstance, 0, len(values))
	seen := make(map[string]int, len(values))

	for idx, raw := range values {
		item := raw
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("discord-%d", idx+1)
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}

		item.ID = id
		item.BotToken = strings.TrimSpace(item.BotToken)
		item.APIBaseURL = strings.TrimRight(strings.TrimSpace(item.APIBaseURL), "/")
		item.GatewayURL = strings.TrimSpace(item.GatewayURL)
		item.AllowedChannelIDs = filterEmpty(item.AllowedChannelIDs)
		item.DefaultAgentID = strings.TrimSpace(item.DefaultAgentID)
		item.AgentBindings = filterBindingMap(item.AgentBindings)
		if item.DefaultAgentID == "" {
			item.DefaultAgentID = strings.TrimSpace(fallbackAgent)
		}
		if item.APIBaseURL == "" {
			item.APIBaseURL = defaultAPIBaseURL()
		}
		if item.GatewayURL == "" {
			item.GatewayURL = defaultDiscordGatewayURL()
		}
		result = append(result, item)
	}

	return result
}

func normalizeTelegramInstances(values []TelegramInstance, fallbackAgent string, fallbackPollTimeout time.Duration, fallbackWebhookPath string) []TelegramInstance {
	if len(values) == 0 {
		return nil
	}
	result := make([]TelegramInstance, 0, len(values))
	seen := make(map[string]int, len(values))

	for idx, raw := range values {
		item := raw
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("telegram-%d", idx+1)
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}

		item.ID = id
		item.Mode = strings.ToLower(strings.TrimSpace(item.Mode))
		item.Token = strings.TrimSpace(item.Token)
		item.BotUsername = normalizeTelegramUsername(item.BotUsername)
		item.AllowedChatIDs = filterEmpty(item.AllowedChatIDs)
		item.WebhookURL = strings.TrimSpace(item.WebhookURL)
		item.WebhookPath = normalizeWebhookPath(item.WebhookPath)
		item.WebhookSecret = strings.TrimSpace(item.WebhookSecret)
		item.DefaultAgentID = strings.TrimSpace(item.DefaultAgentID)
		item.AgentBindings = filterBindingMap(item.AgentBindings)
		if item.DefaultAgentID == "" {
			item.DefaultAgentID = strings.TrimSpace(fallbackAgent)
		}
		if item.Mode == "" {
			item.Mode = "polling"
		}
		if item.WebhookPath == "" {
			defaultPath := normalizeWebhookPath(fallbackWebhookPath)
			if defaultPath == "" {
				defaultPath = "/webhooks/telegram"
			}
			if len(values) > 1 {
				defaultPath = strings.TrimRight(defaultPath, "/") + "/" + id
			}
			item.WebhookPath = defaultPath
		}
		if item.PollingTimeout <= 0 {
			item.PollingTimeout = fallbackPollTimeout
		}
		if item.PollingTimeout <= 0 {
			item.PollingTimeout = 30 * time.Second
		}
		result = append(result, item)
	}

	return result
}

func normalizeFeishuInstances(values []FeishuInstance, fallbackAgent string) []FeishuInstance {
	if len(values) == 0 {
		return nil
	}
	result := make([]FeishuInstance, 0, len(values))
	seen := make(map[string]int, len(values))

	for idx, raw := range values {
		item := raw
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("feishu-%d", idx+1)
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}

		item.ID = id
		item.Mode = strings.ToLower(strings.TrimSpace(item.Mode))
		item.AppID = strings.TrimSpace(item.AppID)
		item.AppSecret = strings.TrimSpace(item.AppSecret)
		item.VerificationToken = strings.TrimSpace(item.VerificationToken)
		item.EncryptKey = strings.TrimSpace(item.EncryptKey)
		item.DefaultAgentID = strings.TrimSpace(item.DefaultAgentID)
		item.AgentBindings = filterBindingMap(item.AgentBindings)
		if item.DefaultAgentID == "" {
			item.DefaultAgentID = strings.TrimSpace(fallbackAgent)
		}
		if item.Mode == "" {
			item.Mode = "webhook"
		}
		result = append(result, item)
	}

	return result
}

func normalizeWeComInstances(values []WeComInstance, fallbackAgent string) []WeComInstance {
	if len(values) == 0 {
		return nil
	}
	result := make([]WeComInstance, 0, len(values))
	seen := make(map[string]int, len(values))

	for idx, raw := range values {
		item := raw
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = fmt.Sprintf("wecom-%d", idx+1)
		}
		seen[id]++
		if seen[id] > 1 {
			id = fmt.Sprintf("%s-%d", id, seen[id])
		}

		item.ID = id
		item.Mode = strings.ToLower(strings.TrimSpace(item.Mode))
		item.CorpID = strings.TrimSpace(item.CorpID)
		item.AgentID = strings.TrimSpace(item.AgentID)
		item.Secret = strings.TrimSpace(item.Secret)
		item.Token = strings.TrimSpace(item.Token)
		item.EncodingAESKey = strings.TrimSpace(item.EncodingAESKey)
		item.DefaultAgentID = strings.TrimSpace(item.DefaultAgentID)
		item.AgentBindings = filterBindingMap(item.AgentBindings)
		if item.DefaultAgentID == "" {
			item.DefaultAgentID = strings.TrimSpace(fallbackAgent)
		}
		if item.Mode == "" {
			item.Mode = "webhook"
		}
		result = append(result, item)
	}

	return result
}

func hasEnabledDiscordInstance(values []DiscordInstance) bool {
	for _, item := range values {
		if item.Enabled {
			return true
		}
	}
	return false
}

func hasEnabledTelegramInstance(values []TelegramInstance) bool {
	for _, item := range values {
		if item.Enabled {
			return true
		}
	}
	return false
}

func hasEnabledFeishuInstance(values []FeishuInstance) bool {
	for _, item := range values {
		if item.Enabled {
			return true
		}
	}
	return false
}

func hasEnabledWeComInstance(values []WeComInstance) bool {
	for _, item := range values {
		if item.Enabled {
			return true
		}
	}
	return false
}

func hasAnyDiscordLegacyConfig(s Snapshot) bool {
	return s.DiscordEnabled ||
		strings.TrimSpace(s.DiscordBotToken) != "" ||
		len(s.DiscordAllowedChannelIDs) > 0 ||
		strings.TrimSpace(s.DiscordDefaultAgentID) != "" ||
		len(s.DiscordAgentBindings) > 0
}

func hasAnyTelegramLegacyConfig(s Snapshot) bool {
	return s.TelegramEnabled ||
		strings.EqualFold(strings.TrimSpace(s.TelegramMode), "webhook") ||
		strings.TrimSpace(s.TelegramToken) != "" ||
		strings.TrimSpace(s.TelegramBotUsername) != "" ||
		len(s.TelegramAllowedChatIDs) > 0 ||
		strings.TrimSpace(s.TelegramWebhookURL) != "" ||
		strings.TrimSpace(s.TelegramWebhookPath) != "" ||
		strings.TrimSpace(s.TelegramWebhookSecret) != "" ||
		strings.TrimSpace(s.TelegramDefaultAgentID) != "" ||
		len(s.TelegramAgentBindings) > 0
}

func hasAnyFeishuLegacyConfig(s Snapshot) bool {
	return s.FeishuEnabled ||
		strings.TrimSpace(s.FeishuAppID) != "" ||
		strings.TrimSpace(s.FeishuAppSecret) != "" ||
		strings.TrimSpace(s.FeishuVerificationToken) != "" ||
		strings.TrimSpace(s.FeishuEncryptKey) != "" ||
		strings.TrimSpace(s.FeishuDefaultAgentID) != "" ||
		len(s.FeishuAgentBindings) > 0
}

func hasAnyWeComLegacyConfig(s Snapshot) bool {
	return s.WeComEnabled ||
		strings.TrimSpace(s.WeComCorpID) != "" ||
		strings.TrimSpace(s.WeComAgentID) != "" ||
		strings.TrimSpace(s.WeComSecret) != "" ||
		strings.TrimSpace(s.WeComToken) != "" ||
		strings.TrimSpace(s.WeComEncodingAESKey) != "" ||
		strings.TrimSpace(s.WeComDefaultAgentID) != "" ||
		len(s.WeComAgentBindings) > 0
}

func firstDiscordInstance(values []DiscordInstance) (DiscordInstance, bool) {
	for _, item := range values {
		if item.Enabled {
			return item, true
		}
	}
	if len(values) == 0 {
		return DiscordInstance{}, false
	}
	return values[0], true
}

func firstTelegramInstance(values []TelegramInstance) (TelegramInstance, bool) {
	for _, item := range values {
		if item.Enabled {
			return item, true
		}
	}
	if len(values) == 0 {
		return TelegramInstance{}, false
	}
	return values[0], true
}

func firstFeishuInstance(values []FeishuInstance) (FeishuInstance, bool) {
	for _, item := range values {
		if item.Enabled {
			return item, true
		}
	}
	if len(values) == 0 {
		return FeishuInstance{}, false
	}
	return values[0], true
}

func firstWeComInstance(values []WeComInstance) (WeComInstance, bool) {
	for _, item := range values {
		if item.Enabled {
			return item, true
		}
	}
	if len(values) == 0 {
		return WeComInstance{}, false
	}
	return values[0], true
}

func (s Snapshot) Validate() error {
	if s.Timeout <= 0 {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.DefaultCWD) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.ExecCommand) == "" && s.ActiveProfile == nil {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.HTTPListenAddr) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.HealthProbePath) == "" {
		return ErrInvalidConfig
	}
	for _, root := range s.AllowedRoots {
		if strings.TrimSpace(root) == "" {
			return ErrInvalidConfig
		}
	}
	for _, profile := range s.ProviderProfiles {
		if strings.TrimSpace(profile.Kind) == "" {
			return ErrInvalidConfig
		}
		if strings.TrimSpace(profile.Command) == "" {
			return ErrInvalidConfig
		}
	}
	for _, agent := range s.Agents {
		if strings.TrimSpace(agent.ID) == "" || strings.TrimSpace(agent.ProfileID) == "" {
			return ErrInvalidConfig
		}
	}
	if strings.TrimSpace(s.DefaultAgentID) != "" && len(s.Agents) > 0 {
		if _, ok := s.Agents[strings.TrimSpace(s.DefaultAgentID)]; !ok {
			return ErrUnknownAgent
		}
	}
	for _, agent := range s.Agents {
		if _, ok := s.ProviderProfiles[agent.ProfileID]; !ok {
			return ErrUnknownAgentProfile
		}
	}

	if err := s.validateChannelAgentReference(s.DiscordDefaultAgentID); err != nil {
		return err
	}
	for _, agentID := range s.DiscordAgentBindings {
		if err := s.validateChannelAgentReference(agentID); err != nil {
			return err
		}
	}
	for _, instance := range s.DiscordInstances {
		if strings.TrimSpace(instance.ID) == "" {
			return ErrInvalidConfig
		}
		if instance.Enabled {
			if strings.TrimSpace(instance.BotToken) == "" {
				return ErrInvalidConfig
			}
			if strings.TrimSpace(instance.APIBaseURL) == "" {
				return ErrInvalidConfig
			}
			if strings.TrimSpace(instance.GatewayURL) == "" {
				return ErrInvalidConfig
			}
		}
		if err := s.validateChannelAgentReference(instance.DefaultAgentID); err != nil {
			return err
		}
		for _, agentID := range instance.AgentBindings {
			if err := s.validateChannelAgentReference(agentID); err != nil {
				return err
			}
		}
	}

	if err := s.validateChannelAgentReference(s.TelegramDefaultAgentID); err != nil {
		return err
	}
	for _, agentID := range s.TelegramAgentBindings {
		if err := s.validateChannelAgentReference(agentID); err != nil {
			return err
		}
	}
	for _, instance := range s.TelegramInstances {
		if strings.TrimSpace(instance.ID) == "" {
			return ErrInvalidConfig
		}
		if instance.Enabled {
			mode := strings.ToLower(strings.TrimSpace(instance.Mode))
			if mode != "polling" && mode != "webhook" {
				return ErrInvalidConfig
			}
			if strings.TrimSpace(instance.Token) == "" {
				return ErrInvalidConfig
			}
			if mode == "polling" && instance.PollingTimeout <= 0 {
				return ErrInvalidConfig
			}
			if mode == "webhook" && (strings.TrimSpace(instance.WebhookURL) == "" || strings.TrimSpace(instance.WebhookPath) == "") {
				return ErrInvalidConfig
			}
		}
		if err := s.validateChannelAgentReference(instance.DefaultAgentID); err != nil {
			return err
		}
		for _, agentID := range instance.AgentBindings {
			if err := s.validateChannelAgentReference(agentID); err != nil {
				return err
			}
		}
	}

	if err := s.validateChannelAgentReference(s.FeishuDefaultAgentID); err != nil {
		return err
	}
	for _, agentID := range s.FeishuAgentBindings {
		if err := s.validateChannelAgentReference(agentID); err != nil {
			return err
		}
	}
	for _, instance := range s.FeishuInstances {
		if strings.TrimSpace(instance.ID) == "" {
			return ErrInvalidConfig
		}
		if instance.Enabled {
			mode := strings.ToLower(strings.TrimSpace(instance.Mode))
			if mode != "webhook" {
				return ErrInvalidConfig
			}
			if strings.TrimSpace(instance.AppID) == "" ||
				strings.TrimSpace(instance.AppSecret) == "" ||
				strings.TrimSpace(instance.VerificationToken) == "" {
				return ErrInvalidConfig
			}
		}
		if err := s.validateChannelAgentReference(instance.DefaultAgentID); err != nil {
			return err
		}
		for _, agentID := range instance.AgentBindings {
			if err := s.validateChannelAgentReference(agentID); err != nil {
				return err
			}
		}
	}

	if err := s.validateChannelAgentReference(s.WeComDefaultAgentID); err != nil {
		return err
	}
	for _, agentID := range s.WeComAgentBindings {
		if err := s.validateChannelAgentReference(agentID); err != nil {
			return err
		}
	}
	for _, instance := range s.WeComInstances {
		if strings.TrimSpace(instance.ID) == "" {
			return ErrInvalidConfig
		}
		if instance.Enabled {
			mode := strings.ToLower(strings.TrimSpace(instance.Mode))
			if mode != "webhook" {
				return ErrInvalidConfig
			}
			if strings.TrimSpace(instance.CorpID) == "" ||
				strings.TrimSpace(instance.AgentID) == "" ||
				strings.TrimSpace(instance.Secret) == "" ||
				strings.TrimSpace(instance.Token) == "" ||
				strings.TrimSpace(instance.EncodingAESKey) == "" {
				return ErrInvalidConfig
			}
		}
		if err := s.validateChannelAgentReference(instance.DefaultAgentID); err != nil {
			return err
		}
		for _, agentID := range instance.AgentBindings {
			if err := s.validateChannelAgentReference(agentID); err != nil {
				return err
			}
		}
	}

	if s.Database.Enabled {
		if strings.TrimSpace(s.Database.Driver) == "" {
			return ErrInvalidConfig
		}
		if strings.ToLower(strings.TrimSpace(s.Database.Driver)) != "postgres" {
			return ErrInvalidConfig
		}
		if strings.TrimSpace(s.Database.Host) == "" {
			return ErrInvalidConfig
		}
		if s.Database.Port <= 0 {
			return ErrInvalidConfig
		}
		if strings.TrimSpace(s.Database.Name) == "" {
			return ErrInvalidConfig
		}
		if strings.TrimSpace(s.Database.User) == "" {
			return ErrInvalidConfig
		}
		if strings.TrimSpace(s.Database.SSLMode) == "" {
			return ErrInvalidConfig
		}
	}
	if strings.TrimSpace(s.Skills.Sources.UserDir) == "" {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.Skills.Sources.WorkspaceDir) == "" {
		return ErrInvalidConfig
	}
	if s.Skills.PairingTTL <= 0 {
		return ErrInvalidConfig
	}
	if strings.TrimSpace(s.IntentRouter.Mode) == "" {
		return ErrInvalidConfig
	}
	if s.IntentRouter.LLMFallback.ConfidenceThreshold <= 0 || s.IntentRouter.LLMFallback.ConfidenceThreshold > 1 {
		return ErrInvalidConfig
	}
	return nil
}

func (s Snapshot) validateChannelAgentReference(agentID string) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" || len(s.Agents) == 0 {
		return nil
	}
	if _, ok := s.Agents[agentID]; !ok {
		return ErrUnknownAgent
	}
	return nil
}

func (s Snapshot) ValidateWorkingDirectory(cwd string) error {
	if strings.TrimSpace(cwd) == "" {
		return ErrForbiddenCWD
	}

	target := filepath.Clean(cwd)
	for _, root := range s.AllowedRoots {
		allowed := filepath.Clean(root)
		relative, err := filepath.Rel(allowed, target)
		if err != nil {
			continue
		}
		if relative == "." || (!strings.HasPrefix(relative, "..") && relative != "..") {
			return nil
		}
	}
	return ErrForbiddenCWD
}

func (s Snapshot) IsChannelEnabled(channel string) bool {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "discord":
		if len(s.DiscordInstances) > 0 {
			return hasEnabledDiscordInstance(s.DiscordInstances)
		}
		return s.DiscordEnabled
	case "telegram":
		if len(s.TelegramInstances) > 0 {
			return hasEnabledTelegramInstance(s.TelegramInstances)
		}
		return s.TelegramEnabled
	case "feishu":
		if len(s.FeishuInstances) > 0 {
			return hasEnabledFeishuInstance(s.FeishuInstances)
		}
		return s.FeishuEnabled
	case "wecom":
		if len(s.WeComInstances) > 0 {
			return hasEnabledWeComInstance(s.WeComInstances)
		}
		return s.WeComEnabled
	default:
		return false
	}
}

func (s Snapshot) SkillUserDir() string {
	return strings.TrimSpace(s.Skills.Sources.UserDir)
}

func (s Snapshot) WorkspaceSkillDir(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return ""
	}
	relative := strings.TrimSpace(s.Skills.Sources.WorkspaceDir)
	if relative == "" {
		relative = ".synapsex/skills"
	}
	if filepath.IsAbs(relative) {
		return filepath.Clean(relative)
	}
	return filepath.Join(workspace, relative)
}

func splitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	items := strings.Split(raw, ",")
	return filterEmpty(items)
}

func filterEmpty(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func filterBindingMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for key, value := range values {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		result[trimmedKey] = trimmedValue
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func valueOrDefaultBool(raw *bool, fallback bool) bool {
	if raw == nil {
		return fallback
	}
	return *raw
}

func splitShellWords(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Fields(raw)
}

func parseBoolOrDefault(raw string, fallback bool) bool {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func defaultWorkspaceRoot() string {
	return filepath.Join(stateDir(), "workspaces")
}

func defaultSkillsRoot() string {
	return filepath.Join(stateDir(), "skills")
}

func defaultAgentWorkspace(agentID string) string {
	return filepath.Join(defaultWorkspaceRoot(), normalizeWorkspaceSegment(agentID))
}

func normalizeWorkspaceSegment(raw string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	if value == "" {
		return "main"
	}

	var b strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}

	segment := strings.Trim(strings.ReplaceAll(b.String(), "--", "-"), "-")
	if segment == "" {
		return "main"
	}
	return segment
}

func defaultAPIBaseURL() string {
	return "https://discord.com/api/v10"
}

func defaultDiscordGatewayURL() string {
	return "wss://gateway.discord.gg/?v=10&encoding=json"
}

func normalizeHealthPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "/healthz"
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}

func normalizeWebhookPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/") {
		return value
	}
	return "/" + value
}

func normalizeTelegramUsername(raw string) string {
	value := strings.TrimSpace(raw)
	return strings.TrimPrefix(value, "@")
}

func normalizeProfileKind(raw string) string {
	kind := strings.ToLower(strings.TrimSpace(raw))
	if kind == "" {
		return "generic-cli"
	}
	return kind
}
