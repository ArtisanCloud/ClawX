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

var executionEvidenceCommandPattern = regexp.MustCompile(`(?m)(^|\n)\s*(go\s+test|go\s+run|npm\s+run|pnpm\s+run|yarn\s+|pytest|cargo\s+test|make\s+test|bash\s+|sh\s+|uv\s+run|curl\s+|systemctl\s+)`)

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
	fmt.Fprintf(os.Stdout, "Usage: clawx [serve|run|config|skill|install-service|setup-service|help]\n")
	fmt.Fprintf(os.Stdout, "  serve  Start the service. If config.json is missing, bootstrap it first.\n")
	fmt.Fprintf(os.Stdout, "  run    Start selected runtime target (serve|telegram|discord|feishu|wecom); defaults to config service.run.\n")
	fmt.Fprintf(os.Stdout, "  config Launch the interactive config wizard, or run `config agent ...` for agent management.\n")
	fmt.Fprintf(os.Stdout, "  skill  Manage skill registry (list|reload|enable|disable).\n")
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
	projectService, err := newProjectCommandService(cfg)
	if err != nil {
		return nil, "", nil, fmt.Errorf("init project service: %w", err)
	}
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
				service.WithProjectResolver(projectService),
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
	if cfg.ActiveProfile != nil {
		if strings.TrimSpace(cfg.ActiveProfile.Kind) != "" {
			profileKind = cfg.ActiveProfile.Kind
		}
		if strings.TrimSpace(cfg.ActiveProfile.Command) != "" {
			profileCmd = cfg.ActiveProfile.Command
		}
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
				service.WithProjectResolver(projectService),
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
			service.WithProjectResolver(projectService),
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
		return defaultCWD
	}
	record, err := runtime.projectCWD.GetProject(ctx, projectID)
	if err != nil {
		log.Printf("resolve execute cwd fallback: agent=%s project_id=%s err=%v", runtime.agentID, projectID, err)
		return defaultCWD
	}
	projectWorkspace := strings.TrimSpace(record.WorkspacePath)
	if projectWorkspace == "" {
		return defaultCWD
	}
	agentID := normalizeExecutionDirSegment(runtime.agentID)
	if agentID == "" {
		return projectWorkspace
	}
	agentWorkspace := filepath.Join(projectWorkspace, ".agents", agentID, "workspace")
	if err := os.MkdirAll(agentWorkspace, 0o755); err != nil {
		log.Printf("resolve execute cwd agent workspace fallback: agent=%s project_id=%s path=%s err=%v", runtime.agentID, projectID, agentWorkspace, err)
		return projectWorkspace
	}
	return agentWorkspace
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
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
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
	eventID := normalizeAuditEventID(envelope.EventID)
	sendTelegramDirect(ctx, adapter, envelope.Target, "正在思考...")

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
		log.Printf("telegram execute begin: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
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
		})
		if err != nil {
			log.Printf("telegram execute failed: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("telegram execute done: channel=telegram instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if applied, ok, autoErr := maybeAutoApplyAgentSwitch(decision.Message.Text, output, scopeKey, overrides, runtimes, defaultRuntimeID); autoErr != nil {
			log.Printf("telegram auto apply agent switch failed: channel=telegram instance=%s scope=%s err=%v", instanceID, scopeKey, autoErr)
		} else if ok {
			output = output + "\n\n" + formatControlApplyResult(applied)
		}
		output = finalizeExecutionOutput(decision, output)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, telegramchat.MaxMessageLength, 1)
		deliverTelegramOutputFiles(ctx, adapter, envelope.Target, output)
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
			sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
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
	eventID := normalizeAuditEventID(envelope.EventID)
	sendFeishuDirect(ctx, adapter, envelope.Target, "正在思考...")

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
		log.Printf("feishu execute begin: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
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
		})
		if err != nil {
			log.Printf("feishu execute failed: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendFeishuDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("feishu execute done: channel=feishu instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if applied, ok, autoErr := maybeAutoApplyAgentSwitch(decision.Message.Text, output, scopeKey, overrides, runtimes, defaultRuntimeID); autoErr != nil {
			log.Printf("feishu auto apply agent switch failed: channel=feishu instance=%s scope=%s err=%v", instanceID, scopeKey, autoErr)
		} else if ok {
			output = output + "\n\n" + formatControlApplyResult(applied)
		}
		output = finalizeExecutionOutput(decision, output)
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
			sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
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
	eventID := normalizeAuditEventID(envelope.EventID)
	sendWeComDirect(ctx, adapter, envelope.Target, "正在思考...")

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
		log.Printf("wecom execute begin: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
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
		})
		if err != nil {
			log.Printf("wecom execute failed: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendWeComDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("wecom execute done: channel=wecom instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, eventID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if applied, ok, autoErr := maybeAutoApplyAgentSwitch(decision.Message.Text, output, scopeKey, overrides, runtimes, defaultRuntimeID); autoErr != nil {
			log.Printf("wecom auto apply agent switch failed: channel=wecom instance=%s scope=%s err=%v", instanceID, scopeKey, autoErr)
		} else if ok {
			output = output + "\n\n" + formatControlApplyResult(applied)
		}
		output = finalizeExecutionOutput(decision, output)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, wecomchat.MaxMessageLength, 1)
		deliverWeComOutputFiles(ctx, adapter, envelope.Target, output)
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
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
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
		log.Printf("discord execute begin: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, executeCWD, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		stopTyping := startDiscordTypingLoop(ctx, adapter, envelope.Target)
		defer stopTyping()
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
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
		})
		if err != nil {
			log.Printf("discord execute failed: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("discord execute done: channel=discord instance=%s event_id=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s project_id=%s project_mode=%s session_id=%s backend_session_id=%s state=%s memory_scope=%q memory_acl_mode=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, "-", runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.ProjectID, decision.ProjectMode, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, flowResult.Execution.MemoryScope, flowResult.Execution.MemoryACLMode, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if applied, ok, autoErr := maybeAutoApplyAgentSwitch(decision.Message.Text, output, scopeKey, overrides, runtimes, defaultRuntimeID); autoErr != nil {
			log.Printf("discord auto apply agent switch failed: channel=discord instance=%s scope=%s err=%v", instanceID, scopeKey, autoErr)
		} else if ok {
			output = output + "\n\n" + formatControlApplyResult(applied)
		}
		output = finalizeExecutionOutput(decision, output)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, discordchat.MaxMessageLength, 1)
		deliverDiscordOutputFiles(ctx, adapter, envelope.Target, output)
	}
}

