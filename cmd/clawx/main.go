package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"clawx/internal/application/autonomy"
	"clawx/internal/application/command"
	"clawx/internal/application/service"
	"clawx/internal/application/skillorchestrator"
	"clawx/internal/application/skillregistry"
	projectdomain "clawx/internal/domain/project"
	sessiondomain "clawx/internal/domain/session"
	skilldomain "clawx/internal/domain/skill"
	"clawx/internal/infrastructure/backend"
	"clawx/internal/infrastructure/config"
	"clawx/internal/infrastructure/health"
	stdlogging "clawx/internal/infrastructure/logging"
	"clawx/internal/infrastructure/persistence"
	adminiface "clawx/internal/interfaces/admin"
	chatiface "clawx/internal/interfaces/chat"
	discordchat "clawx/internal/interfaces/chat/discord"
	feishuchat "clawx/internal/interfaces/chat/feishu"
	telegramchat "clawx/internal/interfaces/chat/telegram"
	wecomchat "clawx/internal/interfaces/chat/wecom"
)

type menuOption struct {
	key      string
	label    string
	aliases  []string
	selected bool
}

type channelPlan struct {
	enableTelegram bool
	enableDiscord  bool
}

type databasePlan struct {
	enabled    bool
	driver     string
	host       string
	port       int
	name       string
	user       string
	password   string
	sslMode    string
	autoCreate bool
}

type agentRuntime struct {
	agentID         string
	backendName     string
	cwd             string
	cfgSnapshot     config.Snapshot
	projectCWD      executionCWDProjectResolver
	attachmentStore *attachmentContextStore
	profileKind     string
	profileCmd      string
	profileModel    string
	router          *service.Router
	runner          *backend.DirectRunner
	skills          *skillregistry.Service
	skillControl    *skillorchestrator.Runtime
}

type runnerWrapper interface {
	Run(context.Context) error
}

type executionCWDProjectResolver interface {
	GetProject(ctx context.Context, projectID string) (projectdomain.Record, error)
}

type adapterRetryScope struct {
	channel   string
	instance  string
	component string
}

var channelRouteMetrics = service.NewChannelRouteMetrics(2048)
var traceLogger *stdlogging.Logger

var executionEvidenceCommandPattern = regexp.MustCompile(`(?m)(^|\n)\s*(go\s+test|go\s+run|npm\s+run|pnpm\s+run|yarn\s+|pytest|cargo\s+test|make\s+test|bash\s+|sh\s+|uv\s+run|curl\s+|systemctl\s+|pip\s+install|pip3\s+install|python(?:3)?\s+-m\s+pip\s+install)`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("%v", err)
	}
}

func run(args []string) error {
	switch firstArg(args) {
	case "", "serve":
		return runServe()
	case "run":
		return runRun(args[1:])
	case "config":
		return runConfigEntry(args[1:])
	case "skill":
		return runSkillCommand(args[1:])
	case "install-service":
		return runInstallService(args[1:])
	case "setup-service":
		return runSetupService(args[1:])
	case "trace":
		return runTraceCommand(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", firstArg(args))
	}
}

func runRun(args []string) error {
	target := ""
	if len(args) > 0 {
		target = normalizeRunTarget(args[0])
		if target == "" {
			return fmt.Errorf("unknown run target %q; expected serve|telegram|discord|feishu|wecom", strings.TrimSpace(args[0]))
		}
	}

	if err := os.Setenv("CLAWX_RUN_MODE", "true"); err != nil {
		return err
	}
	defer os.Unsetenv("CLAWX_RUN_MODE")

	if target != "" {
		if err := os.Setenv("CLAWX_SERVICE_RUN", target); err != nil {
			return err
		}
		defer os.Unsetenv("CLAWX_SERVICE_RUN")
	}
	return runServe()
}

func runServe() error {
	if err := config.EnsureStateLayout(); err != nil {
		return fmt.Errorf("prepare state layout: %w", err)
	}

	path, exists, err := config.Exists()
	if err != nil {
		return fmt.Errorf("check config: %w", err)
	}
	if !exists {
		log.Printf("config file %q not found; entering configuration bootstrap", path)
		if err := runConfigCommand(); err != nil {
			return err
		}
		log.Printf("configuration bootstrap completed; continuing startup")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	runStartupExecutionSelfCheck(cfg)
	initTraceLogger()
	if updatedCfg, changed, err := autoBootstrapDefaultAgentWorkspace(cfg); err != nil {
		return fmt.Errorf("workspace bootstrap failed: %w", err)
	} else if changed {
		cfg = updatedCfg
		if cfg.ActiveAgent != nil {
			log.Printf("auto workspace bootstrap completed: agent=%s workspace=%s", cfg.ActiveAgent.ID, cfg.ActiveAgent.Workspace)
		} else {
			log.Printf("auto workspace bootstrap completed")
		}
	}
	if err := ensureWorkspacesReady(cfg); err != nil {
		return fmt.Errorf("workspace preflight failed: %w", err)
	}
	if isEnvTrue("CLAWX_RUN_MODE") {
		runTarget := normalizeRunTarget(cfg.Service.Run)
		if runTarget == "" {
			runTarget = "serve"
		}
		applyRunTargetOverrides(&cfg, runTarget)
		log.Printf("run target applied: %s", runTarget)
	}

	store, closeStore, err := newSessionStore(cfg)
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}
	defer closeStore()

	sessionManager := service.NewSessionManager(store, store, nil)
	runtimes, defaultRuntimeID, scheduleRunner, err := buildAgentRuntimes(cfg, sessionManager)
	if err != nil {
		return fmt.Errorf("build runtimes: %w", err)
	}
	defaultRuntime := selectRuntime(runtimes, defaultRuntimeID, "")
	if defaultRuntime.runner == nil {
		return fmt.Errorf("no runtime available")
	}
	agentOverrides := newConversationAgentOverrides()
	if err := agentOverrides.EnablePersistence(filepath.Join(config.StateDir(), "agent_overrides.json")); err != nil {
		log.Printf("load agent overrides failed: %v", err)
	}
	formatter := service.NewOutputFormatter()
	streamer := service.NewOutputStreamer()
	delivery := service.NewOutputDelivery(formatter, streamer)
	probe := health.NewProbe(defaultRuntime.runner.HealthCheck)
	healthHandler := adminiface.NewHealthHandler(probe)
	httpMux := http.NewServeMux()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	fatalErrCh := make(chan error, 1)
	started := false
	httpRuntimeNeeded := false
	if scheduleRunner != nil {
		go func() {
			if err := scheduleRunner.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				select {
				case fatalErrCh <- fmt.Errorf("scheduler runner: %w", err):
				default:
				}
			}
		}()
	}

	if cfg.HealthProbeEnabled {
		healthHandler.Register(httpMux, cfg.HealthProbePath)
		started = true
		httpRuntimeNeeded = true
	}

	webhookPaths := make(map[string]struct{})
	for _, instance := range cfg.TelegramInstances {
		if !instance.Enabled {
			continue
		}
		started = true

		telegramAdapter, err := telegramchat.NewAdapter(telegramchat.Options{
			Token:                 instance.Token,
			BotUsername:           instance.BotUsername,
			PollTimeout:           instance.PollingTimeout,
			WebhookSecretToken:    instance.WebhookSecret,
			AllowedChatIDs:        instance.AllowedChatIDs,
			RequireCommandMention: instance.RequireCommandOrMention,
		})
		if err != nil {
			return fmt.Errorf("init telegram adapter[%s]: %w", instance.ID, err)
		}

		instanceCopy := instance
		telegramInboundHandler := func(messageCtx context.Context, envelope telegramchat.InboundEnvelope) error {
			scopeKey := routingScopeKey("telegram", instanceCopy.ID, envelope.Message.ConversationID)
			if handled, response, err := handleAgentChatCommand(envelope.Message, scopeKey, agentOverrides, runtimes, defaultRuntimeID); handled {
				if err != nil {
					sendTelegramDirect(messageCtx, telegramAdapter, envelope.Target, chatiface.FormatError(err))
				} else {
					sendTelegramDirect(messageCtx, telegramAdapter, envelope.Target, response)
				}
				return nil
			}

			requestedAgentID := resolveTelegramAgentID(cfg, instanceCopy, envelope)
			if forcedAgentID, ok := agentOverrides.Get(scopeKey); ok {
				requestedAgentID = forcedAgentID
			}
			runtime := selectRuntime(runtimes, defaultRuntimeID, requestedAgentID)
			scopedConversationID := scopeConversationID(envelope.Message.ConversationID, "telegram", instanceCopy.ID, runtime.agentID)
			handleTelegramInbound(messageCtx, runtime, scopeKey, agentOverrides, runtimes, defaultRuntimeID, delivery, telegramAdapter, envelope, scopedConversationID, instanceCopy.ID)
			return nil
		}

		mode := strings.ToLower(strings.TrimSpace(instanceCopy.Mode))
		if mode == "" {
			mode = "polling"
		}
		if mode == "webhook" {
			httpRuntimeNeeded = true
			path := normalizeWebhookRoutePath(instanceCopy.WebhookPath, instanceCopy.WebhookURL, instanceCopy.ID)
			if _, exists := webhookPaths[path]; exists {
				return fmt.Errorf("duplicate telegram webhook path %q", path)
			}
			webhookPaths[path] = struct{}{}

			httpMux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				envelope, ok, err := telegramAdapter.ParseWebhookRequest(r)
				if err != nil {
					status := http.StatusBadRequest
					if errors.Is(err, telegramchat.ErrWebhookUnauthorized) {
						status = http.StatusForbidden
					}
					if errors.Is(err, telegramchat.ErrWebhookReplayRejected) {
						status = http.StatusForbidden
					}
					if errors.Is(err, telegramchat.ErrWebhookMethod) {
						status = http.StatusMethodNotAllowed
					}
					http.Error(w, http.StatusText(status), status)
					log.Printf("telegram webhook rejected: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = io.WriteString(w, `{"ok":true}`)
				if !ok {
					return
				}
				go func() {
					if err := telegramInboundHandler(ctx, envelope); err != nil {
						log.Printf("telegram webhook handler error: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
					}
				}()
			})
			log.Printf("telegram webhook route registered: instance=%s path=%s", instanceCopy.ID, path)

			go func() {
				runAdapterWithRetry(ctx, adapterRetryScope{
					channel:   "telegram",
					instance:  instanceCopy.ID,
					component: "webhook_registrar",
				}, func(listenCtx context.Context) error {
					if err := telegramAdapter.SetWebhook(listenCtx, instanceCopy.WebhookURL); err != nil {
						return err
					}
					log.Printf("telegram webhook configured: instance=%s url=%s path=%s", instanceCopy.ID, instanceCopy.WebhookURL, path)
					<-listenCtx.Done()
					return listenCtx.Err()
				})
			}()
			continue
		}

		go func() {
			runAdapterWithRetry(ctx, adapterRetryScope{
				channel:   "telegram",
				instance:  instanceCopy.ID,
				component: "adapter",
			}, func(listenCtx context.Context) error {
				log.Printf("telegram adapter started: instance=%s mode=%s", instanceCopy.ID, instanceCopy.Mode)
				return telegramAdapter.Listen(listenCtx, telegramInboundHandler)
			})
		}()
	}

	for _, instance := range cfg.FeishuInstances {
		if !instance.Enabled {
			continue
		}
		started = true
		httpRuntimeNeeded = true

		feishuAdapter, err := feishuchat.NewAdapter(feishuchat.Options{
			AppID:             instance.AppID,
			AppSecret:         instance.AppSecret,
			VerificationToken: instance.VerificationToken,
			EncryptKey:        instance.EncryptKey,
		})
		if err != nil {
			return fmt.Errorf("init feishu adapter[%s]: %w", instance.ID, err)
		}

		instanceCopy := instance
		feishuAdapterCopy := feishuAdapter
		path := normalizeFeishuRoutePath(instanceCopy.ID)
		if _, exists := webhookPaths[path]; exists {
			return fmt.Errorf("duplicate feishu webhook path %q", path)
		}
		webhookPaths[path] = struct{}{}

		feishuInboundHandler := func(messageCtx context.Context, envelope feishuchat.InboundEnvelope) error {
			scopeKey := routingScopeKey("feishu", instanceCopy.ID, envelope.Message.ConversationID)
			if handled, response, err := handleAgentChatCommand(envelope.Message, scopeKey, agentOverrides, runtimes, defaultRuntimeID); handled {
				if err != nil {
					sendFeishuDirect(messageCtx, feishuAdapterCopy, envelope.Target, chatiface.FormatError(err))
				} else {
					sendFeishuDirect(messageCtx, feishuAdapterCopy, envelope.Target, response)
				}
				return nil
			}

			requestedAgentID := resolveFeishuAgentID(cfg, instanceCopy, envelope)
			if forcedAgentID, ok := agentOverrides.Get(scopeKey); ok {
				requestedAgentID = forcedAgentID
			}
			runtime := selectRuntime(runtimes, defaultRuntimeID, requestedAgentID)
			scopedConversationID := scopeConversationID(envelope.Message.ConversationID, "feishu", instanceCopy.ID, runtime.agentID)
			handleFeishuInbound(messageCtx, runtime, scopeKey, agentOverrides, runtimes, defaultRuntimeID, delivery, feishuAdapterCopy, envelope, scopedConversationID, instanceCopy.ID)
			return nil
		}

		httpMux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			result, err := feishuAdapterCopy.ParseWebhookRequest(r)
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, feishuchat.ErrWebhookUnauthorized) || errors.Is(err, feishuchat.ErrVerificationTokenFail) || errors.Is(err, feishuchat.ErrWebhookReplayRejected) {
					status = http.StatusForbidden
				}
				if errors.Is(err, feishuchat.ErrWebhookMethod) {
					status = http.StatusMethodNotAllowed
				}
				http.Error(w, http.StatusText(status), status)
				log.Printf("feishu webhook rejected: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if result.IsChallenge {
				_ = json.NewEncoder(w).Encode(map[string]string{"challenge": result.Challenge})
				return
			}
			_, _ = io.WriteString(w, `{"code":0}`)
			if !result.HasMessage {
				return
			}
			go func() {
				if err := feishuInboundHandler(ctx, result.Envelope); err != nil {
					log.Printf("feishu webhook handler error: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
				}
			}()
		})

		log.Printf("feishu webhook route registered: instance=%s path=%s", instanceCopy.ID, path)
	}

	for _, instance := range cfg.WeComInstances {
		if !instance.Enabled {
			continue
		}
		started = true
		httpRuntimeNeeded = true

		wecomAdapter, err := wecomchat.NewAdapter(wecomchat.Options{
			CorpID:         instance.CorpID,
			AgentID:        instance.AgentID,
			Secret:         instance.Secret,
			Token:          instance.Token,
			EncodingAESKey: instance.EncodingAESKey,
		})
		if err != nil {
			return fmt.Errorf("init wecom adapter[%s]: %w", instance.ID, err)
		}

		instanceCopy := instance
		wecomAdapterCopy := wecomAdapter
		path := normalizeWeComRoutePath(instanceCopy.ID)
		if _, exists := webhookPaths[path]; exists {
			return fmt.Errorf("duplicate wecom webhook path %q", path)
		}
		webhookPaths[path] = struct{}{}

		wecomInboundHandler := func(messageCtx context.Context, envelope wecomchat.InboundEnvelope) error {
			scopeKey := routingScopeKey("wecom", instanceCopy.ID, envelope.Message.ConversationID)
			if handled, response, err := handleAgentChatCommand(envelope.Message, scopeKey, agentOverrides, runtimes, defaultRuntimeID); handled {
				if err != nil {
					sendWeComDirect(messageCtx, wecomAdapterCopy, envelope.Target, chatiface.FormatError(err))
				} else {
					sendWeComDirect(messageCtx, wecomAdapterCopy, envelope.Target, response)
				}
				return nil
			}

			requestedAgentID := resolveWeComAgentID(cfg, instanceCopy, envelope)
			if forcedAgentID, ok := agentOverrides.Get(scopeKey); ok {
				requestedAgentID = forcedAgentID
			}
			runtime := selectRuntime(runtimes, defaultRuntimeID, requestedAgentID)
			scopedConversationID := scopeConversationID(envelope.Message.ConversationID, "wecom", instanceCopy.ID, runtime.agentID)
			handleWeComInbound(messageCtx, runtime, scopeKey, agentOverrides, runtimes, defaultRuntimeID, delivery, wecomAdapterCopy, envelope, scopedConversationID, instanceCopy.ID)
			return nil
		}

		httpMux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			result, err := wecomAdapterCopy.ParseWebhookRequest(r)
			if err != nil {
				status := http.StatusBadRequest
				if errors.Is(err, wecomchat.ErrWebhookUnauthorized) || errors.Is(err, wecomchat.ErrWebhookDecryptFailed) || errors.Is(err, wecomchat.ErrWebhookReplayRejected) || errors.Is(err, wecomchat.ErrWebhookCorpIDMismatch) {
					status = http.StatusForbidden
				}
				if errors.Is(err, wecomchat.ErrWebhookMethod) {
					status = http.StatusMethodNotAllowed
				}
				http.Error(w, http.StatusText(status), status)
				log.Printf("wecom webhook rejected: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
				return
			}

			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			if result.IsURLVerification {
				_, _ = io.WriteString(w, result.URLVerification)
				return
			}
			_, _ = io.WriteString(w, "success")
			if !result.HasMessage {
				return
			}
			go func() {
				if err := wecomInboundHandler(ctx, result.Envelope); err != nil {
					log.Printf("wecom webhook handler error: instance=%s path=%s err=%v", instanceCopy.ID, path, err)
				}
			}()
		})

		log.Printf("wecom webhook route registered: instance=%s path=%s", instanceCopy.ID, path)
	}

	var httpServer *http.Server
	if httpRuntimeNeeded {
		httpServer = startHTTPServer(ctx, cfg, httpMux, fatalErrCh)
		defer func() {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = httpServer.Shutdown(shutdownCtx)
		}()
	}

	for _, instance := range cfg.DiscordInstances {
		if !instance.Enabled {
			continue
		}
		started = true

		discordAdapter, err := discordchat.NewAdapter(discordchat.Options{
			BotToken:        instance.BotToken,
			APIBaseURL:      instance.APIBaseURL,
			GatewayURL:      instance.GatewayURL,
			AllowedChannels: instance.AllowedChannelIDs,
			RequireMention:  instance.RequireMention,
		})
		if err != nil {
			return fmt.Errorf("init discord adapter[%s]: %w", instance.ID, err)
		}

		instanceCopy := instance
		go func() {
			runAdapterWithRetry(ctx, adapterRetryScope{
				channel:   "discord",
				instance:  instanceCopy.ID,
				component: "adapter",
			}, func(listenCtx context.Context) error {
				log.Printf("discord gateway adapter started: instance=%s", instanceCopy.ID)
				return discordAdapter.Listen(listenCtx, func(messageCtx context.Context, envelope discordchat.InboundEnvelope) error {
					scopeKey := routingScopeKey("discord", instanceCopy.ID, envelope.Message.ConversationID)
					if handled, response, err := handleAgentChatCommand(envelope.Message, scopeKey, agentOverrides, runtimes, defaultRuntimeID); handled {
						if err != nil {
							sendDiscordDirect(messageCtx, discordAdapter, envelope.Target, chatiface.FormatError(err))
						} else {
							sendDiscordDirect(messageCtx, discordAdapter, envelope.Target, response)
						}
						return nil
					}

					requestedAgentID := resolveDiscordAgentID(cfg, instanceCopy, envelope)
					if forcedAgentID, ok := agentOverrides.Get(scopeKey); ok {
						requestedAgentID = forcedAgentID
					}
					runtime := selectRuntime(runtimes, defaultRuntimeID, requestedAgentID)
					scopedConversationID := scopeConversationID(envelope.Message.ConversationID, "discord", instanceCopy.ID, runtime.agentID)
					handleDiscordInbound(messageCtx, runtime, scopeKey, agentOverrides, runtimes, defaultRuntimeID, delivery, discordAdapter, envelope, scopedConversationID, instanceCopy.ID)
					return nil
				})
			})
		}()
	}

	if !started {
		log.Println("no runtime integrations enabled; enable health probe, telegram, feishu, wecom, or discord to start the service")
		return nil
	}

	log.Printf("clawx service started with %d runtime(s); default agent %q", len(runtimes), defaultRuntime.agentID)

	select {
	case err := <-fatalErrCh:
		return fmt.Errorf("runtime error: %w", err)
	case <-ctx.Done():
		log.Println("shutdown signal received")
		return nil
	}
}

