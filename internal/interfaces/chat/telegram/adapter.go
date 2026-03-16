package telegram

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	chatiface "clawx/internal/interfaces/chat"
)

const MaxMessageLength = 4096

const (
	sendMessageRetries  = 3
	sendMessageBackoff  = 300 * time.Millisecond
	sendMessageMaxDelay = 2 * time.Second
	setWebhookRetries   = 3
	setWebhookBackoff   = 300 * time.Millisecond
	setWebhookMaxDelay  = 2 * time.Second
)

var (
	ErrMessageTooLong        = errors.New("telegram message exceeds limit")
	ErrMissingToken          = errors.New("telegram bot token is required")
	ErrUnknownSessionRoute   = errors.New("telegram session target is not bound")
	ErrInvalidWebhook        = errors.New("invalid telegram webhook request")
	ErrWebhookMethod         = errors.New("telegram webhook method is not allowed")
	ErrWebhookUnauthorized   = errors.New("telegram webhook secret mismatch")
	ErrWebhookReplayRejected = errors.New("telegram webhook replay rejected")
)

type Options struct {
	Token                 string
	BaseURL               string
	BotUsername           string
	PollTimeout           time.Duration
	WebhookSecretToken    string
	AllowedChatIDs        []string
	RequireCommandMention bool
	HTTPClient            *http.Client
}

type Target struct {
	ChatID         int64
	ReplyToMessage int64
	ThreadID       int64
}

type InboundEnvelope struct {
	Message chatiface.Message
	Target  Target
	EventID string
}

type InboundHandler func(ctx context.Context, envelope InboundEnvelope) error

type Adapter struct {
	token                 string
	baseURL               string
	botUsername           string
	pollTimeout           time.Duration
	webhookSecretToken    string
	requireCommandMention bool
	client                *http.Client
	allowedChatIDs        map[int64]struct{}

	mu             sync.Mutex
	nextUpdateID   int64
	sessionTargets map[string]Target
	webhookReplay  map[string]time.Time
}

func NewAdapter(options Options) (*Adapter, error) {
	token := strings.TrimSpace(options.Token)
	if token == "" {
		return nil, ErrMissingToken
	}

	baseURL := strings.TrimSpace(options.BaseURL)
	if baseURL == "" {
		baseURL = "https://api.telegram.org"
	}

	pollTimeout := options.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = 30 * time.Second
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: pollTimeout + 10*time.Second}
	}

	return &Adapter{
		token:                 token,
		baseURL:               strings.TrimRight(baseURL, "/"),
		botUsername:           strings.TrimPrefix(strings.TrimSpace(options.BotUsername), "@"),
		pollTimeout:           pollTimeout,
		webhookSecretToken:    strings.TrimSpace(options.WebhookSecretToken),
		requireCommandMention: options.RequireCommandMention,
		client:                client,
		allowedChatIDs:        buildAllowedChatIDSet(options.AllowedChatIDs),
		sessionTargets:        make(map[string]Target),
		webhookReplay:         make(map[string]time.Time),
	}, nil
}

func (a *Adapter) Listen(ctx context.Context, handler InboundHandler) error {
	if handler == nil {
		return nil
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}

		updates, err := a.getUpdates(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return err
		}

		for _, update := range updates {
			if update.Message == nil {
				continue
			}
			envelope, ok, err := a.normalizeUpdate(update)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := handler(ctx, envelope); err != nil {
				return err
			}
		}
	}
}

func (a *Adapter) SetWebhook(ctx context.Context, webhookURL string) error {
	webhookURL = strings.TrimSpace(webhookURL)
	if webhookURL == "" {
		return fmt.Errorf("telegram webhook url is required")
	}

	backoff := setWebhookBackoff
	var lastErr error
	for attempt := 0; attempt < setWebhookRetries; attempt++ {
		err := a.setWebhookOnce(ctx, webhookURL)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableSetWebhookError(err) || attempt == setWebhookRetries-1 {
			return err
		}
		if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
			return sleepErr
		}
		backoff *= 2
		if backoff > setWebhookMaxDelay {
			backoff = setWebhookMaxDelay
		}
	}
	return lastErr
}

