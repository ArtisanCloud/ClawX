package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	chatiface "synapsex/internal/interfaces/chat"
)

const (
	MaxMessageLength = 2000
	defaultAPIBase   = "https://discord.com/api/v10"
	defaultGateway   = "wss://gateway.discord.gg/?v=10&encoding=json"
)

const (
	intentGuildMessages  = 1 << 9
	intentDirectMessages = 1 << 12
	intentMessageContent = 1 << 15
)

const (
	discordInteractionTypeApplicationCommand = 2

	discordInteractionResponseChannelMessageWithSource = 4
	discordApplicationCommandTypeChatInput             = 1
	discordApplicationCommandOptionTypeString          = 3
)

var (
	ErrMessageTooLong      = errors.New("discord message exceeds limit")
	ErrMissingBotToken     = errors.New("discord bot token is required")
	ErrUnknownSessionRoute = errors.New("discord session target is not bound")
)

type Options struct {
	BotToken        string
	APIBaseURL      string
	GatewayURL      string
	AllowedChannels []string
	RequireMention  bool
	UseSystemProxy  bool
	HTTPClient      *http.Client
}

type Target struct {
	ChannelID        string
	InteractionID    string
	InteractionToken string
}

type InboundEnvelope struct {
	Message chatiface.Message
	Target  Target
}

type InboundHandler func(ctx context.Context, envelope InboundEnvelope) error

type Adapter struct {
	token          string
	apiBaseURL     string
	gatewayURL     string
	requireMention bool
	client         *http.Client
	allowed        map[string]struct{}

	mu             sync.Mutex
	sessionTargets map[string]Target
	botUserID      string
	applicationID  string
	commandsSynced bool
	lastSeq        *int64
}

func NewAdapter(options Options) (*Adapter, error) {
	token := strings.TrimSpace(options.BotToken)
	if token == "" {
		return nil, ErrMissingBotToken
	}

	apiBaseURL := strings.TrimRight(strings.TrimSpace(options.APIBaseURL), "/")
	if apiBaseURL == "" {
		apiBaseURL = defaultAPIBase
	}

	gatewayURL := strings.TrimSpace(options.GatewayURL)
	if gatewayURL == "" {
		gatewayURL = defaultGateway
	}

	client := options.HTTPClient
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		if !options.UseSystemProxy {
			transport.Proxy = nil
		}
		client = &http.Client{
			Timeout:   15 * time.Second,
			Transport: transport,
		}
	}

	return &Adapter{
		token:          token,
		apiBaseURL:     apiBaseURL,
		gatewayURL:     gatewayURL,
		requireMention: options.RequireMention,
		client:         client,
		allowed:        buildAllowedChannelSet(options.AllowedChannels),
		sessionTargets: make(map[string]Target),
	}, nil
}

func (a *Adapter) Listen(ctx context.Context, handler InboundHandler) error {
	if handler == nil {
		return nil
	}

	backoff := time.Second
	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		if err := a.listenOnce(ctx, handler); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			log.Printf("discord gateway listen error: %v; reconnecting", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(backoff):
			}
			if backoff < 15*time.Second {
				backoff *= 2
			}
			continue
		}

		backoff = time.Second
	}
}

func (a *Adapter) listenOnce(ctx context.Context, handler InboundHandler) error {
	socket, err := dialGateway(ctx, a.gatewayURL)
	if err != nil {
		return err
	}
	defer socket.Close()

	helloCtx, cancelHello := context.WithTimeout(ctx, 15*time.Second)
	defer cancelHello()

	first, err := socket.ReadText(helloCtx)
	if err != nil {
		return fmt.Errorf("discord gateway hello timeout/read failed: %w", err)
	}

	var hello gatewayEvent
	if err := json.Unmarshal(first, &hello); err != nil {
		return err
	}
	if hello.Op != 10 {
		return fmt.Errorf("discord expected HELLO opcode, got %d", hello.Op)
	}

	var helloData helloPayload
	if err := json.Unmarshal(hello.D, &helloData); err != nil {
		return err
	}

	if err := socket.WriteJSON(ctx, identifyEnvelope{Op: 2, D: identifyData{
		Token:   a.token,
		Intents: intentGuildMessages | intentDirectMessages | intentMessageContent,
		Properties: identifyProperties{
			OS:      "linux",
			Browser: "synapsex",
			Device:  "synapsex",
		},
	}}); err != nil {
		return err
	}
	log.Printf("discord gateway identify sent")

	heartbeatCtx, cancelHeartbeat := context.WithCancel(ctx)
	defer cancelHeartbeat()
	go a.runHeartbeat(heartbeatCtx, socket, time.Duration(helloData.HeartbeatInterval)*time.Millisecond)

	for {
		payload, err := socket.ReadText(ctx)
		if err != nil {
			return err
		}

		var event gatewayEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			continue
		}
		a.setSequence(event.S)

		switch event.Op {
		case 0:
			if err := a.handleDispatch(ctx, event, handler); err != nil {
				return err
			}
		case 1:
			if err := a.sendHeartbeat(ctx, socket); err != nil {
				return err
			}
		case 7, 9:
			return errors.New("discord requested reconnect")
		case 11:
			continue
		default:
			continue
		}
	}
}

