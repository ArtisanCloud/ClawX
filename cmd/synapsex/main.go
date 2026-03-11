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
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"synapsex/internal/application/command"
	"synapsex/internal/application/service"
	"synapsex/internal/application/skillregistry"
	sessiondomain "synapsex/internal/domain/session"
	"synapsex/internal/infrastructure/backend"
	"synapsex/internal/infrastructure/config"
	"synapsex/internal/infrastructure/health"
	"synapsex/internal/infrastructure/persistence"
	adminiface "synapsex/internal/interfaces/admin"
	chatiface "synapsex/internal/interfaces/chat"
	discordchat "synapsex/internal/interfaces/chat/discord"
	telegramchat "synapsex/internal/interfaces/chat/telegram"
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
	agentID     string
	backendName string
	cwd         string
	profileKind string
	profileCmd  string
	router      *service.Router
	runner      *backend.DirectRunner
	skills      *skillregistry.Service
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("%v", err)
	}
}

func run(args []string) error {
	switch firstArg(args) {
	case "", "serve":
		return runServe()
	case "config":
		return runConfigEntry(args[1:])
	case "skill":
		return runSkillCommand(args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", firstArg(args))
	}
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

	store, closeStore, err := newSessionStore(cfg)
	if err != nil {
		return fmt.Errorf("init session store: %w", err)
	}
	defer closeStore()

	sessionManager := service.NewSessionManager(store, store, nil)
	runtimes, defaultRuntimeID, err := buildAgentRuntimes(cfg, sessionManager)
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
			handleTelegramInbound(messageCtx, runtime, delivery, telegramAdapter, envelope, scopedConversationID, instanceCopy.ID)
			return nil
		}

		mode := strings.ToLower(strings.TrimSpace(instanceCopy.Mode))
		if mode == "" {
			mode = "polling"
		}
		if mode == "webhook" {
			httpRuntimeNeeded = true
			path := normalizeWebhookRoutePath(instanceCopy.WebhookPath, instanceCopy.ID)
			if _, exists := webhookPaths[path]; exists {
				return fmt.Errorf("duplicate telegram webhook path %q", path)
			}
			webhookPaths[path] = struct{}{}

			httpMux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					w.WriteHeader(http.StatusMethodNotAllowed)
					return
				}
				envelope, ok, err := telegramAdapter.ParseWebhookRequest(r)
				if err != nil {
					status := http.StatusBadRequest
					if errors.Is(err, telegramchat.ErrWebhookUnauthorized) {
						status = http.StatusForbidden
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
				runAdapterWithRetry(ctx, fmt.Sprintf("telegram webhook registrar[%s]", instanceCopy.ID), func(listenCtx context.Context) error {
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
			runAdapterWithRetry(ctx, fmt.Sprintf("telegram adapter[%s]", instanceCopy.ID), func(listenCtx context.Context) error {
				log.Printf("telegram adapter started: instance=%s mode=%s", instanceCopy.ID, instanceCopy.Mode)
				return telegramAdapter.Listen(listenCtx, telegramInboundHandler)
			})
		}()
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
			runAdapterWithRetry(ctx, fmt.Sprintf("discord adapter[%s]", instanceCopy.ID), func(listenCtx context.Context) error {
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
					handleDiscordInbound(messageCtx, runtime, delivery, discordAdapter, envelope, scopedConversationID, instanceCopy.ID)
					return nil
				})
			})
		}()
	}

	if !started {
		log.Println("no runtime integrations enabled; enable health probe, telegram, or discord to start the service")
		return nil
	}

	log.Printf("synapsex service started with %d runtime(s); default agent %q", len(runtimes), defaultRuntime.agentID)

	select {
	case err := <-fatalErrCh:
		return fmt.Errorf("runtime error: %w", err)
	case <-ctx.Done():
		log.Println("shutdown signal received")
		return nil
	}
}