func isEnvTrue(key string) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func normalizeRunTarget(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "serve", "telegram", "discord", "feishu", "wecom":
		return strings.ToLower(strings.TrimSpace(raw))
	default:
		return ""
	}
}

func applyRunTargetOverrides(cfg *config.Snapshot, target string) {
	if cfg == nil {
		return
	}
	target = normalizeRunTarget(target)
	if target == "" || target == "serve" {
		return
	}
	disableAllChannelInstances(cfg)
	switch target {
	case "telegram":
		enableTelegramInstances(cfg)
	case "discord":
		enableDiscordInstances(cfg)
	case "feishu":
		enableFeishuInstances(cfg)
	case "wecom":
		enableWeComInstances(cfg)
	}
	cfg.TelegramEnabled = target == "telegram"
	cfg.DiscordEnabled = target == "discord"
	cfg.FeishuEnabled = target == "feishu"
	cfg.WeComEnabled = target == "wecom"
}

func disableAllChannelInstances(cfg *config.Snapshot) {
	for idx := range cfg.TelegramInstances {
		cfg.TelegramInstances[idx].Enabled = false
	}
	for idx := range cfg.DiscordInstances {
		cfg.DiscordInstances[idx].Enabled = false
	}
	for idx := range cfg.FeishuInstances {
		cfg.FeishuInstances[idx].Enabled = false
	}
	for idx := range cfg.WeComInstances {
		cfg.WeComInstances[idx].Enabled = false
	}
}

func enableTelegramInstances(cfg *config.Snapshot) {
	for idx := range cfg.TelegramInstances {
		cfg.TelegramInstances[idx].Enabled = true
	}
}

func enableDiscordInstances(cfg *config.Snapshot) {
	for idx := range cfg.DiscordInstances {
		cfg.DiscordInstances[idx].Enabled = true
	}
}

func enableFeishuInstances(cfg *config.Snapshot) {
	for idx := range cfg.FeishuInstances {
		cfg.FeishuInstances[idx].Enabled = true
	}
}

func enableWeComInstances(cfg *config.Snapshot) {
	for idx := range cfg.WeComInstances {
		cfg.WeComInstances[idx].Enabled = true
	}
}

func runAdapterWithRetry(ctx context.Context, scope adapterRetryScope, listen func(context.Context) error) {
	backoff := 2 * time.Second
	const maxBackoff = 60 * time.Second
	retryCount := 0
	channel := strings.TrimSpace(scope.channel)
	if channel == "" {
		channel = "unknown"
	}
	instance := strings.TrimSpace(scope.instance)
	if instance == "" {
		instance = "default"
	}
	component := strings.TrimSpace(scope.component)
	if component == "" {
		component = "adapter"
	}

	for {
		if ctx.Err() != nil {
			return
		}
		err := listen(ctx)
		if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}

		retryCount++
		log.Printf(
			"channel adapter stopped: channel=%s instance=%s component=%s retry_count=%d last_error=%q retry_in=%s",
			channel,
			instance,
			component,
			retryCount,
			err.Error(),
			backoff,
		)
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func normalizeWebhookRoutePath(rawPath, rawURL, instanceID string) string {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		if parsed, err := url.Parse(strings.TrimSpace(rawURL)); err == nil {
			path = strings.TrimSpace(parsed.Path)
		}
	}
	if path == "" {
		path = "/webhooks/telegram/" + sanitizeConversationSegment(instanceID, "default")
	}

	path = strings.TrimSpace(path)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	if path != "/" {
		path = strings.TrimRight(path, "/")
	}
	return path
}

func normalizeFeishuRoutePath(instanceID string) string {
	segment := sanitizeConversationSegment(instanceID, "default")
	return "/webhooks/feishu/" + segment
}

func normalizeWeComRoutePath(instanceID string) string {
	segment := sanitizeConversationSegment(instanceID, "default")
	return "/webhooks/wecom/" + segment
}

type sessionStore interface {
	sessiondomain.Repository
	sessiondomain.Locker
}

func newSessionStore(cfg config.Snapshot) (sessionStore, func(), error) {
	if cfg.Database.Enabled {
		if strings.ToLower(strings.TrimSpace(cfg.Database.Driver)) != "postgres" {
			return nil, nil, fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
		}

		db, repo, err := persistence.NewPostgresSessionStore(context.Background(), persistence.PostgresConfig{
			Host:       cfg.Database.Host,
			Port:       cfg.Database.Port,
			Database:   cfg.Database.Name,
			User:       cfg.Database.User,
			Password:   cfg.Database.Password,
			SSLMode:    cfg.Database.SSLMode,
			AutoCreate: cfg.Database.AutoCreate,
		})
		if err != nil {
			return nil, nil, err
		}
		log.Printf("session store initialized: driver=postgres host=%s port=%d db=%s user=%s", cfg.Database.Host, cfg.Database.Port, cfg.Database.Name, cfg.Database.User)
		return repo, func() {
			if err := db.Close(); err != nil {
				log.Printf("close postgres store: %v", err)
			}
		}, nil
	}

	repo, err := persistence.NewSessionFileRepository(config.StateDir())
	if err != nil {
		return nil, nil, err
	}
	log.Printf("session store initialized: driver=file state_dir=%s", config.StateDir())
	return repo, func() {}, nil
}

func runConfigEntry(args []string) error {
	if err := config.EnsureStateLayout(); err != nil {
		return fmt.Errorf("prepare state layout: %w", err)
	}

	if len(args) == 0 {
		return runConfigCommand()
	}
	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "help", "-h", "--help":
		printConfigUsage()
		return nil
	case "agent":
		return runConfigAgentCommand(args[1:])
	case "channel":
		return runConfigChannelCommand(args[1:])
	case "path":
		fmt.Fprintln(os.Stdout, config.Path())
		return nil
	case "get":
		return runConfigGet(args[1:])
	case "set":
		return runConfigSet(args[1:])
	case "show":
		path := config.Path()
		return reportExistingConfig(path)
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}

func runConfigAgentCommand(args []string) error {
	if len(args) == 0 {
		printConfigAgentUsage()
		return nil
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "list":
		return runConfigAgentList()
	case "add":
		return runConfigAgentAdd(args[1:])
	case "default":
		return runConfigAgentDefault(args[1:])
	default:
		return fmt.Errorf("unknown config agent command %q", args[0])
	}
}

func runConfigAgentList() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if len(cfg.Agents) == 0 {
		fmt.Fprintln(os.Stdout, "No agents configured.")
		return nil
	}

	ids := sortedAgentIDs(cfg.Agents)
	for _, id := range ids {
		agent := cfg.Agents[id]
		defaultMark := ""
		if strings.TrimSpace(cfg.DefaultAgentID) == agent.ID {
			defaultMark = " (default)"
		}
		fmt.Fprintf(os.Stdout, "- %s%s\n", agent.ID, defaultMark)
		fmt.Fprintf(os.Stdout, "  profile: %s\n", agent.ProfileID)
		fmt.Fprintf(os.Stdout, "  workspace: %s\n", agent.Workspace)
		fmt.Fprintf(os.Stdout, "  timeout: %s\n", agent.Timeout)
	}
	return nil
}

func runConfigAgentAdd(args []string) error {
	fs := flag.NewFlagSet("clawx config agent add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	idFlag := fs.String("id", "", "agent id")
	profileFlag := fs.String("profile", "codex", "profile id")
	workspaceFlag := fs.String("workspace", "", "agent workspace path")
	timeoutFlag := fs.Int("timeout", 600, "timeout in seconds")
	defaultFlag := fs.Bool("default", false, "set as default agent")

	if err := fs.Parse(args); err != nil {
		return err
	}

	id := strings.TrimSpace(*idFlag)
	if id == "" && fs.NArg() > 0 {
		id = strings.TrimSpace(fs.Arg(0))
	}
	if id == "" {
		return fmt.Errorf("agent id is required; use --id <agent-id>")
	}

	path, err := config.UpsertAgent(config.AgentUpsertOptions{
		ID:             id,
		ProfileID:      strings.TrimSpace(*profileFlag),
		Workspace:      strings.TrimSpace(*workspaceFlag),
		TimeoutSeconds: *timeoutFlag,
		SetAsDefault:   *defaultFlag,
	})
	if err != nil {
		return err
	}

	workspace := strings.TrimSpace(*workspaceFlag)
	if workspace == "" {
		workspace = config.SuggestedAgentWorkspace(id)
	}
	fmt.Fprintf(os.Stdout, "agent %q saved (workspace=%s)\n", id, workspace)
	fmt.Fprintf(os.Stdout, "config updated: %s\n", path)
	return nil
}

func runConfigAgentDefault(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("agent id is required; usage: clawx config agent default <agent-id>")
	}
	agentID := strings.TrimSpace(args[0])
	path, err := config.SetDefaultAgent(agentID)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "default agent set to %q\n", agentID)
	fmt.Fprintf(os.Stdout, "config updated: %s\n", path)
	return nil
}

func printConfigAgentUsage() {
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  clawx config agent list")
	fmt.Fprintln(os.Stdout, "  clawx config agent add --id <agent-id> [--profile codex|claude|local-smoke] [--workspace <path>] [--timeout <seconds>] [--default]")
	fmt.Fprintln(os.Stdout, "  clawx config agent default <agent-id>")
}

func printConfigUsage() {
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  clawx config")
	fmt.Fprintln(os.Stdout, "  clawx config show")
	fmt.Fprintln(os.Stdout, "  clawx config path")
	fmt.Fprintln(os.Stdout, "  clawx config get [dot-key]")
	fmt.Fprintln(os.Stdout, "  clawx config set <dot-key> <value>")
	fmt.Fprintln(os.Stdout, "  clawx config channel [telegram|discord|feishu|wecom]")
	fmt.Fprintln(os.Stdout, "  clawx config agent list")
	fmt.Fprintln(os.Stdout, "  clawx config agent add --id <agent-id> [--profile codex|claude|local-smoke] [--workspace <path>] [--timeout <seconds>] [--default]")
	fmt.Fprintln(os.Stdout, "  clawx config agent default <agent-id>")
}

func runConfigGet(args []string) error {
	key := ""
	if len(args) > 0 {
		key = strings.TrimSpace(args[0])
	}

	value, err := config.GetValueByDotKey(key)
	if err != nil {
		if errors.Is(err, config.ErrConfigKeyNotFound) {
			return fmt.Errorf("config key %q not found", key)
		}
		return err
	}

	switch typed := value.(type) {
	case map[string]any, []any:
		body, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, string(body))
	default:
		fmt.Fprintln(os.Stdout, typed)
	}
	return nil
}

func runConfigSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: clawx config set <dot-key> <value>")
	}

	key := strings.TrimSpace(args[0])
	value := strings.Join(args[1:], " ")
	path, err := config.SetValueByDotKey(key, value)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "config updated: %s\n", path)
	return nil
}

func runConfigChannelDiscord() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Discord 增量配置（回车保持原值）")

	enabled, err := promptBool("启用 Discord?", cfg.DiscordEnabled)
	if err != nil {
		return err
	}
	token, err := promptString("Discord Bot Token (留空保持): ")
	if err != nil {
		return err
	}
	defaultAgent, err := promptString(fmt.Sprintf("Discord defaultAgent (留空保持，当前 %s): ", firstNonEmpty(cfg.DiscordDefaultAgentID, cfg.DefaultAgentID, "main")))
	if err != nil {
		return err
	}
	requireMention, err := promptBool("群聊要求 @bot 触发?", cfg.DiscordRequireMention)
	if err != nil {
		return err
	}
	channelIDsRaw, err := promptString(fmt.Sprintf("allowedChannelIds 逗号分隔 (留空保持，当前 %s): ", summarizeList(cfg.DiscordAllowedChannelIDs)))
	if err != nil {
		return err
	}

	updates := map[string]string{
		"channels.discord.enabled":        strconv.FormatBool(enabled),
		"channels.discord.requireMention": strconv.FormatBool(requireMention),
	}
	if strings.TrimSpace(token) != "" {
		updates["channels.discord.botToken"] = strings.TrimSpace(token)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		updates["channels.discord.defaultAgent"] = strings.TrimSpace(defaultAgent)
	}
	if strings.TrimSpace(channelIDsRaw) != "" {
		values := splitCSV(strings.TrimSpace(channelIDsRaw))
		payload, err := json.Marshal(values)
		if err != nil {
			return err
		}
		updates["channels.discord.allowedChannelIds"] = string(payload)
	}

	if _, err := config.SetValuesByDotKey(updates); err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, "Discord channel config updated.")
	return nil
}

