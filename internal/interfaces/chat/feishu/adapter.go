package feishu

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	chatiface "clawx/internal/interfaces/chat"
)

const MaxMessageLength = 4000

const (
	sendMessageRetries  = 3
	sendMessageBackoff  = 300 * time.Millisecond
	sendMessageMaxDelay = 2 * time.Second
)

var (
	ErrMissingAppID          = errors.New("feishu app id is required")
	ErrMissingAppSecret      = errors.New("feishu app secret is required")
	ErrMissingVerifyToken    = errors.New("feishu verification token is required")
	ErrMessageTooLong        = errors.New("feishu message exceeds limit")
	ErrUnknownSessionRoute   = errors.New("feishu session target is not bound")
	ErrInvalidWebhook        = errors.New("invalid feishu webhook request")
	ErrWebhookMethod         = errors.New("feishu webhook method is not allowed")
	ErrWebhookUnauthorized   = errors.New("feishu webhook signature mismatch")
	ErrVerificationTokenFail = errors.New("feishu verification token mismatch")
)

type Options struct {
	AppID             string
	AppSecret         string
	VerificationToken string
	EncryptKey        string
	BaseURL           string
	HTTPClient        *http.Client
}

type Target struct {
	ChatID string
}

type InboundEnvelope struct {
	Message chatiface.Message
	Target  Target
	EventID string
}

type ParseResult struct {
	IsChallenge bool
	Challenge   string
	HasMessage  bool
	Envelope    InboundEnvelope
}

type Adapter struct {
	appID             string
	appSecret         string
	verificationToken string
	encryptKey        string
	baseURL           string
	client            *http.Client

	mu             sync.Mutex
	sessionTargets map[string]Target
	tenantToken    string
	tenantTokenExp time.Time
}

func NewAdapter(options Options) (*Adapter, error) {
	appID := strings.TrimSpace(options.AppID)
	if appID == "" {
		return nil, ErrMissingAppID
	}
	appSecret := strings.TrimSpace(options.AppSecret)
	if appSecret == "" {
		return nil, ErrMissingAppSecret
	}
	verificationToken := strings.TrimSpace(options.VerificationToken)
	if verificationToken == "" {
		return nil, ErrMissingVerifyToken
	}

	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://open.feishu.cn"
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &Adapter{
		appID:             appID,
		appSecret:         appSecret,
		verificationToken: verificationToken,
		encryptKey:        strings.TrimSpace(options.EncryptKey),
		baseURL:           baseURL,
		client:            client,
		sessionTargets:    make(map[string]Target),
	}, nil
}

func (a *Adapter) ParseWebhookRequest(r *http.Request) (ParseResult, error) {
	if r == nil {
		return ParseResult{}, ErrInvalidWebhook
	}
	if r.Method != http.MethodPost {
		return ParseResult{}, ErrWebhookMethod
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return ParseResult{}, fmt.Errorf("%w: read body: %v", ErrInvalidWebhook, err)
	}

	if err := a.verifySignature(r, body); err != nil {
		return ParseResult{}, err
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return ParseResult{}, fmt.Errorf("%w: decode payload: %v", ErrInvalidWebhook, err)
	}

	if strings.EqualFold(strings.TrimSpace(payload.Type), "url_verification") {
		if err := a.verifyToken(payload.Token); err != nil {
			return ParseResult{}, err
		}
		return ParseResult{
			IsChallenge: true,
			Challenge:   strings.TrimSpace(payload.Challenge),
		}, nil
	}

	if strings.TrimSpace(payload.Header.Token) != "" {
		if err := a.verifyToken(payload.Header.Token); err != nil {
			return ParseResult{}, err
		}
	}

	envelope, ok, err := a.normalizeEvent(payload)
	if err != nil {
		return ParseResult{}, err
	}
	if !ok {
		return ParseResult{HasMessage: false}, nil
	}
	return ParseResult{
		HasMessage: true,
		Envelope:   envelope,
	}, nil
}

func (a *Adapter) BindSession(sessionID string, target Target) {
	sessionID = strings.TrimSpace(sessionID)
	target.ChatID = strings.TrimSpace(target.ChatID)
	if sessionID == "" || target.ChatID == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionTargets[sessionID] = target
}