func runAdapterWithRetry(ctx context.Context, component string, listen func(context.Context) error) {
	backoff := 2 * time.Second
	const maxBackoff = 30 * time.Second

	for {
		if ctx.Err() != nil {
			return
		}
		err := listen(ctx)
		if err == nil || errors.Is(err, context.Canceled) || ctx.Err() != nil {
			return
		}

		log.Printf("%s stopped: %v; retrying in %s", component, err, backoff)
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

func normalizeWebhookRoutePath(raw, instanceID string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		path = "/webhooks/telegram/" + sanitizeConversationSegment(instanceID, "default")
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
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
	fs := flag.NewFlagSet("synapsex config agent add", flag.ContinueOnError)
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
		return fmt.Errorf("agent id is required; usage: synapsex config agent default <agent-id>")
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
	fmt.Fprintln(os.Stdout, "  synapsex config agent list")
	fmt.Fprintln(os.Stdout, "  synapsex config agent add --id <agent-id> [--profile codex|claude|local-smoke] [--workspace <path>] [--timeout <seconds>] [--default]")
	fmt.Fprintln(os.Stdout, "  synapsex config agent default <agent-id>")
}

func printConfigUsage() {
	fmt.Fprintln(os.Stdout, "Usage:")
	fmt.Fprintln(os.Stdout, "  synapsex config")
	fmt.Fprintln(os.Stdout, "  synapsex config show")
	fmt.Fprintln(os.Stdout, "  synapsex config path")
	fmt.Fprintln(os.Stdout, "  synapsex config get [dot-key]")
	fmt.Fprintln(os.Stdout, "  synapsex config set <dot-key> <value>")
	fmt.Fprintln(os.Stdout, "  synapsex config channel [telegram|discord]")
	fmt.Fprintln(os.Stdout, "  synapsex config agent list")
	fmt.Fprintln(os.Stdout, "  synapsex config agent add --id <agent-id> [--profile codex|claude|local-smoke] [--workspace <path>] [--timeout <seconds>] [--default]")
	fmt.Fprintln(os.Stdout, "  synapsex config agent default <agent-id>")
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
		return fmt.Errorf("usage: synapsex config set <dot-key> <value>")
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

func runConfigChannelCommand(args []string) error {
	if _, _, err := config.EnsureDefaultFile(); err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}

	channel := ""
	if len(args) > 0 {
		channel = strings.ToLower(strings.TrimSpace(args[0]))
	}
	if channel == "" {
		if !interactiveInputAvailable() {
			return fmt.Errorf("channel is required in non-interactive mode; use `synapsex config channel telegram` or `synapsex config channel discord`")
		}
		selected, err := promptMenu(
			"选择要增量配置的 Channel:",
			[]menuOption{
				{key: "telegram", label: "Telegram", aliases: []string{"1", "telegram", "tg"}, selected: true},
				{key: "discord", label: "Discord", aliases: []string{"2", "discord", "dc"}},
			},
		)
		if err != nil {
			return err
		}
		channel = selected
	}

	switch channel {
	case "telegram", "tg":
		return runConfigChannelTelegram()
	case "discord", "dc":
		return runConfigChannelDiscord()
	case "help", "-h", "--help":
		fmt.Fprintln(os.Stdout, "Usage: synapsex config channel [telegram|discord]")
		return nil
	default:
		return fmt.Errorf("unknown channel %q; expected telegram or discord", channel)
	}
}

func runConfigChannelTelegram() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "")
	fmt.Fprintln(os.Stdout, "Telegram 增量配置（回车保持原值）")

	enabled, err := promptBool("启用 Telegram?", cfg.TelegramEnabled)
	if err != nil {
		return err
	}

	token, err := promptString("Telegram Bot Token (留空保持): ")
	if err != nil {
		return err
	}
	botUsername, err := promptString("Telegram Bot Username (留空保持): ")
	if err != nil {
		return err
	}
	defaultAgent, err := promptString(fmt.Sprintf("Telegram defaultAgent (留空保持，当前 %s): ", firstNonEmpty(cfg.TelegramDefaultAgentID, cfg.DefaultAgentID, "main")))
	if err != nil {
		return err
	}
	requireMention, err := promptBool("群聊要求命令或@提及?", cfg.TelegramRequireCommandMention)
	if err != nil {
		return err
	}
	currentMode := strings.ToLower(strings.TrimSpace(cfg.TelegramMode))
	if currentMode == "" {
		currentMode = "polling"
	}
	mode, err := promptMenu(
		fmt.Sprintf("Telegram mode (当前 %s):", currentMode),
		[]menuOption{
			{key: "polling", label: "Polling (默认)", aliases: []string{"1", "polling"}, selected: currentMode == "polling"},
			{key: "webhook", label: "Webhook", aliases: []string{"2", "webhook"}, selected: currentMode == "webhook"},
		},
	)
	if err != nil {
		return err
	}
	if mode == "" {
		mode = currentMode
	}

	pollingSeconds := int(defaultDurationSeconds(cfg.TelegramPollingTimeout, 30*time.Second))
	webhookURL := ""
	webhookPath := ""
	webhookSecret := ""
	if mode == "polling" {
		pollingSeconds, err = promptIntDefault("pollingSeconds", pollingSeconds)
		if err != nil {
			return err
		}
	} else {
		webhookURL, err = promptString(fmt.Sprintf("webhookUrl (当前 %s): ", firstNonEmpty(cfg.TelegramWebhookURL, "空")))
		if err != nil {
			return err
		}
		if strings.TrimSpace(webhookURL) == "" {
			webhookURL = strings.TrimSpace(cfg.TelegramWebhookURL)
		}
		if strings.TrimSpace(webhookURL) == "" {
			return fmt.Errorf("webhook mode requires webhookUrl")
		}
		webhookPath, err = promptString(fmt.Sprintf("webhookPath (当前 %s): ", firstNonEmpty(cfg.TelegramWebhookPath, "/webhooks/telegram")))
		if err != nil {
			return err
		}
		if strings.TrimSpace(webhookPath) == "" {
			webhookPath = firstNonEmpty(cfg.TelegramWebhookPath, "/webhooks/telegram")
		}
		webhookSecret, err = promptString("webhookSecret (留空保持): ")
		if err != nil {
			return err
		}
	}

	chatIDsRaw, err := promptString(fmt.Sprintf("allowedChatIds 逗号分隔 (留空保持，当前 %s): ", summarizeList(cfg.TelegramAllowedChatIDs)))
	if err != nil {
		return err
	}

	updates := map[string]string{
		"channels.telegram.enabled":                 strconv.FormatBool(enabled),
		"channels.telegram.mode":                    mode,
		"channels.telegram.requireCommandOrMention": strconv.FormatBool(requireMention),
	}
	if mode == "polling" {
		updates["channels.telegram.pollingSeconds"] = strconv.Itoa(pollingSeconds)
	} else {
		updates["channels.telegram.webhookUrl"] = strings.TrimSpace(webhookURL)
		updates["channels.telegram.webhookPath"] = strings.TrimSpace(webhookPath)
		if strings.TrimSpace(webhookSecret) != "" {
			updates["channels.telegram.webhookSecret"] = strings.TrimSpace(webhookSecret)
		}
	}
	if strings.TrimSpace(token) != "" {
		updates["channels.telegram.token"] = strings.TrimSpace(token)
	}
	if strings.TrimSpace(botUsername) != "" {
		updates["channels.telegram.botUsername"] = strings.TrimSpace(botUsername)
	}
	if strings.TrimSpace(defaultAgent) != "" {
		updates["channels.telegram.defaultAgent"] = strings.TrimSpace(defaultAgent)
	}
	if strings.TrimSpace(chatIDsRaw) != "" {
		values := splitCSV(strings.TrimSpace(chatIDsRaw))
		payload, err := json.Marshal(values)
		if err != nil {
			return err
		}
		updates["channels.telegram.allowedChatIds"] = string(payload)
	}

	for key, value := range updates {
		if _, err := config.SetValueByDotKey(key, value); err != nil {
			return err
		}
	}

	fmt.Fprintln(os.Stdout, "Telegram channel config updated.")
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

	for key, value := range updates {
		if _, err := config.SetValueByDotKey(key, value); err != nil {
			return err
		}
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
	fmt.Fprintf(os.Stdout, "Usage: synapsex [serve|config|skill|help]\n")
	fmt.Fprintf(os.Stdout, "  serve  Start the service. If config.json is missing, bootstrap it first.\n")
	fmt.Fprintf(os.Stdout, "  config Launch the interactive config wizard, or run `config agent ...` for agent management.\n")
	fmt.Fprintf(os.Stdout, "  skill  Manage skill registry (list|reload|enable|disable).\n")
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
			firstNonEmpty(strings.TrimSpace(opts.DatabaseName), "synapse_x"),
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
	log.Printf("edit %q if needed, then run `synapsex serve`", path)
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
			name:       "synapse_x",
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
	name, err := promptStringDefault("Database Name", "synapse_x")
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

func buildAgentRuntimes(cfg config.Snapshot, sessionManager *service.SessionManager) (map[string]agentRuntime, string, error) {
	runtimes := make(map[string]agentRuntime)

	if len(cfg.Agents) > 0 && len(cfg.ProviderProfiles) > 0 {
		agentIDs := sortedAgentIDs(cfg.Agents)
		for _, agentID := range agentIDs {
			agent := cfg.Agents[agentID]
			profile, ok := cfg.ProviderProfiles[agent.ProfileID]
			if !ok {
				return nil, "", fmt.Errorf("agent %q references unknown profile %q", agent.ID, agent.ProfileID)
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
				return nil, "", fmt.Errorf("init skill runtime for agent %q: %w", agent.ID, err)
			}
			router := service.NewRouter(runtimeCfg, sessionManager, runner, service.WithIntentPipeline(pipeline))
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
				agentID:     agent.ID,
				backendName: runner.Name(),
				cwd:         workspace,
				profileKind: profile.Kind,
				profileCmd:  profileCommand,
				router:      router,
				runner:      runner,
				skills:      registry,
			}
		}
	}

	defaultAgentID := strings.TrimSpace(cfg.DefaultAgentID)
	if defaultAgentID == "" && cfg.ActiveAgent != nil {
		defaultAgentID = strings.TrimSpace(cfg.ActiveAgent.ID)
	}
	if defaultAgentID != "" {
		if _, ok := runtimes[defaultAgentID]; ok {
			return runtimes, defaultAgentID, nil
		}
	}
	if len(runtimes) > 0 {
		ids := make([]string, 0, len(runtimes))
		for id := range runtimes {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		return runtimes, ids[0], nil
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
	primaryRouter := func() *service.Router {
		registry, pipeline, err := buildSkillRuntimeComponents(cfg, primaryID, cfg.DefaultCWD)
		if err != nil {
			return service.NewRouter(cfg, sessionManager, runner)
		}
		primaryRegistry = registry
		return service.NewRouter(cfg, sessionManager, runner, service.WithIntentPipeline(pipeline))
	}()

	runtimes[primaryID] = agentRuntime{
		agentID:     primaryID,
		backendName: runner.Name(),
		cwd:         cfg.DefaultCWD,
		profileKind: profileKind,
		profileCmd:  profileCmd,
		router:      primaryRouter,
		runner:      runner,
		skills:      primaryRegistry,
	}
	return runtimes, primaryID, nil
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

func handleTelegramInbound(
	ctx context.Context,
	runtime agentRuntime,
	delivery *service.OutputDelivery,
	adapter *telegramchat.Adapter,
	envelope telegramchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		if err != nil {
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendTelegramDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleSynapseXSkillMetaCommand(runtime, message.Text); handled {
		sendTelegramDirect(ctx, adapter, envelope.Target, response)
		return
	}

	decision, err := runtime.router.Route(ctx, message)
	if err != nil {
		sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	sendTelegramDirect(ctx, adapter, envelope.Target, "正在思考...")

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID)
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
		executeInput := buildExecutionInput(decision)
		log.Printf("telegram execute begin: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, runtime.cwd, decision.ConversationID, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       decision.WindowID,
			Input:          executeInput,
			Backend:        runtime.backendName,
			CWD:            runtime.cwd,
		})
		if err != nil {
			log.Printf("telegram execute failed: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendTelegramDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("telegram execute done: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s session_id=%s backend_session_id=%s state=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if output == "" {
			output = "执行完成，无可见输出"
		}
		output = applyExecutionSourceLabel(decision, output)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, telegramchat.MaxMessageLength, 1)
	}
}

func sendTelegramDirect(ctx context.Context, adapter *telegramchat.Adapter, target telegramchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send telegram message: %v", err)
	}
}

func handleDiscordInbound(
	ctx context.Context,
	runtime agentRuntime,
	delivery *service.OutputDelivery,
	adapter *discordchat.Adapter,
	envelope discordchat.InboundEnvelope,
	scopedConversationID string,
	instanceID string,
) {
	message := envelope.Message
	message.ConversationID = scopedConversationID
	if handled, response, err := handleConfigChatCommand(message); handled {
		if err != nil {
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		sendDiscordDirect(ctx, adapter, envelope.Target, response)
		return
	}
	if handled, response := handleSynapseXSkillMetaCommand(runtime, message.Text); handled {
		sendDiscordDirect(ctx, adapter, envelope.Target, response)
		return
	}

	log.Printf("discord route begin: instance=%s agent=%s conversation_id=%s text=%q", instanceID, runtime.agentID, message.ConversationID, message.Text)
	decision, err := runtime.router.Route(ctx, message)
	if err != nil {
		log.Printf("discord route rejected: %v", err)
		sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
		return
	}
	log.Printf("discord route decision: instance=%s agent=%s kind=%s conversation_id=%s", instanceID, runtime.agentID, decision.Kind, decision.ConversationID)

	switch decision.Kind {
	case service.DecisionControl:
		result, err := runtime.router.HandleControlCommand(ctx, decision.Command, decision.ConversationID, decision.WindowID)
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
		executeInput := buildExecutionInput(decision)
		log.Printf("discord execute begin: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s cwd=%s conversation_id=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, runtime.cwd, decision.ConversationID, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence)
		stopTyping := startDiscordTypingLoop(ctx, adapter, envelope.Target)
		defer stopTyping()
		flowResult, err := runtime.router.HandleSessionFlow(ctx, command.SessionCommand{
			Mode:           command.ModeContinue,
			ConversationID: decision.ConversationID,
			WindowID:       decision.WindowID,
			Input:          executeInput,
			Backend:        runtime.backendName,
			CWD:            runtime.cwd,
		})
		if err != nil {
			log.Printf("discord execute failed: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d err=%v", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), err)
			sendDiscordDirect(ctx, adapter, envelope.Target, chatiface.FormatError(err))
			return
		}
		log.Printf("discord execute done: instance=%s agent=%s backend=%s profile_kind=%s profile_command=%s conversation_id=%s session_id=%s backend_session_id=%s state=%s intent.kind=%s intent.reason=%s intent.skill=%s intent.confidence=%.2f duration_ms=%d output_chars=%d", instanceID, runtime.agentID, runtime.backendName, runtime.profileKind, runtime.profileCmd, decision.ConversationID, flowResult.Session.ID, flowResult.Execution.BackendSessionID, flowResult.Execution.State, decision.Kind, decision.IntentReason, decision.SkillName, decision.Confidence, time.Since(started).Milliseconds(), len(flowResult.Execution.Output))

		adapter.BindSession(flowResult.Session.ID, envelope.Target)

		output := flowResult.Execution.Output
		if output == "" {
			output = "执行完成，无可见输出"
		}
		output = applyExecutionSourceLabel(decision, output)
		delivery.Deliver(ctx, adapter, flowResult.Session.ID, output, discordchat.MaxMessageLength, 1)
	}
}

func sendDiscordDirect(ctx context.Context, adapter *discordchat.Adapter, target discordchat.Target, message string) {
	if err := adapter.SendDirect(ctx, target, message); err != nil {
		log.Printf("send discord message: %v", err)
	}
}

func handleSynapseXSkillMetaCommand(runtime agentRuntime, rawText string) (bool, string) {
	text := strings.TrimSpace(rawText)
	if text == "" {
		return false, ""
	}
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return false, ""
	}
	command := strings.TrimPrefix(strings.ToLower(strings.TrimSpace(fields[0])), "/")
	if command != "sx-skills" {
		return false, ""
	}
	if runtime.skills == nil {
		return true, "[SynapseX Skill Registry]\n当前运行时未加载技能注册中心"
	}
	body := skillregistry.FormatList(runtime.skills.List())
	return true, "[SynapseX Skill Registry]\n" + body
}

func applyExecutionSourceLabel(decision service.Decision, output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return text
	}
	if decision.Kind == service.DecisionSkill {
		prefix := "[SynapseX Skill]"
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

func buildExecutionInput(decision service.Decision) string {
	if decision.Kind != service.DecisionSkill || decision.Skill == nil {
		return decision.Message.Text
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
	return builder.String()
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