func splitCSV(raw string) []string {
	items := strings.Split(raw, ",")
	result := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func summarizeList(values []string) string {
	if len(values) == 0 {
		return "空"
	}
	return strings.Join(values, ",")
}

func defaultDurationSeconds(value time.Duration, fallback time.Duration) int64 {
	if value <= 0 {
		return int64(fallback / time.Second)
	}
	return int64(value / time.Second)
}

func runConfigCommand() error {
	path, exists, err := config.Exists()
	if err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}

	if !interactiveInputAvailable() {
		if !exists {
			createdPath, created, err := config.EnsureDefaultFile()
			if err != nil {
				return fmt.Errorf("prepare config: %w", err)
			}
			if created {
				log.Printf("created %q with default agent %q using profile %q", createdPath, "main", "codex")
				log.Printf("review %q and fill channel credentials before starting the service", createdPath)
				return nil
			}
		}
		return reportExistingConfig(path)
	}

	if exists {
		if err := reportExistingConfig(path); err != nil {
			return err
		}
		overwrite, err := promptBool("重新生成并覆盖现有配置?", false)
		if err != nil {
			return err
		}
		if !overwrite {
			log.Printf("keeping existing config %q", path)
			return nil
		}
	}

	baseProfileID, err := promptProfile()
	if err != nil {
		return err
	}
	mainWorkspace, err := promptWorkspace()
	if err != nil {
		return err
	}

	channels, err := promptChannelPlan()
	if err != nil {
		return err
	}

	enableTelegram := channels.enableTelegram
	telegramToken := ""
	telegramUsername := ""
	if enableTelegram {
		telegramToken, err = promptString("Telegram Bot Token (留空则跳过): ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(telegramToken) == "" {
			enableTelegram = false
		} else {
			telegramUsername, err = promptString("Telegram Bot Username (可留空): ")
			if err != nil {
				return err
			}
		}
	}

	enableDiscord := channels.enableDiscord
	discordToken := ""
	discordRequireMention := true
	if enableDiscord {
		discordToken, err = promptString("Discord Bot Token (留空则跳过): ")
		if err != nil {
			return err
		}
		if strings.TrimSpace(discordToken) == "" {
			enableDiscord = false
		} else {
			discordRequireMention, err = promptBool("群聊要求 @bot 触发?", true)
			if err != nil {
				return err
			}
		}
	}

	dbPlan, err := promptDatabasePlan()
	if err != nil {
		return err
	}

	printConfigSummary(config.BootstrapOptions{
		DefaultProfileID:      baseProfileID,
		MainWorkspace:         mainWorkspace,
		TelegramEnabled:       enableTelegram,
		TelegramToken:         telegramToken,
		TelegramBotUsername:   telegramUsername,
		DiscordEnabled:        enableDiscord,
		DiscordBotToken:       discordToken,
		DiscordRequireMention: discordRequireMention,
		DatabaseEnabled:       dbPlan.enabled,
		DatabaseDriver:        dbPlan.driver,
		DatabaseHost:          dbPlan.host,
		DatabasePort:          dbPlan.port,
		DatabaseName:          dbPlan.name,
		DatabaseUser:          dbPlan.user,
		DatabasePassword:      dbPlan.password,
		DatabaseSSLMode:       dbPlan.sslMode,
		DatabaseAutoCreate:    dbPlan.autoCreate,
	})
	confirm, err := promptBool("确认写入配置?", true)
	if err != nil {
		return err
	}
	if !confirm {
		return fmt.Errorf("configuration canceled")
	}

	writtenPath, err := config.WriteBootstrapFile(config.BootstrapOptions{
		BaseProfileID:         baseProfileID,
		DefaultProfileID:      baseProfileID,
		MainWorkspace:         mainWorkspace,
		TelegramEnabled:       enableTelegram,
		TelegramToken:         telegramToken,
		TelegramBotUsername:   telegramUsername,
		DiscordEnabled:        enableDiscord,
		DiscordBotToken:       discordToken,
		DiscordRequireMention: discordRequireMention,
		DatabaseEnabled:       dbPlan.enabled,
		DatabaseDriver:        dbPlan.driver,
		DatabaseHost:          dbPlan.host,
		DatabasePort:          dbPlan.port,
		DatabaseName:          dbPlan.name,
		DatabaseUser:          dbPlan.user,
		DatabasePassword:      dbPlan.password,
		DatabaseSSLMode:       dbPlan.sslMode,
		DatabaseAutoCreate:    dbPlan.autoCreate,
	})
	if err != nil {
		return err
	}

	log.Printf("wrote %q with default agent %q using profile %q", writtenPath, "main", baseProfileID)
	log.Printf("configuration saved to %q", writtenPath)
	return nil
}

func printUsage() {
	fmt.Fprintf(os.Stdout, "Usage: clawx [serve|run|config|skill|trace|install-service|setup-service|help]\n")
	fmt.Fprintf(os.Stdout, "  serve  Start the service. If config.json is missing, bootstrap it first.\n")
	fmt.Fprintf(os.Stdout, "  run    Start selected runtime target (serve|telegram|discord|feishu|wecom); defaults to config service.run.\n")
	fmt.Fprintf(os.Stdout, "  config Launch the interactive config wizard, or run `config agent ...` for agent management.\n")
	fmt.Fprintf(os.Stdout, "  skill  Manage skill registry (list|reload|enable|disable).\n")
	fmt.Fprintf(os.Stdout, "  trace  Analyze trace logs (cache-report).\n")
	fmt.Fprintf(os.Stdout, "  install-service  Install Linux user-level systemd service (supports --start).\n")
	fmt.Fprintf(os.Stdout, "  setup-service  One-shot setup: build binary + set service.run + install/start user service.\n")
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func printConfigSummary(opts config.BootstrapOptions) {
	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "配置摘要:")
	fmt.Fprintf(os.Stdout, "  默认执行器: %s\n", profileDisplayName(opts.DefaultProfileID))
	fmt.Fprintf(os.Stdout, "  默认项目路径: %s\n", firstNonEmpty(strings.TrimSpace(opts.MainWorkspace), config.SuggestedAgentWorkspace("main")))
	fmt.Fprintf(os.Stdout, "  Telegram: %s\n", channelSummary(
		opts.TelegramEnabled,
		strings.TrimSpace(opts.TelegramToken) != "",
		displayOrPlaceholder(strings.TrimPrefix(strings.TrimSpace(opts.TelegramBotUsername), "@")),
	))
	if opts.DiscordEnabled {
		fmt.Fprintf(os.Stdout, "  Discord: 已启用 (Bot Token: 已填写, 群聊需@bot: %s)\n", boolText(opts.DiscordRequireMention))
	} else {
		fmt.Fprintln(os.Stdout, "  Discord: 未启用")
	}
	if opts.DatabaseEnabled {
		fmt.Fprintf(os.Stdout, "  Database: 已启用 (%s://%s:%d/%s user=%s autoCreate=%s)\n",
			firstNonEmpty(strings.TrimSpace(opts.DatabaseDriver), "postgres"),
			firstNonEmpty(strings.TrimSpace(opts.DatabaseHost), "127.0.0.1"),
			defaultPort(opts.DatabasePort, 5432),
			firstNonEmpty(strings.TrimSpace(opts.DatabaseName), "claw_x"),
			firstNonEmpty(strings.TrimSpace(opts.DatabaseUser), "postgres"),
			boolText(opts.DatabaseAutoCreate),
		)
	} else {
		fmt.Fprintln(os.Stdout, "  Database: 未启用（默认使用本地文件持久化）")
	}
	fmt.Fprintln(os.Stdout, "")
}

func profileDisplayName(profileID string) string {
	switch strings.TrimSpace(profileID) {
	case "codex":
		return "Codex"
	case "claude":
		return "Claude"
	case "local-smoke":
		return "Local Smoke"
	default:
		return profileID
	}
}

func channelSummary(enabled bool, tokenConfigured bool, extra string) string {
	if !enabled {
		return "未启用"
	}
	if extra != "" {
		return fmt.Sprintf("已启用 (Token: %s, %s)", yesNoConfigured(tokenConfigured), extra)
	}
	return fmt.Sprintf("已启用 (Token: %s)", yesNoConfigured(tokenConfigured))
}

func yesNoConfigured(ok bool) string {
	if ok {
		return "已填写"
	}
	return "未填写"
}

func displayOrPlaceholder(value string) string {
	if strings.TrimSpace(value) == "" {
		return "Bot Username: 未填写"
	}
	return fmt.Sprintf("Bot Username: %s", value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func boolText(value bool) string {
	if value {
		return "是"
	}
	return "否"
}

func interactiveInputAvailable() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func reportExistingConfig(path string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load existing config: %w", err)
	}

	if cfg.ActiveAgent != nil && cfg.ActiveProfile != nil {
		log.Printf("config file %q is ready; default agent %q uses profile %q", path, cfg.ActiveAgent.ID, cfg.ActiveProfile.ID)
	} else {
		log.Printf("config file %q is ready", path)
	}
	log.Printf("edit %q if needed, then run `clawx serve`", path)
	return nil
}

func promptProfile() (string, error) {
	choice, err := promptMenu(
		"选择默认执行器:",
		[]menuOption{
			{key: "codex", label: "Codex (默认推荐)", aliases: []string{"1", "codex"}, selected: true},
			{key: "claude", label: "Claude", aliases: []string{"2", "claude"}},
			{key: "local", label: "Local Smoke", aliases: []string{"3", "local", "smoke", "local-smoke"}},
		},
	)
	if err != nil {
		return "", err
	}

	switch choice {
	case "codex":
		return "codex", nil
	case "claude":
		return "claude", nil
	default:
		return "local-smoke", nil
	}
}

func promptWorkspace() (string, error) {
	defaultWorkspace := config.SuggestedAgentWorkspace("main")
	value, err := promptString(fmt.Sprintf("默认项目路径 Workspace (默认 %s): ", defaultWorkspace))
	if err != nil {
		return "", err
	}
	value = sanitizePromptInput(value)
	if value == "" {
		return defaultWorkspace, nil
	}
	return value, nil
}

func promptChannelPlan() (channelPlan, error) {
	choice, err := promptMenu(
		"选择要配置的 Channel:",
		[]menuOption{
			{key: "telegram", label: "只配置 Telegram", aliases: []string{"1", "telegram"}},
			{key: "discord", label: "只配置 Discord", aliases: []string{"2", "discord"}},
			{key: "both", label: "同时配置 Telegram + Discord", aliases: []string{"3", "both", "all"}},
			{key: "none", label: "暂不配置 Channel (默认)", aliases: []string{"4", "none", "skip"}, selected: true},
		},
	)
	if err != nil {
		return channelPlan{}, err
	}

	switch choice {
	case "telegram":
		return channelPlan{enableTelegram: true}, nil
	case "discord":
		return channelPlan{enableDiscord: true}, nil
	case "both":
		return channelPlan{enableTelegram: true, enableDiscord: true}, nil
	default:
		return channelPlan{}, nil
	}
}

func promptDatabasePlan() (databasePlan, error) {
	enableDB, err := promptBool("启用 PostgreSQL 持久化 Session? (可选)", false)
	if err != nil {
		return databasePlan{}, err
	}
	if !enableDB {
		return databasePlan{
			enabled:    false,
			driver:     "postgres",
			host:       "127.0.0.1",
			port:       5432,
			name:       "claw_x",
			user:       "postgres",
			sslMode:    "disable",
			autoCreate: true,
		}, nil
	}

	host, err := promptStringDefault("PostgreSQL Host", "127.0.0.1")
	if err != nil {
		return databasePlan{}, err
	}
	port, err := promptIntDefault("PostgreSQL Port", 5432)
	if err != nil {
		return databasePlan{}, err
	}
	name, err := promptStringDefault("Database Name", "claw_x")
	if err != nil {
		return databasePlan{}, err
	}
	user, err := promptStringDefault("Database User", "postgres")
	if err != nil {
		return databasePlan{}, err
	}
	password, err := promptString("Database Password (可留空): ")
	if err != nil {
		return databasePlan{}, err
	}
	sslMode, err := promptStringDefault("SSL Mode", "disable")
	if err != nil {
		return databasePlan{}, err
	}
	autoCreate, err := promptBool("启动时自动创建数据库并执行迁移?", true)
	if err != nil {
		return databasePlan{}, err
	}

	return databasePlan{
		enabled:    true,
		driver:     "postgres",
		host:       host,
		port:       port,
		name:       name,
		user:       user,
		password:   password,
		sslMode:    sslMode,
		autoCreate: autoCreate,
	}, nil
}

func promptStringDefault(label, fallback string) (string, error) {
	value, err := promptString(fmt.Sprintf("%s (默认 %s): ", strings.TrimSpace(label), strings.TrimSpace(fallback)))
	if err != nil {
		return "", err
	}
	value = sanitizePromptInput(value)
	if value == "" {
		return strings.TrimSpace(fallback), nil
	}
	return value, nil
}

func promptIntDefault(label string, fallback int) (int, error) {
	value, err := promptString(fmt.Sprintf("%s (默认 %d): ", strings.TrimSpace(label), fallback))
	if err != nil {
		return 0, err
	}
	value = sanitizePromptInput(value)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s 需要输入正整数", strings.TrimSpace(label))
	}
	return parsed, nil
}

func promptBool(prompt string, fallback bool) (bool, error) {
	defaultLabel := "否"
	trueLabel := "是"
	falseLabel := "否"
	if fallback {
		defaultLabel = "是"
		trueLabel = "是 (默认)"
		falseLabel = "否"
	} else {
		trueLabel = "是"
		falseLabel = "否 (默认)"
	}
	choice, err := promptMenu(
		fmt.Sprintf("%s 默认: %s", strings.TrimSpace(prompt), defaultLabel),
		[]menuOption{
			{key: "true", label: trueLabel, aliases: []string{"1", "y", "yes", "true"}, selected: fallback},
			{key: "false", label: falseLabel, aliases: []string{"2", "n", "no", "false"}, selected: !fallback},
		},
	)
	if err != nil {
		return false, err
	}
	if choice == "" {
		return fallback, nil
	}
	switch choice {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return fallback, nil
	}
}

func defaultPort(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func promptString(prompt string) (string, error) {
	restore, err := ensureCookedStdin()
	if err == nil && restore != nil {
		defer restore()
	}

	fmt.Fprint(os.Stdout, prompt)
	reader := bufio.NewReader(os.Stdin)
	var line strings.Builder
	pendingCaret := false
	for {
		ch, err := reader.ReadByte()
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
				if pendingCaret {
					line.WriteByte('^')
				}
				return sanitizePromptInput(line.String()), nil
			}
			if line.Len() == 0 {
				return "", err
			}
			if pendingCaret {
				line.WriteByte('^')
			}
			return sanitizePromptInput(line.String()), nil
		}

		if pendingCaret {
			switch ch {
			case 'm', 'M', 'j', 'J':
				return sanitizePromptInput(line.String()), nil
			default:
				line.WriteByte('^')
				pendingCaret = false
			}
		}

		if ch == '\n' || ch == '\r' {
			return sanitizePromptInput(line.String()), nil
		}
		// Some remote terminals emit non-standard control bytes for Enter.
		// Treat any control character as line terminator once we have input.
		if ch < 0x20 || ch == 0x7f {
			return sanitizePromptInput(line.String()), nil
		}
		if ch == '^' {
			pendingCaret = true
			continue
		}
		line.WriteByte(ch)
	}
}

func sanitizePromptInput(value string) string {
	cleaned := strings.TrimSpace(value)
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	for strings.HasSuffix(strings.ToLower(cleaned), "^m") {
		cleaned = strings.TrimSpace(cleaned[:len(cleaned)-2])
	}
	return strings.TrimSpace(cleaned)
}

func buildAgentRuntimes(cfg config.Snapshot, sessionManager *service.SessionManager) (map[string]agentRuntime, string, runnerWrapper, error) {
	runtimes := make(map[string]agentRuntime)
	// Project routing persistence is intentionally disabled in runtime path.
	// Keep route resolution in fallback mode (project=main) to avoid creating
	// ~/.clawx/projects/*.json in default flows.
	var projectService service.ProjectCommandService
	var err error
	memoryService, err := newMemoryCommandService(cfg, projectService)
	if err != nil {
		return nil, "", nil, fmt.Errorf("init memory service: %w", err)
	}
	attachmentStore, err := newAttachmentContextStore(cfg)
	if err != nil {
		return nil, "", nil, fmt.Errorf("init attachment context store: %w", err)
	}
	serviceCommandService, err := newServiceCommandService(cfg)
	if err != nil {
		return nil, "", nil, fmt.Errorf("init service command service: %w", err)
	}
	scheduleCommandService, scheduleRunner, err := newScheduleCommandService(cfg)
	if err != nil {
		return nil, "", nil, fmt.Errorf("init schedule command service: %w", err)
	}
	if len(cfg.Agents) > 0 && len(cfg.ProviderProfiles) > 0 {
		agentIDs := sortedAgentIDs(cfg.Agents)
		for _, agentID := range agentIDs {
			agent := cfg.Agents[agentID]
			profile, ok := cfg.ProviderProfiles[agent.ProfileID]
			if !ok {
				return nil, "", nil, fmt.Errorf("agent %q references unknown profile %q", agent.ID, agent.ProfileID)
			}

			timeout := cfg.Timeout
			if agent.Timeout > 0 {
				timeout = agent.Timeout
			}

			workspace := cfg.DefaultCWD
			if strings.TrimSpace(agent.Workspace) != "" {
				workspace = strings.TrimSpace(agent.Workspace)
			}

			runner := backend.NewProfileRunner(agent.ID, timeout, backend.Profile{
				Kind:       profile.Kind,
				Command:    profile.Command,
				Args:       profile.Args,
				HealthArgs: profile.HealthArgs,
				Model:      profile.Model,
			}, cfg.ValidateWorkingDirectory)

			runtimeCfg := cfg
			runtimeCfg.DefaultCWD = workspace
			runtimeCfg.Timeout = timeout
			registry, pipeline, err := buildSkillRuntimeComponents(runtimeCfg, agent.ID, workspace)
			if err != nil {
				return nil, "", nil, fmt.Errorf("init skill runtime for agent %q: %w", agent.ID, err)
			}
			skillControl, err := buildSkillControlRuntime(cfg, agent.ID)
			if err != nil {
				return nil, "", nil, fmt.Errorf("init skill control runtime for agent %q: %w", agent.ID, err)
			}
			router := service.NewRouter(
				runtimeCfg,
				sessionManager,
				runner,
				service.WithIntentPipeline(pipeline),
				service.WithMemoryCommandService(memoryService),
				service.WithServiceCommandService(serviceCommandService),
				service.WithScheduleCommandService(scheduleCommandService),
			)
			profileCommand := strings.TrimSpace(profile.Command)
			if profileCommand == "" {
				switch strings.ToLower(strings.TrimSpace(profile.Kind)) {
				case "codex-cli":
					profileCommand = "codex"
				case "claude-cli":
					profileCommand = "claude"
				}
			}

			runtimes[agent.ID] = agentRuntime{
				agentID:         agent.ID,
				backendName:     runner.Name(),
				cwd:             workspace,
				cfgSnapshot:     cfg,
				projectCWD:      projectService,
				attachmentStore: attachmentStore,
				profileKind:     profile.Kind,
				profileCmd:      profileCommand,
				profileModel:    strings.TrimSpace(profile.Model),
				router:          router,
				runner:          runner,
				skills:          registry,
				skillControl:    skillControl,
			}
		}
	}

	defaultAgentID := strings.TrimSpace(cfg.DefaultAgentID)
	if defaultAgentID == "" && cfg.ActiveAgent != nil {
		defaultAgentID = strings.TrimSpace(cfg.ActiveAgent.ID)
	}
	if defaultAgentID != "" {
		if _, ok := runtimes[defaultAgentID]; ok {
			return runtimes, defaultAgentID, scheduleRunner, nil
		}
	}
	if len(runtimes) > 0 {
		ids := make([]string, 0, len(runtimes))
		for id := range runtimes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return runtimes, ids[0], scheduleRunner, nil
	}

	runner := buildRunner(cfg)
	primaryID := "primary"
	profileKind := "generic-cli"
	profileCmd := cfg.ExecCommand
	profileModel := ""
	if cfg.ActiveProfile != nil {
		if strings.TrimSpace(cfg.ActiveProfile.Kind) != "" {
			profileKind = cfg.ActiveProfile.Kind
		}
		if strings.TrimSpace(cfg.ActiveProfile.Command) != "" {
			profileCmd = cfg.ActiveProfile.Command
		}
		profileModel = strings.TrimSpace(cfg.ActiveProfile.Model)
	}
	var primaryRegistry *skillregistry.Service
	var primarySkillControl *skillorchestrator.Runtime
	primaryRouter := func() *service.Router {
		registry, pipeline, err := buildSkillRuntimeComponents(cfg, primaryID, cfg.DefaultCWD)
		if err != nil {
			return service.NewRouter(
				cfg,
				sessionManager,
				runner,
				service.WithMemoryCommandService(memoryService),
				service.WithServiceCommandService(serviceCommandService),
				service.WithScheduleCommandService(scheduleCommandService),
			)
		}
		primaryRegistry = registry
		primarySkillControl, _ = buildSkillControlRuntime(cfg, primaryID)
		return service.NewRouter(
			cfg,
			sessionManager,
			runner,
			service.WithIntentPipeline(pipeline),
			service.WithMemoryCommandService(memoryService),
			service.WithServiceCommandService(serviceCommandService),
			service.WithScheduleCommandService(scheduleCommandService),
		)
	}()

	runtimes[primaryID] = agentRuntime{
		agentID:         primaryID,
		backendName:     runner.Name(),
		cwd:             cfg.DefaultCWD,
		cfgSnapshot:     cfg,
		projectCWD:      projectService,
		attachmentStore: attachmentStore,
		profileKind:     profileKind,
		profileCmd:      profileCmd,
		profileModel:    profileModel,
		router:          primaryRouter,
		runner:          runner,
		skills:          primaryRegistry,
		skillControl:    primarySkillControl,
	}
	return runtimes, primaryID, scheduleRunner, nil
}