func (a *Adapter) setWebhookOnce(ctx context.Context, webhookURL string) error {
	payload := setWebhookRequest{
		URL: webhookURL,
	}
	if token := strings.TrimSpace(a.webhookSecretToken); token != "" {
		payload.SecretToken = token
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/bot%s/setWebhook", a.baseURL, a.token)
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

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("telegram setWebhook http status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var payloadResponse apiResponse
	if err := json.Unmarshal(responseBody, &payloadResponse); err != nil {
		return err
	}
	if !payloadResponse.OK {
		return fmt.Errorf("telegram setWebhook failed: %s", strings.TrimSpace(payloadResponse.Description))
	}
	return nil
}

func (a *Adapter) ParseWebhookRequest(r *http.Request) (InboundEnvelope, bool, error) {
	if r == nil {
		return InboundEnvelope{}, false, ErrInvalidWebhook
	}
	if r.Method != http.MethodPost {
		return InboundEnvelope{}, false, ErrWebhookMethod
	}

	expectedSecret := strings.TrimSpace(a.webhookSecretToken)
	if expectedSecret != "" {
		provided := strings.TrimSpace(r.Header.Get("X-Telegram-Bot-Api-Secret-Token"))
		if subtle.ConstantTimeCompare([]byte(expectedSecret), []byte(provided)) != 1 {
			return InboundEnvelope{}, false, ErrWebhookUnauthorized
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return InboundEnvelope{}, false, fmt.Errorf("%w: read body: %v", ErrInvalidWebhook, err)
	}

	var update telegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		return InboundEnvelope{}, false, fmt.Errorf("%w: decode payload: %v", ErrInvalidWebhook, err)
	}
	if update.Message == nil {
		return InboundEnvelope{}, false, nil
	}
	eventID := buildTelegramEventID(update)
	if !a.acceptWebhookEvent(eventID, time.Now().UTC()) {
		return InboundEnvelope{}, false, ErrWebhookReplayRejected
	}
	return a.normalizeUpdate(update)
}

func (a *Adapter) BindSession(sessionID string, target Target) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" || target.ChatID == 0 {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionTargets[sessionID] = target
}

func (a *Adapter) SendDirect(ctx context.Context, target Target, text string) error {
	content := strings.TrimSpace(text)
	if target.ChatID == 0 || content == "" {
		return nil
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

func (a *Adapter) normalizeUpdate(update telegramUpdate) (InboundEnvelope, bool, error) {
	if update.Message == nil {
		return InboundEnvelope{}, false, nil
	}
	message := *update.Message
	if strings.TrimSpace(message.Text) == "" {
		return InboundEnvelope{}, false, nil
	}

	target := Target{
		ChatID:         message.Chat.ID,
		ReplyToMessage: message.MessageID,
		ThreadID:       message.MessageThreadID,
	}

	isDirect := message.Chat.Type == "private"
	text, shouldHandle := a.prepareText(message.Text, isDirect)
	if !shouldHandle {
		return InboundEnvelope{}, false, nil
	}

	normalized, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
		Channel:         "telegram",
		UserID:          strconv.FormatInt(message.From.ID, 10),
		GuildID:         buildGuildID(message.Chat.ID, isDirect),
		ThreadID:        buildThreadID(message.MessageThreadID),
		Text:            text,
		IsDirectMessage: isDirect,
		IsThread:        message.MessageThreadID != 0,
		IsAllowed:       a.isChatAllowed(message.Chat.ID),
	})
	if err != nil {
		return InboundEnvelope{}, false, err
	}

	return InboundEnvelope{
		Message: normalized,
		Target:  target,
		EventID: buildTelegramEventID(update),
	}, true, nil
}

func (a *Adapter) acceptWebhookEvent(eventID string, now time.Time) bool {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return true
	}
	cutoff := now.Add(-15 * time.Minute)

	a.mu.Lock()
	defer a.mu.Unlock()

	for key, seenAt := range a.webhookReplay {
		if seenAt.Before(cutoff) {
			delete(a.webhookReplay, key)
		}
	}
	if seenAt, exists := a.webhookReplay[eventID]; exists && !seenAt.Before(cutoff) {
		return false
	}
	a.webhookReplay[eventID] = now
	return true
}

func buildTelegramEventID(update telegramUpdate) string {
	if update.UpdateID > 0 {
		return strconv.FormatInt(update.UpdateID, 10)
	}
	if update.Message != nil && update.Message.MessageID > 0 {
		return fmt.Sprintf("%d:%d", update.Message.Chat.ID, update.Message.MessageID)
	}
	return ""
}

func (a *Adapter) prepareText(raw string, isDirect bool) (string, bool) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return "", false
	}

	if strings.HasPrefix(text, "/") {
		if a.botUsername != "" {
			fields := strings.Fields(text)
			if len(fields) > 0 {
				commandToken := fields[0]
				suffix := "@" + strings.ToLower(a.botUsername)
				if strings.Contains(commandToken, "@") && strings.HasSuffix(strings.ToLower(commandToken), suffix) {
					base := commandToken[:len(commandToken)-len(suffix)]
					fields[0] = base
					text = strings.Join(fields, " ")
				}
			}
		}
		return strings.TrimSpace(text), true
	}

	if isDirect || !a.requireCommandMention {
		return strings.TrimSpace(text), true
	}

	if a.botUsername == "" {
		return "", false
	}

	mention := "@" + a.botUsername
	if !strings.Contains(strings.ToLower(text), strings.ToLower(mention)) {
		return "", false
	}

	cleaned := strings.ReplaceAll(text, mention, "")
	cleaned = strings.TrimSpace(cleaned)
	if cleaned == "" {
		return "", false
	}
	return cleaned, true
}