func (a *Adapter) runHeartbeat(ctx context.Context, socket *gatewaySocket, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = a.sendHeartbeat(ctx, socket)
		}
	}
}

func (a *Adapter) sendHeartbeat(ctx context.Context, socket *gatewaySocket) error {
	return socket.WriteJSON(ctx, gatewayHeartbeat{Op: 1, D: a.sequencePayload()})
}

func (a *Adapter) handleDispatch(ctx context.Context, event gatewayEvent, handler InboundHandler) error {
	switch strings.TrimSpace(event.T) {
	case "READY":
		var ready readyPayload
		if err := json.Unmarshal(event.D, &ready); err != nil {
			return nil
		}
		a.setBotUserID(ready.User.ID)
		a.setApplicationID(ready.Application.ID)
		log.Printf("discord gateway ready: bot_user_id=%s application_id=%s", ready.User.ID, strings.TrimSpace(ready.Application.ID))
		if err := a.syncSlashCommands(ctx); err != nil {
			log.Printf("discord command sync failed: %v", err)
		}
		return nil
	case "MESSAGE_CREATE":
		var message discordMessageCreate
		if err := json.Unmarshal(event.D, &message); err != nil {
			return nil
		}
		envelope, ok, err := a.normalizeMessage(message)
		if err != nil || !ok {
			return err
		}
		log.Printf("discord inbound message: channel_id=%s author_id=%s", message.ChannelID, message.Author.ID)
		return handler(ctx, envelope)
	case "INTERACTION_CREATE":
		var interaction discordInteractionCreate
		if err := json.Unmarshal(event.D, &interaction); err != nil {
			return nil
		}
		envelope, ok, err := a.normalizeInteraction(interaction)
		if err != nil || !ok {
			return err
		}
		log.Printf("discord inbound interaction: command=%s channel_id=%s user_id=%s", strings.TrimSpace(interaction.Data.Name), interaction.ChannelID, envelope.Message.UserID)
		return handler(ctx, envelope)
	default:
		return nil
	}
}

func (a *Adapter) BindSession(sessionID string, target Target) {
	sessionID = strings.TrimSpace(sessionID)
	target.ChannelID = strings.TrimSpace(target.ChannelID)
	target.InteractionID = ""
	target.InteractionToken = ""
	if sessionID == "" || target.ChannelID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionTargets[sessionID] = target
}

func (a *Adapter) SendDirect(ctx context.Context, target Target, text string) error {
	content := strings.TrimSpace(text)
	if content == "" {
		return nil
	}
	target.ChannelID = strings.TrimSpace(target.ChannelID)
	target.InteractionID = strings.TrimSpace(target.InteractionID)
	target.InteractionToken = strings.TrimSpace(target.InteractionToken)
	if target.ChannelID == "" {
		return nil
	}

	if target.InteractionID != "" && target.InteractionToken != "" {
		return a.sendInteractionResponse(ctx, target, content)
	}

	for _, chunk := range splitText(content, MaxMessageLength) {
		if err := a.sendMessage(ctx, target, chunk); err != nil {
			return err
		}
	}
	return nil
}

func (a *Adapter) SendText(ctx context.Context, sessionID, chunk string, _ bool) error {
	if len([]rune(chunk)) > MaxMessageLength {
		return ErrMessageTooLong
	}

	target, err := a.lookupTarget(sessionID)
	if err != nil {
		return err
	}
	return a.sendMessage(ctx, target, chunk)
}