func sendDiscordDirect(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send discord message: %v", err)
	}
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
	prefix := "[Agent Direct]"
	if strings.HasPrefix(text, prefix) {
		return text
	}
	return prefix + "\n" + text
}

func finalizeExecutionOutput(decision service.Decision, raw string) string {
	output := strings.TrimSpace(raw)
	if output == "" {
		output = "执行完成，无可见输出"
	}
	output = applyExecutionCompletionGate(decision, output)
	return applyExecutionSourceLabel(decision, output)
}

func applyExecutionCompletionGate(decision service.Decision, output string) string {
	if decision.Kind != service.DecisionExecute {
		return output
	}
	requestText := strings.ToLower(strings.TrimSpace(decision.Message.Text))
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
	hasResult := containsAnyPhrase(lower, "测试结果", "结果：", "result:", "通过", "失败", "ok ", "exit code")
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

	builder.WriteString("[Available Tools]\n")
	builder.WriteString("- tool.agent_inventory: 读取全局智能体配置并回答数量/默认智能体/工作区等问题。\n")
	builder.WriteString("- tool.skill_catalog: 读取当前运行时可用技能与来源。\n")
	builder.WriteString("- tool.execute_runtime: 对需要操作文件/命令的请求执行实际动作。\n\n")

	builder.WriteString("[Policy]\n")
	builder.WriteString("- 非 / 开头消息必须按自然语言处理，不走硬编码命令规则。\n")
	builder.WriteString("- 涉及“数量/状态/配置”提问时，优先使用下方快照，不要基于当前会话猜测。\n")
	builder.WriteString("- 信息不足时明确说明缺口，禁止编造。\n\n")

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

	builder.WriteString("\n[User Request]\n")
	builder.WriteString(request)
	return strings.TrimSpace(builder.String())
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
)

func deliverDiscordOutputFiles(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target, output string) {
	if adapter == nil {
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

func deliverTelegramOutputFiles(ctx context.Context, adapter *telegramchat.Adapter, target telegramchat.Target, output string) {
	if adapter == nil {
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

func deliverWeComOutputFiles(ctx context.Context, adapter *wecomchat.Adapter, target wecomchat.Target, output string) {
	if adapter == nil {
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