func buildSkillControlRuntime(cfg config.Snapshot, agentID string) (*skillorchestrator.Runtime, error) {
	segment := strings.TrimSpace(agentID)
	if segment == "" {
		segment = "default"
	}
	replacer := strings.NewReplacer("/", "_", "\\", "_", " ", "_", ":", "_")
	segment = replacer.Replace(segment)
	registryPath := filepath.Join(config.SkillStateDir(), "skill_registry_"+segment+".json")
	policyPath := filepath.Join(config.SkillStateDir(), "skill_policy_"+segment+".json")
	bindingPath := filepath.Join(config.SkillStateDir(), "skill_bindings.json")

	registryStore, err := persistence.NewSkillRegistryFileStore(registryPath)
	if err != nil {
		return nil, err
	}
	policyStore, err := persistence.NewSkillPolicyFileStore(policyPath)
	if err != nil {
		return nil, err
	}
	bindingStore, err := persistence.NewSkillBindingFileStore(bindingPath)
	if err != nil {
		return nil, err
	}
	return skillorchestrator.NewRuntime(policyStore, bindingStore, registryStore, buildAgentStateProvider(cfg)), nil
}

func buildAgentStateProvider(cfg config.Snapshot) skillorchestrator.AgentStateProvider {
	values := make(map[string]skillorchestrator.AgentState, len(cfg.Agents))
	defaultID := strings.TrimSpace(cfg.DefaultAgentID)
	for id, agent := range cfg.Agents {
		values[strings.TrimSpace(id)] = skillorchestrator.AgentState{
			AgentID:   strings.TrimSpace(id),
			Workspace: strings.TrimSpace(agent.Workspace),
			ProfileID: strings.TrimSpace(agent.ProfileID),
			IsDefault: strings.TrimSpace(id) == defaultID,
		}
	}
	return skillorchestrator.NewMapAgentStateProvider(values)
}

func sortedAgentIDs(values map[string]config.Agent) []string {
	if len(values) == 0 {
		return nil
	}
	ids := make([]string, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func selectRuntime(runtimes map[string]agentRuntime, defaultAgentID, requestedAgentID string) agentRuntime {
	if runtime, ok := runtimes[strings.TrimSpace(requestedAgentID)]; ok {
		return runtime
	}
	if runtime, ok := runtimes[strings.TrimSpace(defaultAgentID)]; ok {
		return runtime
	}
	for _, runtime := range runtimes {
		return runtime
	}
	return agentRuntime{}
}

func resolveDiscordAgentID(cfg config.Snapshot, instance config.DiscordInstance, envelope discordchat.InboundEnvelope) string {
	targetChannelID := strings.TrimSpace(envelope.Target.ChannelID)
	if agentID := resolveAgentBinding(instance.AgentBindings, "channel", targetChannelID, envelope.Message); agentID != "" {
		return agentID
	}
	if agentID := resolveAgentBinding(cfg.DiscordAgentBindings, "channel", targetChannelID, envelope.Message); agentID != "" {
		return agentID
	}
	if strings.TrimSpace(instance.DefaultAgentID) != "" {
		return strings.TrimSpace(instance.DefaultAgentID)
	}
	if strings.TrimSpace(cfg.DiscordDefaultAgentID) != "" {
		return strings.TrimSpace(cfg.DiscordDefaultAgentID)
	}
	return strings.TrimSpace(cfg.DefaultAgentID)
}

func resolveTelegramAgentID(cfg config.Snapshot, instance config.TelegramInstance, envelope telegramchat.InboundEnvelope) string {
	targetChatID := strconv.FormatInt(envelope.Target.ChatID, 10)
	if agentID := resolveAgentBinding(instance.AgentBindings, "chat", targetChatID, envelope.Message); agentID != "" {
		return agentID
	}
	if agentID := resolveAgentBinding(cfg.TelegramAgentBindings, "chat", targetChatID, envelope.Message); agentID != "" {
		return agentID
	}
	if strings.TrimSpace(instance.DefaultAgentID) != "" {
		return strings.TrimSpace(instance.DefaultAgentID)
	}
	if strings.TrimSpace(cfg.TelegramDefaultAgentID) != "" {
		return strings.TrimSpace(cfg.TelegramDefaultAgentID)
	}
	return strings.TrimSpace(cfg.DefaultAgentID)
}

func resolveFeishuAgentID(cfg config.Snapshot, instance config.FeishuInstance, envelope feishuchat.InboundEnvelope) string {
	targetChatID := strings.TrimSpace(envelope.Target.ChatID)
	if agentID := resolveAgentBinding(instance.AgentBindings, "chat", targetChatID, envelope.Message); agentID != "" {
		return agentID
	}
	if agentID := resolveAgentBinding(cfg.FeishuAgentBindings, "chat", targetChatID, envelope.Message); agentID != "" {
		return agentID
	}
	if strings.TrimSpace(instance.DefaultAgentID) != "" {
		return strings.TrimSpace(instance.DefaultAgentID)
	}
	if strings.TrimSpace(cfg.FeishuDefaultAgentID) != "" {
		return strings.TrimSpace(cfg.FeishuDefaultAgentID)
	}
	return strings.TrimSpace(cfg.DefaultAgentID)
}

func resolveWeComAgentID(cfg config.Snapshot, instance config.WeComInstance, envelope wecomchat.InboundEnvelope) string {
	targetUserID := strings.TrimSpace(envelope.Target.ToUser)
	if agentID := resolveAgentBinding(instance.AgentBindings, "user", targetUserID, envelope.Message); agentID != "" {
		return agentID
	}
	if agentID := resolveAgentBinding(cfg.WeComAgentBindings, "user", targetUserID, envelope.Message); agentID != "" {
		return agentID
	}
	if strings.TrimSpace(instance.DefaultAgentID) != "" {
		return strings.TrimSpace(instance.DefaultAgentID)
	}
	if strings.TrimSpace(cfg.WeComDefaultAgentID) != "" {
		return strings.TrimSpace(cfg.WeComDefaultAgentID)
	}
	return strings.TrimSpace(cfg.DefaultAgentID)
}

func resolveAgentBinding(bindings map[string]string, targetKind, targetID string, message chatiface.Message) string {
	if len(bindings) == 0 {
		return ""
	}
	keys := []string{
		strings.TrimSpace(targetID),
		fmt.Sprintf("%s:%s", strings.TrimSpace(targetKind), strings.TrimSpace(targetID)),
		fmt.Sprintf("user:%s", strings.TrimSpace(message.UserID)),
		fmt.Sprintf("conversation:%s", strings.TrimSpace(message.ConversationID)),
	}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if agentID, ok := bindings[key]; ok {
			agentID = strings.TrimSpace(agentID)
			if agentID != "" {
				return agentID
			}
		}
	}
	return ""
}

func scopeConversationID(baseConversationID, channel, instanceID, agentID string) string {
	base := strings.TrimSpace(baseConversationID)
	if base == "" {
		base = "-"
	}
	channel = sanitizeConversationSegment(channel, "channel")
	instanceID = sanitizeConversationSegment(instanceID, "default")
	agentID = sanitizeConversationSegment(agentID, "primary")
	return fmt.Sprintf("%s|ch=%s|inst=%s|agent=%s", base, channel, instanceID, agentID)
}

func sanitizeConversationSegment(raw, fallback string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		value = fallback
	}
	value = strings.ReplaceAll(value, "|", "_")
	return value
}

func buildRunner(cfg config.Snapshot) *backend.DirectRunner {
	if cfg.ActiveProfile != nil {
		name := cfg.ActiveProfile.ID
		if cfg.ActiveAgent != nil && cfg.ActiveAgent.ID != "" {
			name = cfg.ActiveAgent.ID
		}
		return backend.NewProfileRunner(name, cfg.Timeout, backend.Profile{
			Kind:       cfg.ActiveProfile.Kind,
			Command:    cfg.ActiveProfile.Command,
			Args:       cfg.ActiveProfile.Args,
			HealthArgs: cfg.ActiveProfile.HealthArgs,
			Model:      cfg.ActiveProfile.Model,
		}, cfg.ValidateWorkingDirectory)
	}

	return backend.NewDirectRunner("primary", cfg.Timeout, backend.Options{
		Command:     cfg.ExecCommand,
		Args:        cfg.ExecArgs,
		HealthArgs:  cfg.ExecHealthArgs,
		ValidateCWD: cfg.ValidateWorkingDirectory,
	})
}

func startHTTPServer(ctx context.Context, cfg config.Snapshot, mux *http.ServeMux, errCh chan<- error) *http.Server {
	server := &http.Server{
		Addr:              cfg.HTTPListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("http runtime listening on http://%s", cfg.HTTPListenAddr)
		if cfg.HealthProbeEnabled {
			log.Printf("health probe available at http://%s%s", cfg.HTTPListenAddr, cfg.HealthProbePath)
		}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	return server
}

func resolveExecutionCWD(ctx context.Context, runtime agentRuntime, decision service.Decision) string {
	defaultCWD := strings.TrimSpace(runtime.cwd)
	if defaultCWD == "" {
		defaultCWD = "."
	}
	projectID := strings.TrimSpace(decision.ProjectID)
	if projectID == "" || runtime.projectCWD == nil {
		return normalizeExecutionCWD(runtime, defaultCWD)
	}
	record, err := runtime.projectCWD.GetProject(ctx, projectID)
	if err != nil {
		log.Printf("resolve execute cwd fallback: agent=%s project_id=%s err=%v", runtime.agentID, projectID, err)
		return normalizeExecutionCWD(runtime, defaultCWD)
	}
	projectWorkspace := strings.TrimSpace(record.WorkspacePath)
	if projectWorkspace == "" {
		return normalizeExecutionCWD(runtime, defaultCWD)
	}
	agentID := normalizeExecutionDirSegment(runtime.agentID)
	if agentID == "" {
		return normalizeExecutionCWD(runtime, projectWorkspace)
	}
	agentWorkspace := filepath.Join(projectWorkspace, ".agents", agentID, "workspace")
	if err := os.MkdirAll(agentWorkspace, 0o755); err != nil {
		log.Printf("resolve execute cwd agent workspace fallback: agent=%s project_id=%s path=%s err=%v", runtime.agentID, projectID, agentWorkspace, err)
		return normalizeExecutionCWD(runtime, projectWorkspace)
	}
	return normalizeExecutionCWD(runtime, agentWorkspace)
}

func normalizeExecutionCWD(runtime agentRuntime, candidate string) string {
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		candidate = "."
	}
	if runtime.cfgSnapshot.ValidateWorkingDirectory(candidate) == nil {
		return candidate
	}

	agentID := strings.TrimSpace(runtime.agentID)
	if agentID != "" {
		if agent, ok := runtime.cfgSnapshot.Agents[agentID]; ok {
			workspace := strings.TrimSpace(agent.Workspace)
			if workspace != "" && runtime.cfgSnapshot.ValidateWorkingDirectory(workspace) == nil {
				return workspace
			}
		}
	}

	if runtime.cfgSnapshot.ActiveAgent != nil {
		workspace := strings.TrimSpace(runtime.cfgSnapshot.ActiveAgent.Workspace)
		if workspace != "" && runtime.cfgSnapshot.ValidateWorkingDirectory(workspace) == nil {
			return workspace
		}
	}

	defaultCWD := strings.TrimSpace(runtime.cfgSnapshot.DefaultCWD)
	if defaultCWD != "" && runtime.cfgSnapshot.ValidateWorkingDirectory(defaultCWD) == nil {
		return defaultCWD
	}

	for _, root := range runtime.cfgSnapshot.AllowedRoots {
		root = strings.TrimSpace(root)
		if root != "" {
			return filepath.Clean(root)
		}
	}

	return "."
}

func normalizeExecutionDirSegment(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	return strings.Trim(b.String(), "-_")
}

func enforceRoutingIronLaw(message chatiface.Message, decision service.Decision) error {
	return autonomy.EnforceRoutingIronLaw(message.Text, string(decision.Kind))
}

var defaultEscalationPolicy = autonomy.NewEscalationPolicy()

type taskReceiptPhase string

const (
	taskReceiptReceived taskReceiptPhase = "received"
	taskReceiptProgress taskReceiptPhase = "progress"
	taskReceiptComplete taskReceiptPhase = "complete"
	taskReceiptFailed   taskReceiptPhase = "failed"
)

func formatTaskReceipt(phase taskReceiptPhase, detail string) string {
	detail = strings.TrimSpace(detail)
	switch phase {
	case taskReceiptReceived:
		if detail == "" {
			return "已接收，开始处理。"
		}
		return "已接收，开始处理：" + detail
	case taskReceiptProgress:
		if detail == "" {
			return "处理中，请稍候。"
		}
		return "处理中：" + detail
	case taskReceiptComplete:
		if detail == "" {
			return "处理完成。"
		}
		return detail
	case taskReceiptFailed:
		if detail == "" {
			return "处理失败。"
		}
		if strings.Contains(detail, "\n") {
			return "处理失败。\n" + detail
		}
		return "处理失败：" + detail
	default:
		return detail
	}
}

func emitExecutionFailureClassification(channel string, instanceID string, runtime agentRuntime, decision service.Decision, execErr error) {
	classification := autonomy.ClassifyFailure(execErr)
	emitAutonomyFlow(channel, instanceID, runtime, decision, "classify", "failed", classification, "execution_failed", execErr)
}

func buildAutonomyEscalationPrompt(classification autonomy.FailureClassification, attempted []string, recommendation string) string {
	lines := []string{
		"执行失败，需你确认后继续。",
		"失败类型: " + string(classification.Class),
		"失败原因: " + classification.Reason,
		"已尝试:",
	}
	if len(attempted) == 0 {
		lines = append(lines, "- 尚无可自动重试步骤")
	} else {
		for _, step := range attempted {
			step = strings.TrimSpace(step)
			if step == "" {
				continue
			}
			lines = append(lines, "- "+step)
		}
	}
	if strings.TrimSpace(recommendation) == "" {
		recommendation = "请确认是否授权继续执行恢复动作。"
	}
	lines = append(lines, "推荐操作: "+recommendation)
	return strings.Join(lines, "\n")
}

func defaultEscalationRecommendation(classification autonomy.FailureClassification) string {
	switch classification.Class {
	case autonomy.FailureClassPermission:
		return "请确认授权范围或调整 runtime.allowedRoots 后重试。"
	case autonomy.FailureClassAuth:
		return "请确认凭据配置（token/secret/profile）后重试。"
	case autonomy.FailureClassResource:
		return "执行进程超时或被系统中断。请提高该 agent timeout（例如 1200s）或将任务拆成更小步骤后继续。"
	case autonomy.FailureClassUnknown:
		return "请确认是否继续按保守策略重试，或由我输出详细诊断。"
	default:
		return "请确认是否继续执行恢复动作。"
	}
}