func (a *Adapter) SendTyping(ctx context.Context, target Target) error {
	target.ChannelID = strings.TrimSpace(target.ChannelID)
	if target.ChannelID == "" {
		return nil
	}
	return a.sendTyping(ctx, target)
}

func (a *Adapter) SendError(ctx context.Context, sessionID, message string) error {
	target, err := a.lookupTarget(sessionID)
	if err != nil {
		return err
	}
	return a.SendDirect(ctx, target, strings.TrimSpace(message))
}

func (a *Adapter) lookupTarget(sessionID string) (Target, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	target, ok := a.sessionTargets[strings.TrimSpace(sessionID)]
	if !ok {
		return Target{}, ErrUnknownSessionRoute
	}
	return target, nil
}

func (a *Adapter) normalizeMessage(message discordMessageCreate) (InboundEnvelope, bool, error) {
	if message.Author.Bot {
		return InboundEnvelope{}, false, nil
	}

	content, ok := a.prepareContent(message.Content, message.GuildID == "")
	if !ok {
		return InboundEnvelope{}, false, nil
	}

	normalized, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          message.Author.ID,
		GuildID:         buildGuildID(message.GuildID),
		ThreadID:        buildThreadID(message.ChannelID),
		Text:            content,
		IsDirectMessage: message.GuildID == "",
		IsThread:        false,
		IsAllowed:       a.isChannelAllowed(message.ChannelID),
	})
	if err != nil {
		return InboundEnvelope{}, false, err
	}

	return InboundEnvelope{
		Message: normalized,
		Target:  Target{ChannelID: message.ChannelID},
	}, true, nil
}

func (a *Adapter) normalizeInteraction(interaction discordInteractionCreate) (InboundEnvelope, bool, error) {
	if interaction.Type != discordInteractionTypeApplicationCommand {
		return InboundEnvelope{}, false, nil
	}

	commandText, ok, err := interactionToCommandText(interaction.Data)
	if err != nil || !ok {
		return InboundEnvelope{}, false, err
	}

	userID := strings.TrimSpace(interaction.User.ID)
	if strings.TrimSpace(interaction.Member.User.ID) != "" {
		userID = strings.TrimSpace(interaction.Member.User.ID)
	}
	if userID == "" {
		return InboundEnvelope{}, false, nil
	}

	normalized, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "discord",
		UserID:          userID,
		GuildID:         buildGuildID(interaction.GuildID),
		ThreadID:        buildThreadID(interaction.ChannelID),
		Text:            commandText,
		IsDirectMessage: strings.TrimSpace(interaction.GuildID) == "",
		IsThread:        false,
		IsAllowed:       a.isChannelAllowed(interaction.ChannelID),
	})
	if err != nil {
		return InboundEnvelope{}, false, err
	}

	return InboundEnvelope{
		Message: normalized,
		Target: Target{
			ChannelID:        strings.TrimSpace(interaction.ChannelID),
			InteractionID:    strings.TrimSpace(interaction.ID),
			InteractionToken: strings.TrimSpace(interaction.Token),
		},
	}, true, nil
}

func interactionToCommandText(data discordInteractionData) (string, bool, error) {
	command := strings.ToLower(strings.TrimSpace(data.Name))
	if command == "" {
		return "", false, nil
	}

	switch command {
	case "new", "list", "cancel", "current":
		return "/" + command, true, nil
	case "sx-skills":
		return "/sx-skills", true, nil
	case "resume":
		sessionID := strings.TrimSpace(interactionOptionValue(data.Options, "session_id"))
		if sessionID == "" {
			return "", false, fmt.Errorf("discord interaction /resume missing session_id option")
		}
		return "/resume " + sessionID, true, nil
	case "switch":
		sessionID := strings.TrimSpace(interactionOptionValue(data.Options, "session_id"))
		if sessionID == "" {
			return "", false, fmt.Errorf("discord interaction /switch missing session_id option")
		}
		return "/switch " + sessionID, true, nil
	case "sx-skill":
		name := strings.TrimSpace(interactionOptionValue(data.Options, "name"))
		if name == "" {
			return "", false, fmt.Errorf("discord interaction /sx-skill missing name option")
		}
		input := strings.TrimSpace(interactionOptionValue(data.Options, "input"))
		if input == "" {
			return "/sx-skill " + name, true, nil
		}
		return "/sx-skill " + name + " " + input, true, nil
	default:
		return "/" + command, true, nil
	}
}