func (a *Adapter) isChatAllowed(chatID int64) bool {
	if len(a.allowedChatIDs) == 0 {
		return true
	}
	_, ok := a.allowedChatIDs[chatID]
	return ok
}

func (a *Adapter) getUpdates(ctx context.Context) ([]telegramUpdate, error) {
	query := url.Values{}
	query.Set("timeout", strconv.Itoa(int(a.pollTimeout/time.Second)))
	query.Set("offset", strconv.FormatInt(a.currentOffset(), 10))

	endpoint := fmt.Sprintf("%s/bot%s/getUpdates?%s", a.baseURL, a.token, query.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var payload getUpdatesResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	if !payload.OK {
		return nil, fmt.Errorf("telegram getUpdates failed: %s", strings.TrimSpace(payload.Description))
	}

	if len(payload.Result) > 0 {
		a.setOffset(payload.Result[len(payload.Result)-1].UpdateID + 1)
	}
	return payload.Result, nil
}

func (a *Adapter) sendMessage(ctx context.Context, target Target, text string) error {
	if len([]rune(text)) > MaxMessageLength {
		return ErrMessageTooLong
	}

	backoff := sendMessageBackoff
	var lastErr error
	for attempt := 0; attempt < sendMessageRetries; attempt++ {
		err := a.sendMessageOnce(ctx, target, text)
		if err == nil {
			return nil
		}
		lastErr = err
		if !isRetryableSendMessageError(err) || attempt == sendMessageRetries-1 {
			return err
		}
		if sleepErr := sleepWithContext(ctx, backoff); sleepErr != nil {
			return sleepErr
		}
		backoff *= 2
		if backoff > sendMessageMaxDelay {
			backoff = sendMessageMaxDelay
		}
	}
	return lastErr
}

func (a *Adapter) sendMessageOnce(ctx context.Context, target Target, text string) error {
	payload := sendMessageRequest{
		ChatID:           target.ChatID,
		Text:             text,
		ReplyToMessageID: target.ReplyToMessage,
	}
	if target.ThreadID != 0 {
		payload.MessageThreadID = target.ThreadID
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/bot%s/sendMessage", a.baseURL, a.token)
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

	var payloadResponse apiResponse
	if err := json.Unmarshal(responseBody, &payloadResponse); err != nil {
		return err
	}
	if !payloadResponse.OK {
		return fmt.Errorf("telegram sendMessage failed: %s", strings.TrimSpace(payloadResponse.Description))
	}
	return nil
}

func isRetryableSendMessageError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(err.Error())), "telegram sendmessage failed:") {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "connection reset by peer") ||
		strings.Contains(text, "broken pipe") ||
		strings.Contains(text, "eof") ||
		strings.Contains(text, "timeout")
}

func isRetryableSetWebhookError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(err.Error())), "telegram setwebhook failed:") {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "connection reset by peer") ||
		strings.Contains(text, "broken pipe") ||
		strings.Contains(text, "eof") ||
		strings.Contains(text, "timeout") ||
		strings.Contains(text, "http status 5")
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *Adapter) currentOffset() int64 {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.nextUpdateID
}

func (a *Adapter) setOffset(offset int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.nextUpdateID = offset
}

func buildAllowedChatIDSet(values []string) map[int64]struct{} {
	if len(values) == 0 {
		return nil
	}

	allowed := make(map[int64]struct{}, len(values))
	for _, value := range values {
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			continue
		}
		allowed[parsed] = struct{}{}
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}

func buildGuildID(chatID int64, isDirect bool) string {
	if isDirect {
		return ""
	}
	return strconv.FormatInt(chatID, 10)
}

func buildThreadID(threadID int64) string {
	if threadID == 0 {
		return ""
	}
	return strconv.FormatInt(threadID, 10)
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

type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

type getUpdatesResponse struct {
	OK          bool             `json:"ok"`
	Result      []telegramUpdate `json:"result"`
	Description string           `json:"description"`
}

type telegramUpdate struct {
	UpdateID int64            `json:"update_id"`
	Message  *telegramMessage `json:"message"`
}

type telegramMessage struct {
	MessageID       int64        `json:"message_id"`
	MessageThreadID int64        `json:"message_thread_id"`
	Text            string       `json:"text"`
	Chat            telegramChat `json:"chat"`
	From            telegramUser `json:"from"`
}

type telegramChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type telegramUser struct {
	ID int64 `json:"id"`
}

type sendMessageRequest struct {
	ChatID           int64  `json:"chat_id"`
	Text             string `json:"text"`
	ReplyToMessageID int64  `json:"reply_to_message_id,omitempty"`
	MessageThreadID  int64  `json:"message_thread_id,omitempty"`
}

type setWebhookRequest struct {
	URL         string `json:"url"`
	SecretToken string `json:"secret_token,omitempty"`
}