func (a *Adapter) SendDirect(ctx context.Context, target Target, text string) error {
	target.ChatID = strings.TrimSpace(target.ChatID)
	content := strings.TrimSpace(text)
	if target.ChatID == "" || content == "" {
		return nil
	}

	for _, chunk := range splitText(content, MaxMessageLength) {
		if err := a.sendMessage(ctx, target.ChatID, chunk); err != nil {
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
	return a.sendMessage(ctx, target.ChatID, chunk)
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

func (a *Adapter) verifySignature(r *http.Request, body []byte) error {
	timestamp := strings.TrimSpace(r.Header.Get("X-Lark-Request-Timestamp"))
	nonce := strings.TrimSpace(r.Header.Get("X-Lark-Request-Nonce"))
	signature := strings.TrimSpace(r.Header.Get("X-Lark-Signature"))
	if timestamp == "" || nonce == "" || signature == "" {
		return ErrWebhookUnauthorized
	}

	raw := timestamp + nonce + string(body)
	mac := hmac.New(sha256.New, []byte(a.appSecret))
	_, _ = mac.Write([]byte(raw))
	sum := mac.Sum(nil)

	expectedHex := strings.ToLower(hex.EncodeToString(sum))
	expectedBase64 := strings.TrimSpace(base64.StdEncoding.EncodeToString(sum))
	normalizedSignature := strings.ToLower(signature)

	if subtle.ConstantTimeCompare([]byte(normalizedSignature), []byte(expectedHex)) == 1 {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(signature), []byte(expectedBase64)) == 1 {
		return nil
	}
	return ErrWebhookUnauthorized
}

func (a *Adapter) verifyToken(token string) error {
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(token)), []byte(a.verificationToken)) != 1 {
		return ErrVerificationTokenFail
	}
	return nil
}

func (a *Adapter) normalizeEvent(payload webhookPayload) (InboundEnvelope, bool, error) {
	if strings.TrimSpace(payload.Event.Message.MessageType) != "text" {
		return InboundEnvelope{}, false, nil
	}
	textContent, err := decodeText(payload.Event.Message.Content)
	if err != nil {
		return InboundEnvelope{}, false, fmt.Errorf("%w: decode text content: %v", ErrInvalidWebhook, err)
	}
	if strings.TrimSpace(textContent) == "" {
		return InboundEnvelope{}, false, nil
	}

	userID := firstNonEmpty(
		payload.Event.Sender.SenderID.OpenID,
		payload.Event.Sender.SenderID.UserID,
		payload.Event.Sender.SenderID.UnionID,
	)
	message, err := chatiface.NormalizeFeishuTextEvent(chatiface.FeishuNormalizeInput{
		ChatID:   payload.Event.Message.ChatID,
		ChatType: payload.Event.Message.ChatType,
		UserID:   userID,
		Text:     textContent,
	})
	if err != nil {
		return InboundEnvelope{}, false, err
	}

	return InboundEnvelope{
		Message: message,
		Target: Target{
			ChatID: strings.TrimSpace(payload.Event.Message.ChatID),
		},
		EventID: strings.TrimSpace(payload.Header.EventID),
	}, true, nil
}

func decodeText(raw string) (string, error) {
	var payload struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return "", err
	}
	return strings.TrimSpace(payload.Text), nil
}

func (a *Adapter) sendMessage(ctx context.Context, chatID, text string) error {
	if len([]rune(text)) > MaxMessageLength {
		return ErrMessageTooLong
	}
	backoff := sendMessageBackoff
	var lastErr error
	for attempt := 0; attempt < sendMessageRetries; attempt++ {
		err := a.sendMessageOnce(ctx, chatID, text)
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

func (a *Adapter) sendMessageOnce(ctx context.Context, chatID, text string) error {
	accessToken, err := a.tenantAccessToken(ctx)
	if err != nil {
		return err
	}

	contentPayload, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return err
	}
	body, err := json.Marshal(map[string]string{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(contentPayload),
	})
	if err != nil {
		return err
	}

	endpoint := a.baseURL + "/open-apis/im/v1/messages?receive_id_type=chat_id"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return fmt.Errorf("feishu send message http status %d", resp.StatusCode)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var payload struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return err
	}
	if payload.Code != 0 {
		return fmt.Errorf("feishu send message failed: code=%d msg=%s", payload.Code, strings.TrimSpace(payload.Msg))
	}
	return nil
}

func (a *Adapter) tenantAccessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	token := strings.TrimSpace(a.tenantToken)
	exp := a.tenantTokenExp
	a.mu.Unlock()

	if token != "" && time.Now().Before(exp.Add(-30*time.Second)) {
		return token, nil
	}

	endpoint := a.baseURL + "/open-apis/auth/v3/tenant_access_token/internal"
	body, err := json.Marshal(map[string]string{
		"app_id":     a.appID,
		"app_secret": a.appSecret,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var payload struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"`
	}
	if err := json.Unmarshal(respBody, &payload); err != nil {
		return "", err
	}
	if payload.Code != 0 {
		return "", fmt.Errorf("feishu get tenant token failed: code=%d msg=%s", payload.Code, strings.TrimSpace(payload.Msg))
	}
	token = strings.TrimSpace(payload.TenantAccessToken)
	if token == "" {
		return "", fmt.Errorf("feishu get tenant token failed: empty token")
	}
	expiresIn := payload.Expire
	if expiresIn <= 0 {
		expiresIn = 7200
	}

	a.mu.Lock()
	a.tenantToken = token
	a.tenantTokenExp = time.Now().Add(time.Duration(expiresIn) * time.Second)
	a.mu.Unlock()

	return token, nil
}

func isRetryableSendMessageError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(err.Error())), "feishu send message failed:") {
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type webhookPayload struct {
	Type      string              `json:"type"`
	Token     string              `json:"token"`
	Challenge string              `json:"challenge"`
	Header    webhookHeader       `json:"header"`
	Event     webhookMessageEvent `json:"event"`
}

type webhookHeader struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	Token     string `json:"token"`
}

type webhookMessageEvent struct {
	Sender  webhookSender  `json:"sender"`
	Message webhookMessage `json:"message"`
}

type webhookSender struct {
	SenderID webhookSenderID `json:"sender_id"`
}

type webhookSenderID struct {
	OpenID  string `json:"open_id"`
	UserID  string `json:"user_id"`
	UnionID string `json:"union_id"`
}

type webhookMessage struct {
	MessageID   string `json:"message_id"`
	MessageType string `json:"message_type"`
	ChatID      string `json:"chat_id"`
	ChatType    string `json:"chat_type"`
	Content     string `json:"content"`
}