func interactionOptionValue(options []discordInteractionOption, key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	for _, option := range options {
		if strings.TrimSpace(option.Name) != key {
			continue
		}
		var text string
		if err := json.Unmarshal(option.Value, &text); err == nil {
			return strings.TrimSpace(text)
		}
		var number float64
		if err := json.Unmarshal(option.Value, &number); err == nil {
			return strings.TrimSpace(strconv.FormatFloat(number, 'f', -1, 64))
		}
		var flag bool
		if err := json.Unmarshal(option.Value, &flag); err == nil {
			return strings.TrimSpace(strconv.FormatBool(flag))
		}
	}
	return ""
}

func (a *Adapter) prepareContent(raw string, isDirect bool) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}
	if isDirect || !a.requireMention {
		return text, true
	}

	botUserID := a.currentBotUserID()
	if botUserID == "" {
		return "", false
	}

	mention := "<@" + botUserID + ">"
	altMention := "<@!" + botUserID + ">"
	if !strings.Contains(text, mention) && !strings.Contains(text, altMention) {
		return "", false
	}

	cleaned := strings.ReplaceAll(text, mention, "")
	cleaned = strings.ReplaceAll(cleaned, altMention, "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "", false
	}
	return cleaned, true
}

func (a *Adapter) sendMessage(ctx context.Context, target Target, text string) error {
	if len([]rune(text)) > MaxMessageLength {
		return ErrMessageTooLong
	}

	body, err := json.Marshal(map[string]string{"content": text})
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/channels/%s/messages", a.apiBaseURL, target.ChannelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+a.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord send message failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (a *Adapter) sendInteractionResponse(ctx context.Context, target Target, text string) error {
	content := strings.TrimSpace(text)
	if content == "" {
		return nil
	}
	content = trimRunes(content, MaxMessageLength)

	payload := discordInteractionResponse{
		Type: discordInteractionResponseChannelMessageWithSource,
		Data: discordInteractionResponseData{
			Content: content,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/interactions/%s/%s/callback", a.apiBaseURL, target.InteractionID, target.InteractionToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord interaction callback failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (a *Adapter) sendTyping(ctx context.Context, target Target) error {
	endpoint := fmt.Sprintf("%s/channels/%s/typing", a.apiBaseURL, target.ChannelID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bot "+a.token)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord send typing failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return nil
}

func (a *Adapter) syncSlashCommands(ctx context.Context) error {
	applicationID := a.currentApplicationID()
	if applicationID == "" {
		return nil
	}
	if a.commandsAlreadySynced() {
		return nil
	}

	commandDefs := []discordApplicationCommand{
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "new",
			Description: "创建新会话",
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "list",
			Description: "列出可用会话",
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "current",
			Description: "查看当前会话",
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "resume",
			Description: "恢复指定会话",
			Options: []discordApplicationCommandOption{
				{
					Type:        discordApplicationCommandOptionTypeString,
					Name:        "session_id",
					Description: "会话ID，例如 sess-xxxx",
					Required:    true,
				},
			},
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "switch",
			Description: "切换当前会话",
			Options: []discordApplicationCommandOption{
				{
					Type:        discordApplicationCommandOptionTypeString,
					Name:        "session_id",
					Description: "会话ID，例如 sess-xxxx",
					Required:    true,
				},
			},
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "cancel",
			Description: "取消当前会话执行",
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "sx-skills",
			Description: "列出 SynapseX 技能目录",
		},
		{
			Type:        discordApplicationCommandTypeChatInput,
			Name:        "sx-skill",
			Description: "强制使用 SynapseX 技能",
			Options: []discordApplicationCommandOption{
				{
					Type:        discordApplicationCommandOptionTypeString,
					Name:        "name",
					Description: "技能名称，例如 echo",
					Required:    true,
				},
				{
					Type:        discordApplicationCommandOptionTypeString,
					Name:        "input",
					Description: "传给技能的输入",
					Required:    false,
				},
			},
		},
	}

	body, err := json.Marshal(commandDefs)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/applications/%s/commands", a.apiBaseURL, applicationID)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bot "+a.token)
	request.Header.Set("Content-Type", "application/json")

	response, err := a.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("discord command sync failed: status=%d body=%s", response.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	a.markCommandsSynced(true)
	log.Printf("discord slash commands synced: application_id=%s count=%d", applicationID, len(commandDefs))
	return nil
}

func (a *Adapter) isChannelAllowed(channelID string) bool {
	if len(a.allowed) == 0 {
		return true
	}
	_, ok := a.allowed[strings.TrimSpace(channelID)]
	return ok
}

func (a *Adapter) setBotUserID(userID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.botUserID = strings.TrimSpace(userID)
}

func (a *Adapter) currentBotUserID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.botUserID
}

func (a *Adapter) setApplicationID(value string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return
	}
	if a.applicationID != trimmed {
		a.applicationID = trimmed
		a.commandsSynced = false
	}
}