func formatExecutionFailureResponse(execErr error, attempted []string, recoveryExhausted bool) string {
	classification := autonomy.ClassifyFailure(execErr)
	decision := defaultEscalationPolicy.Decide(classification, recoveryExhausted)
	if !decision.ShouldEscalate {
		return formatTaskReceipt(taskReceiptFailed, chatiface.FormatError(execErr))
	}
	escalationBody := buildAutonomyEscalationPrompt(classification, attempted, defaultEscalationRecommendation(classification))
	return formatTaskReceipt(taskReceiptFailed, "自动恢复失败，需要你确认后继续。") + "\n" + escalationBody
}

func handleExecutionFailure(channel string, instanceID string, runtime agentRuntime, decision service.Decision, execErr error) string {
	classification := autonomy.ClassifyFailure(execErr)
	emitAutonomyFlow(channel, instanceID, runtime, decision, "classify", "failed", classification, "execution_failed", execErr)
	emitAutonomyFlow(channel, instanceID, runtime, decision, "attempt", "failed", classification, "auto_recovery_exhausted_or_unavailable", execErr)
	escalation := defaultEscalationPolicy.Decide(classification, true)
	if escalation.ShouldEscalate {
		emitAutonomyFlow(channel, instanceID, runtime, decision, "escalate", "prompted", classification, escalation.Reason, nil)
		emitAutonomyFlow(channel, instanceID, runtime, decision, "result", "escalated", classification, "awaiting_user_decision", nil)
		escalationBody := buildAutonomyEscalationPrompt(classification, nil, defaultEscalationRecommendation(classification))
		return formatTaskReceipt(taskReceiptFailed, "自动恢复失败，需要你确认后继续。") + "\n" + escalationBody
	}
	emitAutonomyFlow(channel, instanceID, runtime, decision, "escalate", "skipped", classification, escalation.Reason, nil)
	emitAutonomyFlow(channel, instanceID, runtime, decision, "result", "failed_no_escalation", classification, "returned_direct_error", execErr)
	return formatTaskReceipt(taskReceiptFailed, chatiface.FormatError(execErr))
}

func buildSessionCommandFromDecision(decision service.Decision, runtime agentRuntime, executeInput string, executeCWD string) command.SessionCommand {
	return command.SessionCommand{
		Mode:            command.ModeContinue,
		ConversationID:  decision.ConversationID,
		WindowID:        decision.WindowID,
		ProjectID:       decision.ProjectID,
		RouteKey:        decision.RouteKey,
		UserID:          decision.Message.UserID,
		IsDirectMessage: decision.Message.ContextFlags.IsDirectMessage,
		Input:           executeInput,
		Backend:         runtime.backendName,
		CWD:             executeCWD,
	}
}

func tryRecoverSessionFlow(
	ctx context.Context,
	channel string,
	instanceID string,
	runtime agentRuntime,
	decision service.Decision,
	classification autonomy.FailureClassification,
	runAttempt func(context.Context, int) (service.SessionFlowResult, error),
) (service.SessionFlowResult, autonomy.RecoveryExecutionResult, bool) {
	executor := autonomy.NewRecoveryExecutor(autonomy.NewDefaultRecoveryRegistry())
	var recovered service.SessionFlowResult
	result := executor.Execute(ctx, classification, func(attemptCtx context.Context, attempt int) error {
		emitAutonomyFlow(channel, instanceID, runtime, decision, "attempt", "retrying", classification, fmt.Sprintf("retry_attempt=%d", attempt), nil)
		flowResult, err := runAttempt(attemptCtx, attempt)
		if err != nil {
			emitAutonomyFlow(channel, instanceID, runtime, decision, "attempt", "retry_failed", classification, fmt.Sprintf("retry_attempt=%d", attempt), err)
			return err
		}
		recovered = flowResult
		emitAutonomyFlow(channel, instanceID, runtime, decision, "attempt", "retry_succeeded", classification, fmt.Sprintf("retry_attempt=%d", attempt), nil)
		return nil
	})
	if result.Recovered {
		emitAutonomyFlow(channel, instanceID, runtime, decision, "result", "recovered", classification, fmt.Sprintf("policy=%s attempts=%d", strings.TrimSpace(result.Policy.Name), result.Attempts), nil)
		return recovered, result, true
	}
	emitAutonomyFlow(channel, instanceID, runtime, decision, "result", "not_recovered", classification, fmt.Sprintf("attempts=%d stop_reason=%s", result.Attempts, strings.TrimSpace(result.StopReason)), result.LastError)
	return service.SessionFlowResult{}, result, false
}

func enforceAutonomyStructuredPlanGateOnOutput(output string) (string, bool, error) {
	if err := autonomy.EnforceStructuredPlanGate(output); err != nil {
		trimmed := stripActionPlanPayload(output)
		msg := "已拦截未通过 schema/allowlist 校验的结构化计划，未执行任何自动动作。"
		if strings.TrimSpace(trimmed) == "" {
			return msg, true, err
		}
		return trimmed + "\n\n" + msg, true, err
	}
	return output, false, nil
}