func (a *Adapter) currentApplicationID() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.applicationID
}

func (a *Adapter) commandsAlreadySynced() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.commandsSynced
}

func (a *Adapter) markCommandsSynced(value bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commandsSynced = value
}

func (a *Adapter) setSequence(seq *int64) {
	if seq == nil {
		return
	}
	value := *seq
	a.mu.Lock()
	defer a.mu.Unlock()
	a.lastSeq = &value
}

func (a *Adapter) sequencePayload() any {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.lastSeq == nil {
		return nil
	}
	return *a.lastSeq
}

func buildAllowedChannelSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		allowed[trimmed] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func buildGuildID(guildID string) string {
	return strings.TrimSpace(guildID)
}

func buildThreadID(channelID string) string {
	return strings.TrimSpace(channelID)
}

func splitText(text string, maxLen int) []string {
	if len([]rune(text)) <= maxLen {
		return []string{text}
	}

	runes := []rune(text)
	segments := make([]string, 0, (len(runes)/maxLen)+1)
	for start := 0; start < len(runes); start += maxLen {
		end := start + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		segments = append(segments, string(runes[start:end]))
	}
	return segments
}

func trimRunes(text string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(text)
	if len(runes) <= maxLen {
		return text
	}
	return string(runes[:maxLen])
}

type gatewayEvent struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d"`
	S  *int64          `json:"s"`
	T  string          `json:"t"`
}

type helloPayload struct {
	HeartbeatInterval int `json:"heartbeat_interval"`
}

type identifyEnvelope struct {
	Op int          `json:"op"`
	D  identifyData `json:"d"`
}

type identifyData struct {
	Token      string             `json:"token"`
	Intents    int                `json:"intents"`
	Properties identifyProperties `json:"properties"`
}

type identifyProperties struct {
	OS      string `json:"os"`
	Browser string `json:"browser"`
	Device  string `json:"device"`
}

type gatewayHeartbeat struct {
	Op int `json:"op"`
	D  any `json:"d"`
}

type readyPayload struct {
	User struct {
		ID string `json:"id"`
	} `json:"user"`
	Application struct {
		ID string `json:"id"`
	} `json:"application"`
}

type discordMessageCreate struct {
	ID        string `json:"id"`
	ChannelID string `json:"channel_id"`
	GuildID   string `json:"guild_id"`
	Content   string `json:"content"`
	Author    struct {
		ID  string `json:"id"`
		Bot bool   `json:"bot"`
	} `json:"author"`
}

type discordInteractionCreate struct {
	ID        string                 `json:"id"`
	Type      int                    `json:"type"`
	Token     string                 `json:"token"`
	GuildID   string                 `json:"guild_id"`
	ChannelID string                 `json:"channel_id"`
	Data      discordInteractionData `json:"data"`
	User      struct {
		ID string `json:"id"`
	} `json:"user"`
	Member struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"member"`
}

type discordInteractionData struct {
	Name    string                     `json:"name"`
	Options []discordInteractionOption `json:"options"`
}

type discordInteractionOption struct {
	Name  string          `json:"name"`
	Type  int             `json:"type"`
	Value json.RawMessage `json:"value"`
}

type discordInteractionResponse struct {
	Type int                            `json:"type"`
	Data discordInteractionResponseData `json:"data"`
}

type discordInteractionResponseData struct {
	Content string `json:"content"`
}

type discordApplicationCommand struct {
	Type        int                               `json:"type"`
	Name        string                            `json:"name"`
	Description string                            `json:"description"`
	Options     []discordApplicationCommandOption `json:"options,omitempty"`
}

type discordApplicationCommandOption struct {
	Type        int    `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required,omitempty"`
}