func handleTelegramInbound(
	ctx context.Context,
	runtime agentRuntime,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultRuntimeID string,
	delivery *service.OutputDelivery,
	adapter *telegramchat.Adapter,
	envelope telegramchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		logConfigControlHandled("telegram", instanceID, message.ConversationID, message.UserID, message.Text, response, err)
		if err != nil {
			sendTelegramDirect(ctx, adapter, envelope.Target, formatExecutionFailureResponse(err, nil, false))
			return
		}
		sendTelegramDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response, err := handleSkillChatCommand(message, runtime); handled {
		if err != nil {
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendTelegramDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleClawXSkillMetaCommand(runtime, message.Text); handled {
		sendTelegramDirect(ctx, adapter, envelope.Target, response)
		return
	}

	routeStarted := time.Now()
	decision, err := runtime.router.Route(ctx, message)
	channelRouteMetrics.Observe("telegram", instanceID, time.Since(routeStarted))
	if err != nil {
		sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	if err := enforceRoutingIronLaw(message, decision); err != nil {
		sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	eventID := normalizeAuditEventID(envelope.EventID)
	sendTelegramDirect(ctx, adapter, envelope.Target, formatTaskReceipt(taskReceiptProgress, "正在思考并执行。"))

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
		if err != nil {
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}

		if result.CreatedSessionID != "" {
			adapter.BindSession(result.CreatedSessionID, envelope.Target)
		}
		if result.ResumedSessionID != "" {
			adapter.BindSession(result.ResumedSessionID, envelope.Target)
		}
		if result.CancelledSessionID != "" {
			adapter.BindSession(result.CancelledSessionID, envelope.Target)
		}

		sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatControlResponse(toControlResponse(result)))
	case service.DecisionSkill, service.DecisionExecute:
		started := time.Now()
		decision = mergeDecisionAttachments(runtime, decision)
		executeInput := buildExecutionInput(decision, runtime)
		executeCWD := resolveExecutionCWD(ctx, runtime, decision)
		emitTrace("llm_io", map[string]any{
			"phase":                  "request",
			"channel":                "telegram",
			"instance":               strings.TrimSpace(instanceID),
			"agent_id":               strings.TrimSpace(runtime.agentID),
			"conversation_id":        strings.TrimSpace(decision.ConversationID),
			"project_id":             strings.TrimSpace(decision.ProjectID),
			"intent_kind":            string(decision.Kind),
			"input_chars":            len(executeInput),
			"input_preview":          tracePreview(executeInput, 400),
			"prompt_cache_key":       buildTracePromptCacheKey(decision, runtime),
			"prompt_cache_retention": resolveTracePromptCacheRetention(),
		})
		log.Printf("telegram execute begin: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		sessionCmd := buildSessionCommandFromDecision(decision, runtime, executeInput, executeCWD)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, sessionCmd)
		if err != nil {
			log.Printf("telegram execute failed: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			classification := autonomy.ClassifyFailure(err)
			if recoveredResult, _, recovered := tryRecoverSessionFlow(ctx, "telegram", instanceID, runtime, decision, classification, func(retryCtx context.Context, _ int) (service.SessionFlowResult, error) {
				return runtime.router.HandleSessionFlow(retryCtx, sessionCmd)
			}); recovered {
				flowResult = recoveredResult
			} else {
				sendTelegramDirect(ctx, adapter, envelope.Target, handleExecutionFailure("telegram", instanceID, runtime, decision, err))
				return
			}
		}
		emitTrace("llm_io", map[string]any{
			"phase":                "response",
			"channel":              "telegram",
			"instance":             strings.TrimSpace(instanceID),
			"agent_id":             strings.TrimSpace(runtime.agentID),
			"conversation_id":      strings.TrimSpace(decision.ConversationID),
			"project_id":           strings.TrimSpace(decision.ProjectID),
			"intent_kind":          string(decision.Kind),
			"output_chars":         len(flowResult.Execution.Output),
			"output_preview":       tracePreview(flowResult.Execution.Output, 400),
			"state":                string(flowResult.Execution.State),
			"prompt_cached_tokens": flowResult.Execution.PromptCachedTokens,
			"prompt_tokens":        flowResult.Execution.PromptTokens,
			"completion_tokens":    flowResult.Execution.CompletionTokens,
			"total_tokens":         flowResult.Execution.TotalTokens,
		})
		recordTokenUsage("telegram", instanceID, runtime, decision, flowResult)
		log.Printf("telegram execute done: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		responseAgentID := strings.TrimSpace(runtime.agentID)
		blockedByGate := false
		if gatedOutput, blocked, gateErr := enforceAutonomyStructuredPlanGateOnOutput(output); gateErr != nil {
			log.Printf("telegram autonomy plan gate blocked: channel=telegram instance=%s scope=%s err=%v", instanceID, scopeKey, gateErr)
			output = gatedOutput
			blockedByGate = blocked
			classification := autonomy.FailureClassification{Class: autonomy.FailureClassUnknown, Recoverable: false, Reason: "schema_allowlist_gate_reject"}
			emitAutonomyFlow("telegram", instanceID, runtime, decision, "attempt", "blocked_by_gate", classification, "structured_plan_rejected", gateErr)
			emitAutonomyFlow("telegram", instanceID, runtime, decision, "result", "blocked", classification, "no_auto_action_executed", nil)
		} else {
			output = gatedOutput
		}
		if !blockedByGate {
			if execApplied, ok, execErr := maybeAutoApplyActionPlan(ctx, runtime, decision, output, "telegram", instanceID, scopeKey, overrides, runtimes, defaultRuntimeID, executeCWD); execErr != nil {
				log.Printf("telegram unified action plan apply failed: channel=telegram instance=%s scope=%s err=%v", instanceID, scopeKey, execErr)
			} else if ok {
				output = stripActionPlanPayload(output)
				output = strings.TrimSpace(output + "\n\n" + formatActionPlanApplyResult(execApplied))
				if strings.TrimSpace(execApplied.ResponseAgentID) != "" {
					responseAgentID = strings.TrimSpace(execApplied.ResponseAgentID)
				}
			}
		}
		output = finalizeExecutionOutput(decision, runtime, output)
		output = applyRuntimeAgentLabel(output, responseAgentID)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, telegramchat.MaxMessageLength, 1)
		deliverTelegramOutputFiles(ctx, adapter, envelope.Target, output, decision.Message.Text)
	}
}

func sendTelegramDirect(ctx context.Context, adapter *telegramchat.Adapter, target telegramchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send telegram message: %v", err)
	}
}

func handleFeishuInbound(
	ctx context.Context,
	runtime agentRuntime,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultRuntimeID string,
	delivery *service.OutputDelivery,
	adapter *feishuchat.Adapter,
	envelope feishuchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		logConfigControlHandled("feishu", instanceID, message.ConversationID, message.UserID, message.Text, response, err)
		if err != nil {
			sendFeishuDirect(ctx, adapter, envelope.Target, formatExecutionFailureResponse(err, nil, false))
			return
		}
		sendFeishuDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response, err := handleSkillChatCommand(message, runtime); handled {
		if err != nil {
			sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendFeishuDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleClawXSkillMetaCommand(runtime, message.Text); handled {
		sendFeishuDirect(ctx, adapter, envelope.Target, response)
		return
	}

	routeStarted := time.Now()
	decision, err := runtime.router.Route(ctx, message)
	channelRouteMetrics.Observe("feishu", instanceID, time.Since(routeStarted))
	if err != nil {
		sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	if err := enforceRoutingIronLaw(message, decision); err != nil {
		sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	eventID := normalizeAuditEventID(envelope.EventID)
	sendFeishuDirect(ctx, adapter, envelope.Target, formatTaskReceipt(taskReceiptProgress, "正在思考并执行。"))

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
		if err != nil {
			sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}

		if result.CreatedSessionID != "" {
			adapter.BindSession(result.CreatedSessionID, envelope.Target)
		}
		if result.ResumedSessionID != "" {
			adapter.BindSession(result.ResumedSessionID, envelope.Target)
		}
		if result.CancelledSessionID != "" {
			adapter.BindSession(result.CancelledSessionID, envelope.Target)
		}

		sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatControlResponse(toControlResponse(result)))
	case service.DecisionSkill, service.DecisionExecute:
		started := time.Now()
		decision = mergeDecisionAttachments(runtime, decision)
		executeInput := buildExecutionInput(decision, runtime)
		executeCWD := resolveExecutionCWD(ctx, runtime, decision)
		emitTrace("llm_io", map[string]any{
			"phase":                  "request",
			"channel":                "feishu",
			"instance":               strings.TrimSpace(instanceID),
			"agent_id":               strings.TrimSpace(runtime.agentID),
			"conversation_id":        strings.TrimSpace(decision.ConversationID),
			"project_id":             strings.TrimSpace(decision.ProjectID),
			"intent_kind":            string(decision.Kind),
			"input_chars":            len(executeInput),
			"input_preview":          tracePreview(executeInput, 400),
			"prompt_cache_key":       buildTracePromptCacheKey(decision, runtime),
			"prompt_cache_retention": resolveTracePromptCacheRetention(),
		})
		log.Printf("feishu execute begin: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		sessionCmd := buildSessionCommandFromDecision(decision, runtime, executeInput, executeCWD)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, sessionCmd)
		if err != nil {
			log.Printf("feishu execute failed: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			classification := autonomy.ClassifyFailure(err)
			if recoveredResult, _, recovered := tryRecoverSessionFlow(ctx, "feishu", instanceID, runtime, decision, classification, func(retryCtx context.Context, _ int) (service.SessionFlowResult, error) {
				return runtime.router.HandleSessionFlow(retryCtx, sessionCmd)
			}); recovered {
				flowResult = recoveredResult
			} else {
				sendFeishuDirect(ctx, adapter, envelope.Target, handleExecutionFailure("feishu", instanceID, runtime, decision, err))
				return
			}
		}
		emitTrace("llm_io", map[string]any{
			"phase":                "response",
			"channel":              "feishu",
			"instance":             strings.TrimSpace(instanceID),
			"agent_id":             strings.TrimSpace(runtime.agentID),
			"conversation_id":      strings.TrimSpace(decision.ConversationID),
			"project_id":           strings.TrimSpace(decision.ProjectID),
			"intent_kind":          string(decision.Kind),
			"output_chars":         len(flowResult.Execution.Output),
			"output_preview":       tracePreview(flowResult.Execution.Output, 400),
			"state":                string(flowResult.Execution.State),
			"prompt_cached_tokens": flowResult.Execution.PromptCachedTokens,
			"prompt_tokens":        flowResult.Execution.PromptTokens,
			"completion_tokens":    flowResult.Execution.CompletionTokens,
			"total_tokens":         flowResult.Execution.TotalTokens,
		})
		recordTokenUsage("feishu", instanceID, runtime, decision, flowResult)
		log.Printf("feishu execute done: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		responseAgentID := strings.TrimSpace(runtime.agentID)
		blockedByGate := false
		if gatedOutput, blocked, gateErr := enforceAutonomyStructuredPlanGateOnOutput(output); gateErr != nil {
			log.Printf("feishu autonomy plan gate blocked: channel=feishu instance=%s scope=%s err=%v", instanceID, scopeKey, gateErr)
			output = gatedOutput
			blockedByGate = blocked
			classification := autonomy.FailureClassification{Class: autonomy.FailureClassUnknown, Recoverable: false, Reason: "schema_allowlist_gate_reject"}
			emitAutonomyFlow("feishu", instanceID, runtime, decision, "attempt", "blocked_by_gate", classification, "structured_plan_rejected", gateErr)
			emitAutonomyFlow("feishu", instanceID, runtime, decision, "result", "blocked", classification, "no_auto_action_executed", nil)
		} else {
			output = gatedOutput
		}
		if !blockedByGate {
			if execApplied, ok, execErr := maybeAutoApplyActionPlan(ctx, runtime, decision, output, "feishu", instanceID, scopeKey, overrides, runtimes, defaultRuntimeID, executeCWD); execErr != nil {
				log.Printf("feishu unified action plan apply failed: channel=feishu instance=%s scope=%s err=%v", instanceID, scopeKey, execErr)
			} else if ok {
				output = stripActionPlanPayload(output)
				output = strings.TrimSpace(output + "\n\n" + formatActionPlanApplyResult(execApplied))
				if strings.TrimSpace(execApplied.ResponseAgentID) != "" {
					responseAgentID = strings.TrimSpace(execApplied.ResponseAgentID)
				}
			}
		}
		output = finalizeExecutionOutput(decision, runtime, output)
		output = applyRuntimeAgentLabel(output, responseAgentID)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, feishuchat.MaxMessageLength, 1)
	}
}

func sendFeishuDirect(ctx context.Context, adapter *feishuchat.Adapter, target feishuchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send feishu message: %v", err)
	}
}

func handleWeComInbound(
	ctx context.Context,
	runtime agentRuntime,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultRuntimeID string,
	delivery *service.OutputDelivery,
	adapter *wecomchat.Adapter,
	envelope wecomchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		logConfigControlHandled("wecom", instanceID, message.ConversationID, message.UserID, message.Text, response, err)
		if err != nil {
			sendWeComDirect(ctx, adapter, envelope.Target, formatExecutionFailureResponse(err, nil, false))
			return
		}
		sendWeComDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response, err := handleSkillChatCommand(message, runtime); handled {
		if err != nil {
			sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendWeComDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleClawXSkillMetaCommand(runtime, message.Text); handled {
		sendWeComDirect(ctx, adapter, envelope.Target, response)
		return
	}

	routeStarted := time.Now()
	decision, err := runtime.router.Route(ctx, message)
	channelRouteMetrics.Observe("wecom", instanceID, time.Since(routeStarted))
	if err != nil {
		sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	if err := enforceRoutingIronLaw(message, decision); err != nil {
		sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	eventID := normalizeAuditEventID(envelope.EventID)
	sendWeComDirect(ctx, adapter, envelope.Target, formatTaskReceipt(taskReceiptProgress, "正在思考并执行。"))

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
		if err != nil {
			sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}

		if result.CreatedSessionID != "" {
			adapter.BindSession(result.CreatedSessionID, envelope.Target)
		}
		if result.ResumedSessionID != "" {
			adapter.BindSession(result.ResumedSessionID, envelope.Target)
		}
		if result.CancelledSessionID != "" {
			adapter.BindSession(result.CancelledSessionID, envelope.Target)
		}

		sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatControlResponse(toControlResponse(result)))
	case service.DecisionSkill, service.DecisionExecute:
		started := time.Now()
		decision = mergeDecisionAttachments(runtime, decision)
		executeInput := buildExecutionInput(decision, runtime)
		executeCWD := resolveExecutionCWD(ctx, runtime, decision)
		emitTrace("llm_io", map[string]any{
			"phase":                  "request",
			"channel":                "wecom",
			"instance":               strings.TrimSpace(instanceID),
			"agent_id":               strings.TrimSpace(runtime.agentID),
			"conversation_id":        strings.TrimSpace(decision.ConversationID),
			"project_id":             strings.TrimSpace(decision.ProjectID),
			"intent_kind":            string(decision.Kind),
			"input_chars":            len(executeInput),
			"input_preview":          tracePreview(executeInput, 400),
			"prompt_cache_key":       buildTracePromptCacheKey(decision, runtime),
			"prompt_cache_retention": resolveTracePromptCacheRetention(),
		})
		log.Printf("wecom execute begin: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		sessionCmd := buildSessionCommandFromDecision(decision, runtime, executeInput, executeCWD)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, sessionCmd)
		if err != nil {
			log.Printf("wecom execute failed: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			classification := autonomy.ClassifyFailure(err)
			if recoveredResult, _, recovered := tryRecoverSessionFlow(ctx, "wecom", instanceID, runtime, decision, classification, func(retryCtx context.Context, _ int) (service.SessionFlowResult, error) {
				return runtime.router.HandleSessionFlow(retryCtx, sessionCmd)
			}); recovered {
				flowResult = recoveredResult
			} else {
				sendWeComDirect(ctx, adapter, envelope.Target, handleExecutionFailure("wecom", instanceID, runtime, decision, err))
				return
			}
		}
		emitTrace("llm_io", map[string]any{
			"phase":                "response",
			"channel":              "wecom",
			"instance":             strings.TrimSpace(instanceID),
			"agent_id":             strings.TrimSpace(runtime.agentID),
			"conversation_id":      strings.TrimSpace(decision.ConversationID),
			"project_id":           strings.TrimSpace(decision.ProjectID),
			"intent_kind":          string(decision.Kind),
			"output_chars":         len(flowResult.Execution.Output),
			"output_preview":       tracePreview(flowResult.Execution.Output, 400),
			"state":                string(flowResult.Execution.State),
			"prompt_cached_tokens": flowResult.Execution.PromptCachedTokens,
			"prompt_tokens":        flowResult.Execution.PromptTokens,
			"completion_tokens":    flowResult.Execution.CompletionTokens,
			"total_tokens":         flowResult.Execution.TotalTokens,
		})
		recordTokenUsage("wecom", instanceID, runtime, decision, flowResult)
		log.Printf("wecom execute done: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		responseAgentID := strings.TrimSpace(runtime.agentID)
		blockedByGate := false
		if gatedOutput, blocked, gateErr := enforceAutonomyStructuredPlanGateOnOutput(output); gateErr != nil {
			log.Printf("wecom autonomy plan gate blocked: channel=wecom instance=%s scope=%s err=%v", instanceID, scopeKey, gateErr)
			output = gatedOutput
			blockedByGate = blocked
			classification := autonomy.FailureClassification{Class: autonomy.FailureClassUnknown, Recoverable: false, Reason: "schema_allowlist_gate_reject"}
			emitAutonomyFlow("wecom", instanceID, runtime, decision, "attempt", "blocked_by_gate", classification, "structured_plan_rejected", gateErr)
			emitAutonomyFlow("wecom", instanceID, runtime, decision, "result", "blocked", classification, "no_auto_action_executed", nil)
		} else {
			output = gatedOutput
		}
		if !blockedByGate {
			if execApplied, ok, execErr := maybeAutoApplyActionPlan(ctx, runtime, decision, output, "wecom", instanceID, scopeKey, overrides, runtimes, defaultRuntimeID, executeCWD); execErr != nil {
				log.Printf("wecom unified action plan apply failed: channel=wecom instance=%s scope=%s err=%v", instanceID, scopeKey, execErr)
			} else if ok {
				output = stripActionPlanPayload(output)
				output = strings.TrimSpace(output + "\n\n" + formatActionPlanApplyResult(execApplied))
				if strings.TrimSpace(execApplied.ResponseAgentID) != "" {
					responseAgentID = strings.TrimSpace(execApplied.ResponseAgentID)
				}
			}
		}
		output = finalizeExecutionOutput(decision, runtime, output)
		output = applyRuntimeAgentLabel(output, responseAgentID)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, wecomchat.MaxMessageLength, 1)
		deliverWeComOutputFiles(ctx, adapter, envelope.Target, output, decision.Message.Text)
	}
}

func sendWeComDirect(ctx context.Context, adapter *wecomchat.Adapter, target wecomchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send wecom message: %v", err)
	}
}

func handleDiscordInbound(
	ctx context.Context,
	runtime agentRuntime,
	scopeKey string,
	overrides *conversationAgentOverrides,
	runtimes map[string]agentRuntime,
	defaultRuntimeID string,
	delivery *service.OutputDelivery,
	adapter *discordchat.Adapter,
	envelope discordchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		logConfigControlHandled("discord", instanceID, message.ConversationID, message.UserID, message.Text, response, err)
		if err != nil {
			sendDiscordDirect(ctx, adapter, envelope.Target, formatExecutionFailureResponse(err, nil, false))
			return
		}
		sendDiscordDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response, err := handleSkillChatCommand(message, runtime); handled {
		if err != nil {
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendDiscordDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleClawXSkillMetaCommand(runtime, message.Text); handled {
		sendDiscordDirect(ctx, adapter, envelope.Target, response)
		return
	}

	log.Printf("discord route begin: instance=%s agent=%s conversation_id=%s text=%q", instanceID, runtime.agentID, message.ConversationID, message.Text)
	routeStarted := time.Now()
	decision, err := runtime.router.Route(ctx, message)
	channelRouteMetrics.Observe("discord", instanceID, time.Since(routeStarted))
	if err != nil {
		log.Printf("discord route rejected: %v", err)
		sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	if err := enforceRoutingIronLaw(message, decision); err != nil {
		sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	log.Printf("discord route decision: instance=%s agent=%s kind=%s conversation_id=%s project_id=%s project_mode=%s", instanceID, runtime.agentID, decision.Kind, decision.ConversationID, decision.ProjectID, decision.ProjectMode)

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID, decision.RouteKey)
		if err != nil {
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}

		if result.CreatedSessionID != "" {
			adapter.BindSession(result.CreatedSessionID, envelope.Target)
		}
		if result.ResumedSessionID != "" {
			adapter.BindSession(result.ResumedSessionID, envelope.Target)
		}
		if result.CancelledSessionID != "" {
			adapter.BindSession(result.CancelledSessionID, envelope.Target)
		}

		sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatControlResponse(toControlResponse(result)))
	case service.DecisionSkill, service.DecisionExecute:
		started := time.Now()
		decision = mergeDecisionAttachments(runtime, decision)
		executeInput := buildExecutionInput(decision, runtime)
		executeCWD := resolveExecutionCWD(ctx, runtime, decision)
		emitTrace("llm_io", map[string]any{
			"phase":                  "request",
			"channel":                "discord",
			"instance":               strings.TrimSpace(instanceID),
			"agent_id":               strings.TrimSpace(runtime.agentID),
			"conversation_id":        strings.TrimSpace(decision.ConversationID),
			"project_id":             strings.TrimSpace(decision.ProjectID),
			"intent_kind":            string(decision.Kind),
			"input_chars":            len(executeInput),
			"input_preview":          tracePreview(executeInput, 400),
			"prompt_cache_key":       buildTracePromptCacheKey(decision, runtime),
			"prompt_cache_retention": resolveTracePromptCacheRetention(),
		})
		log.Printf("discord execute begin: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		stopTyping := startDiscordTypingLoop(ctx, adapter, envelope.Target)
		defer stopTyping()
		sessionCmd := buildSessionCommandFromDecision(decision, runtime, executeInput, executeCWD)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, sessionCmd)
		if err != nil {
			log.Printf("discord execute failed: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			classification := autonomy.ClassifyFailure(err)
			if recoveredResult, _, recovered := tryRecoverSessionFlow(ctx, "discord", instanceID, runtime, decision, classification, func(retryCtx context.Context, _ int) (service.SessionFlowResult, error) {
				return runtime.router.HandleSessionFlow(retryCtx, sessionCmd)
			}); recovered {
				flowResult = recoveredResult
			} else {
				sendDiscordDirect(ctx, adapter, envelope.Target, handleExecutionFailure("discord", instanceID, runtime, decision, err))
				return
			}
		}
		emitTrace("llm_io", map[string]any{
			"phase":                "response",
			"channel":              "discord",
			"instance":             strings.TrimSpace(instanceID),
			"agent_id":             strings.TrimSpace(runtime.agentID),
			"conversation_id":      strings.TrimSpace(decision.ConversationID),
			"project_id":           strings.TrimSpace(decision.ProjectID),
			"intent_kind":          string(decision.Kind),
			"output_chars":         len(flowResult.Execution.Output),
			"output_preview":       tracePreview(flowResult.Execution.Output, 400),
			"state":                string(flowResult.Execution.State),
			"prompt_cached_tokens": flowResult.Execution.PromptCachedTokens,
			"prompt_tokens":        flowResult.Execution.PromptTokens,
			"completion_tokens":    flowResult.Execution.CompletionTokens,
			"total_tokens":         flowResult.Execution.TotalTokens,
		})
		recordTokenUsage("discord", instanceID, runtime, decision, flowResult)
		log.Printf("discord execute done: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		responseAgentID := strings.TrimSpace(runtime.agentID)
		blockedByGate := false
		if gatedOutput, blocked, gateErr := enforceAutonomyStructuredPlanGateOnOutput(output); gateErr != nil {
			log.Printf("discord autonomy plan gate blocked: channel=discord instance=%s scope=%s err=%v", instanceID, scopeKey, gateErr)
			output = gatedOutput
			blockedByGate = blocked
			classification := autonomy.FailureClassification{Class: autonomy.FailureClassUnknown, Recoverable: false, Reason: "schema_allowlist_gate_reject"}
			emitAutonomyFlow("discord", instanceID, runtime, decision, "attempt", "blocked_by_gate", classification, "structured_plan_rejected", gateErr)
			emitAutonomyFlow("discord", instanceID, runtime, decision, "result", "blocked", classification, "no_auto_action_executed", nil)
		} else {
			output = gatedOutput
		}
		if !blockedByGate {
			if execApplied, ok, execErr := maybeAutoApplyActionPlan(ctx, runtime, decision, output, "discord", instanceID, scopeKey, overrides, runtimes, defaultRuntimeID, executeCWD); execErr != nil {
				log.Printf("discord unified action plan apply failed: channel=discord instance=%s scope=%s err=%v", instanceID, scopeKey, execErr)
			} else if ok {
				output = stripActionPlanPayload(output)
				output = strings.TrimSpace(output + "\n\n" + formatActionPlanApplyResult(execApplied))
				if strings.TrimSpace(execApplied.ResponseAgentID) != "" {
					responseAgentID = strings.TrimSpace(execApplied.ResponseAgentID)
				}
			}
		}
		output = finalizeExecutionOutput(decision, runtime, output)
		output = applyRuntimeAgentLabel(output, responseAgentID)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, discordchat.MaxMessageLength, 1)
		deliverDiscordOutputFiles(ctx, adapter, envelope.Target, output, decision.Message.Text)
	}
}

func sendDiscordDirect(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send discord message: %v", err)
	}
}

func initTraceLogger() {
	if traceLogger != nil {
		return
	}
	maxMB := parsePositiveIntEnv("CLAWX_TRACE_LOG_MAX_MB", 20)
	maxBackups := parsePositiveIntEnv("CLAWX_TRACE_LOG_MAX_BACKUPS", 5)
	mirrorStdout := parseBoolEnv("CLAWX_TRACE_LOG_STDOUT", true)
	tracePath := strings.TrimSpace(os.Getenv("CLAWX_TRACE_LOG_FILE"))
	if tracePath == "" {
		tracePath = filepath.Join(config.StateDir(), "logs", "trace.jsonl")
	}

	writers := make([]io.Writer, 0, 2)
	if mirrorStdout {
		writers = append(writers, os.Stdout)
	}
	if strings.TrimSpace(tracePath) != "" {
		writer, err := stdlogging.NewRotatingWriter(tracePath, int64(maxMB)*1024*1024, maxBackups)
		if err != nil {
			log.Printf("trace logger rotating writer disabled: path=%s err=%v", tracePath, err)
		} else {
			writers = append(writers, writer)
		}
	}
	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}
	traceLogger = stdlogging.New(io.MultiWriter(writers...))
}

func emitTrace(event string, fields map[string]any) {
	if traceLogger == nil {
		return
	}
	payload := make(map[string]any, len(fields)+1)
	payload["event"] = strings.TrimSpace(event)
	for key, value := range fields {
		payload[key] = value
	}
	traceLogger.Info("trace", payload)
}

func parsePositiveIntEnv(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseBoolEnv(key string, fallback bool) bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func tracePreview(text string, limit int) string {
	if limit <= 0 {
		limit = 256
	}
	normalized := strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(normalized) <= limit {
		return normalized
	}
	return normalized[:limit] + "..."
}

func buildTracePromptCacheKey(decision service.Decision, runtime agentRuntime) string {
	agentID := strings.TrimSpace(runtime.agentID)
	if agentID == "" {
		agentID = "default"
	}
	projectID := strings.TrimSpace(decision.ProjectID)
	if projectID == "" {
		projectID = "main"
	}
	routeKey := strings.TrimSpace(decision.RouteKey)
	if routeKey == "" {
		routeKey = "route"
	}
	routeKey = strings.NewReplacer(":", "_", "/", "_", "\\", "_", " ", "_").Replace(routeKey)
	return strings.Join([]string{"clawx", "nl", "execute", agentID, projectID, routeKey}, ":")
}

func resolveTracePromptCacheRetention() string {
	value := strings.TrimSpace(os.Getenv("CLAWX_PROMPT_CACHE_RETENTION"))
	if value == "" {
		return "in_memory"
	}
	return value
}

func logConfigControlHandled(channel, instanceID, conversationID, userID, input, response string, err error) {
	status := "ok"
	errText := ""
	if err != nil {
		status = "error"
		errText = err.Error()
	}
	log.Printf(
		"config_control handled channel=%s instance=%s conversation_id=%s user_id=%s status=%s input_chars=%d output_chars=%d err=%q",
		strings.TrimSpace(channel),
		strings.TrimSpace(instanceID),
		strings.TrimSpace(conversationID),
		strings.TrimSpace(userID),
		status,
		len(strings.TrimSpace(input)),
		len(strings.TrimSpace(response)),
		errText,
	)
}

func handleClawXSkillMetaCommand(runtime agentRuntime, rawText string) (bool, string) {
	text := strings.TrimSpace(rawText)
	if text == "" {
		return false, ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false, ""
	}
	command := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
	if command != "clawx-skills" {
		return false, ""
	}
	if runtime.skills == nil {
		return true, "[ClawX Skill Registry]\n当前运行时未加载技能注册中心"
	}
	body := skillregistry.FormatList(runtime.skills.List())
	return true, "[ClawX Skill Registry]\n" + body
}

func normalizeAuditEventID(raw string) string {
	eventID := strings.TrimSpace(raw)
	if eventID == "" {
		return "-"
	}
	return eventID
}

func applyExecutionSourceLabel(decision service.Decision, output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return text
	}
	if decision.Kind == service.DecisionSkill {
		prefix := "[ClawX Skill]"
		if strings.HasPrefix(text, prefix) {
			return text
		}
		return prefix + "\n" + text
	}
	return text
}

func finalizeExecutionOutput(decision service.Decision, runtime agentRuntime, raw string) string {
	output := strings.TrimSpace(raw)
	if output == "" {
		output = formatTaskReceipt(taskReceiptComplete, "处理完成，无可见输出。")
	}
	output = suppressVerboseCodeBlocks(decision.Message.Text, output)
	output = applyAutonomySelfHealGate(decision, output)
	output = applyExecutionCompletionGate(decision, output)
	output = applyExecutionOwnershipFactGate(decision, runtime, output)
	return applyExecutionSourceLabel(decision, output)
}

func applyExecutionOwnershipFactGate(decision service.Decision, runtime agentRuntime, output string) string {
	request := strings.ToLower(strings.TrimSpace(decision.Message.Text))
	if !isExecutionOwnershipQuestion(request) {
		return output
	}
	backend := strings.TrimSpace(runtime.backendName)
	if backend == "" {
		backend = "unknown"
	}
	agentID := strings.TrimSpace(runtime.agentID)
	if agentID == "" {
		agentID = "main"
	}
	return fmt.Sprintf("自然语言规划由 %s 后端完成；实际命令执行由 ClawX 本机 runtime.exec 执行（agent=%s）。", backend, agentID)
}

func isExecutionOwnershipQuestion(request string) bool {
	request = strings.TrimSpace(strings.ToLower(request))
	if request == "" {
		return false
	}
	if containsAnyPhrase(request, "你现在是在codex环境里执行任务", "你现在在codex环境执行任务", "是不是在codex环境执行", "是否在codex环境执行") {
		return true
	}
	if containsAnyPhrase(request, "谁执行", "谁来执行", "执行任务是谁") &&
		containsAnyPhrase(request, "clawx", "codex", "runtime.exec", "执行命令", "本机") {
		return true
	}
	return false
}

var executionBlockerPattern = regexp.MustCompile("(?is)```(?:json)?\\s*(\\{.*?\"type\"\\s*:\\s*\"execution_blocker\".*?\\})\\s*```")

type executionBlockerPlan struct {
	Type            string   `json:"type"`
	NeedUserInput   bool     `json:"need_user_input"`
	BlockerClass    string   `json:"blocker_class"`
	Attempted       []string `json:"attempted"`
	Evidence        []string `json:"evidence"`
	EvidenceExecIDs []string `json:"evidence_exec_ids"`
	Recommendation  string   `json:"recommendation"`
	Question        string   `json:"question"`
}

func applyAutonomySelfHealGate(decision service.Decision, output string) string {
	if decision.Kind != service.DecisionExecute {
		return output
	}
	plan, ok := parseExecutionBlockerPlan(output)
	if !ok {
		return output
	}
	if !plan.NeedUserInput {
		return output
	}
	if len(normalizeNonEmptyList(plan.Attempted)) > 0 && len(normalizeNonEmptyList(plan.Evidence)) > 0 {
		execIDs := normalizeNonEmptyList(plan.EvidenceExecIDs)
		if len(execIDs) == 0 {
			return formatTaskReceipt(taskReceiptFailed,
				"我还不能向你提问：这次回复里没有附上本轮执行证据。下一步我会先在当前会话完成执行并带上证据后再给你结论。")
		}
		if !hasRuntimeExecAttestation(decision.ConversationID, execIDs) {
			return formatTaskReceipt(taskReceiptFailed,
				"我还不能向你提问：引用的执行证据不是当前会话生成的。请在当前会话重试同一任务，我会重新执行并给出可验证结果。")
		}
		return output
	}
	return formatTaskReceipt(taskReceiptFailed,
		"检测到升级提问，但缺少结构化执行证据（attempted/evidence）。请先完成自治重试并补齐证据，再请求用户决策。")
}

func parseExecutionBlockerPlan(text string) (executionBlockerPlan, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return executionBlockerPlan{}, false
	}
	candidates := make([]string, 0, 2)
	if matches := executionBlockerPattern.FindAllStringSubmatch(trimmed, -1); len(matches) > 0 {
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			candidates = append(candidates, strings.TrimSpace(match[1]))
		}
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		candidates = append(candidates, trimmed)
	}
	for _, candidate := range candidates {
		var payload map[string]any
		if err := json.Unmarshal([]byte(candidate), &payload); err != nil {
			continue
		}
		if strings.TrimSpace(strings.ToLower(toString(payload["type"]))) != "execution_blocker" {
			continue
		}
		plan := executionBlockerPlan{
			Type:            "execution_blocker",
			NeedUserInput:   toBool(payload["need_user_input"]),
			BlockerClass:    strings.TrimSpace(toString(payload["blocker_class"])),
			Attempted:       toStringSlice(payload["attempted"]),
			Evidence:        toStringSlice(payload["evidence"]),
			EvidenceExecIDs: toStringSlice(payload["evidence_exec_ids"]),
			Recommendation:  strings.TrimSpace(toString(payload["recommendation"])),
			Question:        strings.TrimSpace(toString(payload["question"])),
		}
		if evidenceMap, ok := payload["evidence"].(map[string]any); ok {
			if len(plan.Evidence) == 0 {
				plan.Evidence = toStringSlice(evidenceMap["items"])
			}
			if len(plan.Evidence) == 0 {
				plan.Evidence = toStringSlice(evidenceMap["details"])
			}
			if len(plan.EvidenceExecIDs) == 0 {
				plan.EvidenceExecIDs = toStringSlice(evidenceMap["exec_ids"])
			}
		}
		return plan, true
	}
	return executionBlockerPlan{}, false
}

func toBool(value any) bool {
	typed, ok := value.(bool)
	if !ok {
		return false
	}
	return typed
}

func toStringSlice(value any) []string {
	items, ok := value.([]any)
	if !ok || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		str := strings.TrimSpace(toString(item))
		if str == "" {
			continue
		}
		out = append(out, str)
	}
	return out
}

func toString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func normalizeNonEmptyList(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func applyRuntimeAgentLabel(output string, agentID string) string {
	text := strings.TrimSpace(output)
	agentID = strings.TrimSpace(agentID)
	if text == "" || agentID == "" || strings.EqualFold(agentID, "main") {
		return text
	}
	prefix := "[" + agentID + "-agent]:"
	if strings.HasPrefix(text, prefix) {
		return text
	}
	return prefix + "\n" + text
}

func applyExecutionCompletionGate(decision service.Decision, output string) string {
	if decision.Kind != service.DecisionExecute {
		return output
	}
	requestText := strings.ToLower(strings.TrimSpace(decision.Message.Text))
	if looksLikeSpecKitWorkflowRequest(requestText) {
		setSpecKitFlowActive(decision.ConversationID, true)
		return output
	}
	if isSpecKitFlowActive(decision.ConversationID) && looksLikeSpecKitContinuationRequest(requestText) {
		return output
	}
	if !looksLikeImplementationRequest(requestText) {
		return output
	}
	answerText := strings.ToLower(strings.TrimSpace(output))
	if !looksLikeCompletionClaim(answerText) {
		return output
	}
	if hasExecutionEvidence(output, answerText) {
		return output
	}
	return "执行结果未通过平台验收门禁：检测到“已实现/已完成”声明，但缺少可核验证据。\n" +
		"请补充以下至少两项后再回复“已完成”：\n" +
		"1. 实际执行过的命令（如 go test/go run 等）\n" +
		"2. 命令结果摘要（通过/失败、关键输出）\n" +
		"3. 代码变更清单（文件路径）"
}

func looksLikeSpecKitContinuationRequest(text string) bool {
	return containsAnyPhrase(text, "继续补齐", "补齐", "继续", "继续完善", "继续生成", "继续出文档")
}

func looksLikeSpecKitWorkflowRequest(text string) bool {
	if text == "" {
		return false
	}
	if containsAnyPhrase(text,
		"spec kit", "speckit", "/speckit.", "speckit.",
		"生成规范", "先出规范", "只出规范", "不要直接写代码",
		"specify", "plan.md", "tasks.md", "analyze.md",
	) {
		return true
	}
	// "规范 + 文档" should be treated as spec workflow, not completion claim.
	return containsAnyPhrase(text, "规范") && containsAnyPhrase(text, "文档", "spec")
}

func looksLikeImplementationRequest(text string) bool {
	if text == "" {
		return false
	}
	return containsAnyPhrase(text, "实现", "开发", "脚本", "代码", "修复", "补齐", "新增", "通知", "自动化", "定时")
}

func looksLikeCompletionClaim(text string) bool {
	return containsAnyPhrase(text,
		"已实现", "实现完成", "已完成", "已帮你", "完成了", "完成实现",
		"implemented", "implementation completed", "completed", "done",
	)
}

func hasExecutionEvidence(output, lower string) bool {
	hasCommand := executionEvidenceCommandPattern.MatchString(output) || strings.Contains(lower, "```bash")
	hasResult := containsAnyPhrase(lower, "测试结果", "结果：", "结果:", "result:", "通过", "失败", "ok ", "exit code")
	hasChanges := strings.Contains(output, "](/") || containsAnyPhrase(lower, "新增", "更新", "修改", "变更文件")
	return hasCommand && (hasResult || hasChanges)
}

func containsAnyPhrase(text string, keywords ...string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}
	for _, keyword := range keywords {
		keyword = strings.ToLower(strings.TrimSpace(keyword))
		if keyword != "" && strings.Contains(text, keyword) {
			return true
		}
	}
	return false
}

func buildExecutionInput(decision service.Decision, runtime agentRuntime) string {
	if decision.Kind != service.DecisionSkill || decision.Skill == nil {
		if decision.Kind == service.DecisionExecute {
			input := buildNaturalLanguageExecutionInput(decision, runtime)
			return appendAttachmentContext(input, decision.Message.Attachments)
		}
		return appendAttachmentContext(decision.Message.Text, decision.Message.Attachments)
	}

	userInput := strings.TrimSpace(decision.SkillInput)
	if userInput == "" {
		userInput = strings.TrimSpace(decision.Message.Text)
	}

	var builder strings.Builder
	builder.WriteString("你正在执行一个已选中的 Skill。\n")
	builder.WriteString("Skill: ")
	builder.WriteString(decision.Skill.Name)
	builder.WriteString("\n\n[Skill Instructions]\n")
	builder.WriteString(strings.TrimSpace(decision.Skill.InstructionBody))
	builder.WriteString("\n\n[User Request]\n")
	builder.WriteString(userInput)
	return appendAttachmentContext(builder.String(), decision.Message.Attachments)
}

type agentInventorySnapshot struct {
	source         string
	loadedAt       time.Time
	defaultAgentID string
	activeAgentID  string
	agents         []config.Agent
}

func buildNaturalLanguageExecutionInput(decision service.Decision, runtime agentRuntime) string {
	request := strings.TrimSpace(decision.Message.Text)
	inventory := loadAgentInventorySnapshot(runtime)
	skills := listRuntimeSkills(runtime)

	var builder strings.Builder
	builder.WriteString("[ClawX Capability Context]\n")
	builder.WriteString("message_mode=natural_language\n")
	builder.WriteString("routing_policy=slash_command_only_for_/prefix\n")
	builder.WriteString("current_runtime_agent=")
	builder.WriteString(fallbackValue(runtime.agentID, "main"))
	builder.WriteString("\n")
	builder.WriteString("current_backend=")
	builder.WriteString(fallbackValue(runtime.backendName, "unknown"))
	builder.WriteString("\n\n")

	if staged := buildStagedRoutingSnapshot(decision, runtime, skills); staged != "" {
		builder.WriteString("[Staged Routing Snapshot]\n")
		builder.WriteString(staged)
		builder.WriteString("\n\n")
	}

	builder.WriteString("[Available Tools]\n")
	builder.WriteString("- tool.agent_inventory: 读取全局智能体配置并回答数量/默认智能体/工作区等问题。\n")
	builder.WriteString("- tool.skill_catalog: 读取当前运行时可用技能与来源。\n")
	builder.WriteString("- tool.execute_runtime: 对需要操作文件/命令的请求执行实际动作。\n\n")

	builder.WriteString("[Policy]\n")
	builder.WriteString("- 非 / 开头消息必须按自然语言处理，不走硬编码命令规则。\n")
	builder.WriteString("- 若用户在问“为什么/怎么回事/原因是什么”等问句，优先解释原因并给证据；不要无条件转成执行动作。\n")
	builder.WriteString("- 涉及“数量/状态/配置”提问时，优先使用下方快照，不要基于当前会话猜测。\n")
	builder.WriteString("- 涉及控制面变更、需求更新、命令执行等动作时，统一输出 action_plan 由平台执行。\n")
	builder.WriteString("- 当用户输入单条明确命令时，action_plan 默认只放该命令；不要自动追加“第二条验证命令”，除非用户明确要求验证/检查。\n")
	builder.WriteString("- 调试本地 HTTP 接口（如 127.0.0.1/localhost）前，先确认服务已启动且健康，再发业务请求。\n")
	builder.WriteString("- 遇到执行失败时，先自治恢复再升级提问：必须先做诊断与重试，禁止第一轮直接向用户索要环境参数/镜像/离线包。\n")
	builder.WriteString("- 只有当自动恢复已穷尽时才可提问，并附上已尝试命令、关键报错、推荐下一步。\n")
	builder.WriteString("- 信息不足时明确说明缺口，禁止编造。\n\n")

	builder.WriteString("[Autonomous Recovery Playbook]\n")
	builder.WriteString("- 先诊断：检查网络/DNS/权限/路径，给出可复现命令与结果摘要。\n")
	builder.WriteString("- 再重试：对可恢复错误做有限重试，并记录每次尝试。\n")
	builder.WriteString("- 依赖安装失败时默认执行：官方源重试 -> 常见镜像回退 -> 本地离线源探测（若存在）。\n")
	builder.WriteString("- pip 典型回退顺序：pypi.org/simple、清华/阿里/腾讯镜像（按可达性选择），并保留失败证据。\n")
	builder.WriteString("- 若仍失败，再升级提问；提问必须包含“已尝试步骤 + 失败原因 + 推荐动作”。\n\n")

	builder.WriteString("[Execution Blocker Contract]\n")
	builder.WriteString("- 仅在确实需要用户决策时输出 execution_blocker。\n")
	builder.WriteString("- execution_blocker 必须包含 attempted、evidence、evidence_exec_ids；否则平台不会放行升级提问。\n")
	builder.WriteString("- schema:\n")
	builder.WriteString("  {\n")
	builder.WriteString("    \"type\": \"execution_blocker\",\n")
	builder.WriteString("    \"need_user_input\": true,\n")
	builder.WriteString("    \"blocker_class\": \"network|permission|auth|resource|tool|unknown\",\n")
	builder.WriteString("    \"attempted\": [\"已尝试步骤1\", \"已尝试步骤2\"],\n")
	builder.WriteString("    \"evidence\": [\"命令与关键输出\", \"错误摘要\"],\n")
	builder.WriteString("    \"evidence_exec_ids\": [\"rexec-...\", \"rexec-...\"],\n")
	builder.WriteString("    \"recommendation\": \"推荐下一步\",\n")
	builder.WriteString("    \"question\": \"需要用户确认的问题\"\n")
	builder.WriteString("  }\n\n")

	builder.WriteString("[Unified Action Plan Contract]\n")
	builder.WriteString("- 当你判断需要执行动作时，优先输出 action_plan（统一协议），由 ClawX 原生执行器落地。\n")
	builder.WriteString("- 用户表达“启动/初始化/开工”且目标是拉起运行时时，优先使用 runtime.bootstrap，不要直接给大段 shell。\n")
	builder.WriteString("- 当前支持 action.kind: runtime.exec（shell 命令）、runtime.bootstrap（运行时初始化）、agent.use（切换智能体）、requirement.sync（需求落盘）、config.exec（配置面指令）。\n")
	builder.WriteString("- 优先输出纯 JSON；如必须附带解释，将 JSON 放在 ```json 代码块中。\n")
	builder.WriteString("- action_plan schema:\n")
	builder.WriteString("  {\n")
	builder.WriteString("    \"type\": \"action_plan\",\n")
	builder.WriteString("    \"mode\": \"execute|suggest\",\n")
	builder.WriteString("    \"reason\": \"可选；计划说明\",\n")
	builder.WriteString("    \"actions\": [\n")
	builder.WriteString("      {\"kind\":\"runtime.exec\", \"cmd\": \"python3 -m pip install -r requirements.txt\", \"cwd\": \"可选\", \"reason\": \"可选\"}\n")
	builder.WriteString("      {\"kind\":\"runtime.bootstrap\", \"cwd\": \"workspace路径\", \"worker_roles\": [\"planner\",\"executor\",\"reviewer\"]}\n")
	builder.WriteString("      {\"kind\":\"agent.use\", \"agent_id\": \"bid-all\", \"reason\": \"可选\"}\n")
	builder.WriteString("      {\"kind\":\"requirement.sync\", \"mode\":\"execute\", \"agent_id\":\"bid-all\", \"requirement\":\"需求内容\"}\n")
	builder.WriteString("      {\"kind\":\"config.exec\", \"command\":\"/config plan 创建 agent bid-all 使用 codex\"}\n")
	builder.WriteString("    ]\n")
	builder.WriteString("  }\n\n")
	builder.WriteString("- 旧版 control_plan / requirement_sync / runtime_exec_plan 已废弃，不要输出。\n\n")

	builder.WriteString("[Tool Snapshot: agent_inventory]\n")
	builder.WriteString("source=")
	builder.WriteString(inventory.source)
	builder.WriteString(" loaded_at=")
	builder.WriteString(inventory.loadedAt.Format(time.RFC3339))
	builder.WriteString(" total_registered_agents=")
	builder.WriteString(strconv.Itoa(len(inventory.agents)))
	builder.WriteString("\n")
	builder.WriteString("default_agent=")
	builder.WriteString(fallbackValue(inventory.defaultAgentID, "-"))
	builder.WriteString(" active_agent=")
	builder.WriteString(fallbackValue(inventory.activeAgentID, "-"))
	builder.WriteString("\n")
	for _, agent := range inventory.agents {
		builder.WriteString("- id=")
		builder.WriteString(agent.ID)
		builder.WriteString(" profile=")
		builder.WriteString(fallbackValue(agent.ProfileID, "-"))
		builder.WriteString(" workspace=")
		builder.WriteString(fallbackValue(agent.Workspace, "-"))
		builder.WriteString(" timeout_seconds=")
		builder.WriteString(strconv.Itoa(int(agent.Timeout.Seconds())))
		builder.WriteString(" default=")
		builder.WriteString(strconv.FormatBool(agent.IsDefault))
		builder.WriteByte('\n')
	}

	builder.WriteString("\n[Tool Snapshot: skill_catalog]\n")
	builder.WriteString("total_active_skills=")
	builder.WriteString(strconv.Itoa(len(skills)))
	builder.WriteByte('\n')
	limit := len(skills)
	if limit > 12 {
		limit = 12
	}
	for idx := 0; idx < limit; idx++ {
		def := skills[idx]
		builder.WriteString("- name=")
		builder.WriteString(def.Name)
		builder.WriteString(" source=")
		builder.WriteString(string(def.Source))
		builder.WriteString(" aliases=")
		builder.WriteString(strings.Join(def.Aliases, ","))
		builder.WriteString(" description=")
		builder.WriteString(oneLine(def.Description))
		builder.WriteByte('\n')
	}
	if len(skills) > limit {
		builder.WriteString("- omitted_skills=")
		builder.WriteString(strconv.Itoa(len(skills) - limit))
		builder.WriteByte('\n')
	}

	if continuation := buildExecutionContinuationSnapshot(decision.ConversationID, request); continuation != "" {
		builder.WriteString("\n[Execution Continuation Snapshot]\n")
		builder.WriteString(continuation)
		builder.WriteString("\n")
	}

	builder.WriteString("\n[User Request]\n")
	builder.WriteString(request)
	return strings.TrimSpace(builder.String())
}

func buildExecutionContinuationSnapshot(conversationID string, request string) string {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return ""
	}
	records := listRecentRuntimeExecAttestations(conversationID, 4)
	if len(records) == 0 {
		if goal, ok := getExecutionGoalState(conversationID); ok {
			return strings.TrimSpace("goal=" + fallbackValue(goal.Goal, "-") + "\n" +
				"goal_status=" + fallbackValue(goal.Status, "-") + "\n" +
				"goal_last_result=" + fallbackValue(oneLine(goal.LastResult), "-"))
		}
		return ""
	}
	var builder strings.Builder
	if goal, ok := getExecutionGoalState(conversationID); ok {
		builder.WriteString("goal=")
		builder.WriteString(fallbackValue(goal.Goal, "-"))
		builder.WriteByte('\n')
		builder.WriteString("goal_status=")
		builder.WriteString(fallbackValue(goal.Status, "-"))
		builder.WriteByte('\n')
		if strings.TrimSpace(goal.LastResult) != "" {
			builder.WriteString("goal_last_result=")
			builder.WriteString(oneLine(goal.LastResult))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("recent_exec_count=")
	builder.WriteString(strconv.Itoa(len(records)))
	builder.WriteByte('\n')
	last := records[len(records)-1]
	builder.WriteString("last_exec_id=")
	builder.WriteString(strings.TrimSpace(last.ExecID))
	builder.WriteByte('\n')
	builder.WriteString("last_exec_success=")
	builder.WriteString(strconv.FormatBool(last.Success))
	builder.WriteByte('\n')
	if !last.Success {
		builder.WriteString("last_error=")
		builder.WriteString(fallbackValue(oneLine(last.ErrorSummary), oneLine(last.OutputPreview)))
		builder.WriteByte('\n')
	}
	for _, rec := range records {
		builder.WriteString("- exec_id=")
		builder.WriteString(strings.TrimSpace(rec.ExecID))
		builder.WriteString(" success=")
		builder.WriteString(strconv.FormatBool(rec.Success))
		builder.WriteString(" cmd=")
		builder.WriteString(oneLine(summarizeText(rec.Command, 120)))
		builder.WriteString(" preview=")
		builder.WriteString(oneLine(summarizeText(rec.OutputPreview, 120)))
		builder.WriteByte('\n')
	}
	if isContinuationLikeRequest(request) {
		builder.WriteString("continuation_hint=advance_from_recent_runtime_exec\n")
		builder.WriteString("continuation_rule=如果用户只说“继续”，请基于最近失败点推进，不要重复最近已成功命令\n")
	}
	return strings.TrimSpace(builder.String())
}

func isContinuationLikeRequest(text string) bool {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return false
	}
	return containsAnyPhrase(text, "继续", "接着", "下一步", "继续执行", "继续跑", "go on")
}

func buildStagedRoutingSnapshot(decision service.Decision, runtime agentRuntime, skills []skilldomain.Definition) string {
	hints := make([]string, 0, 3)
	for idx := 0; idx < len(skills) && idx < 3; idx++ {
		name := strings.TrimSpace(skills[idx].Name)
		if name != "" {
			hints = append(hints, "skill."+name)
		}
	}

	result := skillorchestrator.NewStagedRouter().Plan(skillorchestrator.StagedRoutingInput{
		Message:          strings.TrimSpace(decision.Message.Text),
		CurrentAgentID:   strings.TrimSpace(runtime.agentID),
		ProjectID:        strings.TrimSpace(decision.ProjectID),
		SkillCatalogSize: len(skills),
		TopSkillHints:    hints,
	})
	intent := result.Intent.Normalize()
	route := result.Route.Normalize()
	return strings.Join([]string{
		"intent.type=" + intent.Type,
		"intent.value=" + fallbackValue(intent.Intent, "-"),
		"intent.route=" + fallbackValue(intent.Route, "-"),
		"intent.risk=" + fallbackValue(intent.Risk, "-"),
		"intent.complexity=" + fallbackValue(intent.Complexity, "-"),
		"route.type=" + route.Type,
		"route.value=" + fallbackValue(route.Route, "-"),
		"route.use_route_planner=" + strconv.FormatBool(route.UseRoutePlanner),
		"route.skip_route_planner=" + strconv.FormatBool(result.SkipRoutePlanner),
		"route.context_budget_tier=" + fallbackValue(route.ContextBudgetTier, "-"),
		"route.tool_hints=" + strings.Join(route.ToolHints, ","),
		"fallback.enabled=" + strconv.FormatBool(result.Fallback.Enabled),
		"fallback.mode=" + fallbackValue(result.Fallback.Mode, "-"),
		"fallback.reason=" + fallbackValue(result.Fallback.Reason, "-"),
		"execution.can_execute=" + strconv.FormatBool(result.CanExecute),
	}, "\n")
}

func loadAgentInventorySnapshot(runtime agentRuntime) agentInventorySnapshot {
	snapshot := runtime.cfgSnapshot
	source := "runtime_snapshot"
	if loaded, err := config.Load(); err == nil {
		snapshot = loaded
		source = "config_file"
	} else {
		log.Printf("load agent inventory from config failed, fallback runtime snapshot: %v", err)
	}

	defaultAgent := strings.TrimSpace(snapshot.DefaultAgentID)
	if defaultAgent == "" && snapshot.ActiveAgent != nil {
		defaultAgent = strings.TrimSpace(snapshot.ActiveAgent.ID)
	}
	activeAgent := ""
	if snapshot.ActiveAgent != nil {
		activeAgent = strings.TrimSpace(snapshot.ActiveAgent.ID)
	}

	ids := sortedAgentIDs(snapshot.Agents)
	agents := make([]config.Agent, 0, len(ids))
	for _, id := range ids {
		agent := snapshot.Agents[id]
		if strings.TrimSpace(agent.ID) == "" {
			agent.ID = id
		}
		agent.Workspace = strings.TrimSpace(agent.Workspace)
		agent.ProfileID = strings.TrimSpace(agent.ProfileID)
		agent.IsDefault = strings.TrimSpace(agent.ID) == defaultAgent
		agents = append(agents, agent)
	}

	return agentInventorySnapshot{
		source:         source,
		loadedAt:       time.Now().UTC(),
		defaultAgentID: defaultAgent,
		activeAgentID:  activeAgent,
		agents:         agents,
	}
}

func listRuntimeSkills(runtime agentRuntime) []skilldomain.Definition {
	if runtime.skills == nil {
		return nil
	}
	entries := runtime.skills.List()
	items := make([]skilldomain.Definition, 0, len(entries))
	for _, entry := range entries {
		if entry.Status != skilldomain.StatusActive || entry.Definition == nil {
			continue
		}
		items = append(items, *entry.Definition)
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].Name)
		right := strings.TrimSpace(items[j].Name)
		if left == right {
			return strings.TrimSpace(items[i].Description) < strings.TrimSpace(items[j].Description)
		}
		return left < right
	})
	return items
}

func oneLine(raw string) string {
	text := strings.TrimSpace(raw)
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 120 {
		return text[:120] + "..."
	}
	return text
}

func fallbackValue(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func appendAttachmentContext(input string, attachments []chatiface.Attachment) string {
	text := strings.TrimSpace(input)
	if len(attachments) == 0 {
		return text
	}

	var builder strings.Builder
	if text != "" {
		builder.WriteString(text)
		builder.WriteString("\n\n")
	}
	builder.WriteString("[Attachments]\n")
	for idx, attachment := range attachments {
		name := strings.TrimSpace(attachment.Name)
		if name == "" {
			name = fmt.Sprintf("attachment_%d", idx+1)
		}
		builder.WriteString("- ")
		builder.WriteString(name)
		if ct := strings.TrimSpace(attachment.ContentType); ct != "" {
			builder.WriteString(" (")
			builder.WriteString(ct)
			builder.WriteString(")")
		}
		if attachment.SizeBytes > 0 {
			builder.WriteString(fmt.Sprintf(" size=%dB", attachment.SizeBytes))
		}
		if local := strings.TrimSpace(attachment.LocalPath); local != "" {
			builder.WriteString(" local_path=")
			builder.WriteString(local)
		}
		if url := strings.TrimSpace(attachment.URL); url != "" {
			builder.WriteString(" url=")
			builder.WriteString(url)
		}
		builder.WriteByte('\n')
	}
	return strings.TrimSpace(builder.String())
}

func startDiscordTypingLoop(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target) func() {
	if adapter == nil || strings.TrimSpace(target.ChannelID) == "" {
		return func() {}
	}

	typingCtx, cancel := context.WithCancel(ctx)
	go func() {
		if err := adapter.SendTyping(typingCtx, target); err != nil {
			log.Printf("send discord typing: %v", err)
		}

		ticker := time.NewTicker(8 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-typingCtx.Done():
				return
			case <-ticker.C:
				if err := adapter.SendTyping(typingCtx, target); err != nil {
					log.Printf("send discord typing: %v", err)
				}
			}
		}
	}()

	return cancel
}

func toControlResponse(result service.ControlFlowResult) chatiface.ControlResponse {
	response := chatiface.ControlResponse{
		Message:            result.Message,
		CreatedSessionID:   result.CreatedSessionID,
		ResumedSessionID:   result.ResumedSessionID,
		SwitchedSessionID:  result.SwitchedSessionID,
		CancelledSessionID: result.CancelledSessionID,
		CancelNoop:         result.CancelNoop,
		Sessions:           make([]chatiface.ControlSessionSummary, 0, len(result.Sessions)),
		CurrentChecked:     result.CurrentChecked,
	}
	if result.CurrentSession != nil {
		response.CurrentSessionID = result.CurrentSession.ID
		response.CurrentStatus = string(result.CurrentSession.Status)
	}

	for _, summary := range result.Sessions {
		response.Sessions = append(response.Sessions, chatiface.ControlSessionSummary{
			ID:     summary.ID,
			Status: string(summary.Status),
		})
	}
	return response
}

var (
	markdownFilePathPattern = regexp.MustCompile(`\[[^\]]+\]\((/[^)\n]+)\)`)
	absolutePathPattern     = regexp.MustCompile(`(?m)(/[^\s\]\)\(<>\"` + "`" + `]+)`)
	codeFencePattern        = regexp.MustCompile("(?s)```[a-zA-Z0-9_+-]*\\n.*?```")
)

func deliverDiscordOutputFiles(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target, output string, userText string) {
	if adapter == nil {
		return
	}
	if !shouldDeliverOutputFiles(userText) {
		return
	}
	paths := extractOutputLocalFiles(output, 3)
	for _, path := range paths {
		caption := "产物文件: " + filepath.Base(path)
		if err := adapter.SendLocalFile(ctx, target, path, caption); err != nil {
			log.Printf("discord send local file failed: path=%s err=%v", path, err)
			sendDiscordDirect(ctx, adapter, target, fmt.Sprintf("文件回传失败: %s (%v)", filepath.Base(path), err))
		}
	}
}

func deliverTelegramOutputFiles(ctx context.Context, adapter *telegramchat.Adapter, target telegramchat.Target, output string, userText string) {
	if adapter == nil {
		return
	}
	if !shouldDeliverOutputFiles(userText) {
		return
	}
	paths := extractOutputLocalFiles(output, 3)
	for _, path := range paths {
		caption := "产物文件: " + filepath.Base(path)
		if err := adapter.SendLocalFile(ctx, target, path, caption); err != nil {
			log.Printf("telegram send local file failed: path=%s err=%v", path, err)
			sendTelegramDirect(ctx, adapter, target, fmt.Sprintf("文件回传失败: %s (%v)", filepath.Base(path), err))
		}
	}
}

func deliverWeComOutputFiles(ctx context.Context, adapter *wecomchat.Adapter, target wecomchat.Target, output string, userText string) {
	if adapter == nil {
		return
	}
	if !shouldDeliverOutputFiles(userText) {
		return
	}
	paths := extractOutputLocalFiles(output, 3)
	for _, path := range paths {
		caption := "产物文件: " + filepath.Base(path)
		if err := adapter.SendLocalFile(ctx, target, path, caption); err != nil {
			log.Printf("wecom send local file failed: path=%s err=%v", path, err)
			sendWeComDirect(ctx, adapter, target, fmt.Sprintf("文件回传失败: %s (%v)", filepath.Base(path), err))
		}
	}
}

func extractOutputLocalFiles(output string, max int) []string {
	if max <= 0 {
		return nil
	}
	seen := make(map[string]struct{}, max)
	result := make([]string, 0, max)
	appendPath := func(raw string) {
		if len(result) >= max {
			return
		}
		path := sanitizeLocalPathToken(raw)
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			return
		}
		info, err := os.Stat(clean)
		if err != nil || info.IsDir() {
			return
		}
		seen[clean] = struct{}{}
		result = append(result, clean)
	}

	for _, match := range markdownFilePathPattern.FindAllStringSubmatch(output, -1) {
		if len(match) < 2 {
			continue
		}
		appendPath(match[1])
	}
	for _, match := range absolutePathPattern.FindAllStringSubmatch(output, -1) {
		if len(match) < 2 {
			continue
		}
		appendPath(match[1])
	}
	return result
}

func shouldDeliverOutputFiles(userText string) bool {
	text := strings.ToLower(strings.TrimSpace(userText))
	if text == "" {
		return false
	}
	if containsAnyPhrase(text,
		"不要贴代码", "不需要代码", "无需代码", "只要路径", "仅路径", "不要附件", "无需附件",
		"no code", "no attachment", "without code",
	) {
		return false
	}
	return containsAnyPhrase(text,
		"贴代码", "发代码", "源码", "代码文件", "产物文件", "发我文件", "下载附件", "上传附件",
		"show code", "source code", "send file", "attach file",
	)
}

func suppressVerboseCodeBlocks(userText string, output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return text
	}
	if shouldDeliverOutputFiles(userText) {
		return text
	}
	if !codeFencePattern.MatchString(text) {
		return text
	}
	return strings.TrimSpace(codeFencePattern.ReplaceAllString(text, "（代码内容已省略；如需查看请回复：贴代码）"))
}

func sanitizeLocalPathToken(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.Trim(value, "`'\"")
	value = strings.TrimRight(value, ".,;:!?)")
	if value == "" || !strings.HasPrefix(value, "/") {
		return ""
	}
	// Reject URLs to avoid treating https://... as file path.
	if strings.HasPrefix(strings.ToLower(value), "/http://") || strings.HasPrefix(strings.ToLower(value), "/https://") {
		return ""
	}
	return value
}
