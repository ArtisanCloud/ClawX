package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	chatiface "clawx/internal/interfaces/chat"
)

const MaxMessageLength = 2048

const (
	sendMessageRetries  = 3
	sendMessageBackoff  = 300 * time.Millisecond
	sendMessageMaxDelay = 2 * time.Second
)

var (
	ErrMissingCorpID         = errors.New("wecom corp id is required")
	ErrMissingAgentID        = errors.New("wecom agent id is required")
	ErrMissingSecret         = errors.New("wecom secret is required")
	ErrMissingToken          = errors.New("wecom token is required")
	ErrMissingEncodingAESKey = errors.New("wecom encoding aes key is required")
	ErrInvalidEncodingAESKey = errors.New("invalid wecom encoding aes key")
	ErrInvalidAgentID        = errors.New("invalid wecom agent id")
	ErrMessageTooLong        = errors.New("wecom message exceeds limit")
	ErrUnknownSessionRoute   = errors.New("wecom session target is not bound")
	ErrInvalidWebhook        = errors.New("invalid wecom webhook request")
	ErrWebhookMethod         = errors.New("wecom webhook method is not allowed")
	ErrWebhookUnauthorized   = errors.New("wecom webhook signature mismatch")
	ErrWebhookDecryptFailed  = errors.New("wecom webhook decrypt failed")
	ErrWebhookReplayRejected = errors.New("wecom webhook replay rejected")
	ErrWebhookCorpIDMismatch = errors.New("wecom webhook corp id mismatch")
)

type Options struct {
	CorpID         string
	AgentID        string
	Secret         string
	Token          string
	EncodingAESKey string
	BaseURL        string
	HTTPClient     *http.Client
}

type Target struct {
	ToUser string
}

type InboundEnvelope struct {
	Message   chatiface.Message
	Target    Target
	EventID   string
	Timestamp string
}

type ParseResult struct {
	IsURLVerification bool
	URLVerification   string
	HasMessage        bool
	Envelope          InboundEnvelope
}

type Adapter struct {
	corpID  string
	agentID int64
	secret  string
	token   string
	aesKey  []byte
	baseURL string
	client  *http.Client

	mu             sync.Mutex
	sessionTargets map[string]Target
	accessToken    string
	accessTokenExp time.Time
}

func NewAdapter(options Options) (*Adapter, error) {
	corpID := strings.TrimSpace(options.CorpID)
	if corpID == "" {
		return nil, ErrMissingCorpID
	}
	agentIDRaw := strings.TrimSpace(options.AgentID)
	if agentIDRaw == "" {
		return nil, ErrMissingAgentID
	}
	agentID, err := strconv.ParseInt(agentIDRaw, 10, 64)
	if err != nil || agentID <= 0 {
		return nil, ErrInvalidAgentID
	}
	secret := strings.TrimSpace(options.Secret)
	if secret == "" {
		return nil, ErrMissingSecret
	}
	token := strings.TrimSpace(options.Token)
	if token == "" {
		return nil, ErrMissingToken
	}
	aesKey, err := decodeEncodingAESKey(options.EncodingAESKey)
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if baseURL == "" {
		baseURL = "https://qyapi.weixin.qq.com"
	}

	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}

	return &Adapter{
		corpID:         corpID,
		agentID:        agentID,
		secret:         secret,
		token:          token,
		aesKey:         aesKey,
		baseURL:        baseURL,
		client:         client,
		sessionTargets: make(map[string]Target),
	}, nil
}

func (a *Adapter) ParseWebhookRequest(r *http.Request) (ParseResult, error) {
	if r == nil {
		return ParseResult{}, ErrInvalidWebhook
	}
	method := strings.ToUpper(strings.TrimSpace(r.Method))
	if method != http.MethodGet && method != http.MethodPost {
		return ParseResult{}, ErrWebhookMethod
	}

	query := r.URL.Query()
	timestamp := strings.TrimSpace(query.Get("timestamp"))
	nonce := strings.TrimSpace(query.Get("nonce"))
	signature := strings.TrimSpace(query.Get("msg_signature"))
	if timestamp == "" || nonce == "" || signature == "" {
		return ParseResult{}, ErrWebhookUnauthorized
	}
	if !isFreshTimestamp(timestamp, 15*time.Minute) {
		return ParseResult{}, ErrWebhookReplayRejected
	}

	if method == http.MethodGet {
		echoStr := strings.TrimSpace(query.Get("echostr"))
		if echoStr == "" {
			return ParseResult{}, ErrInvalidWebhook
		}
		if !verifySignature(a.token, timestamp, nonce, echoStr, signature) {
			return ParseResult{}, ErrWebhookUnauthorized
		}
		plain, err := a.decrypt(echoStr)
		if err != nil {
			return ParseResult{}, err
		}
		return ParseResult{IsURLVerification: true, URLVerification: strings.TrimSpace(string(plain))}, nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return ParseResult{}, fmt.Errorf("%w: read body: %v", ErrInvalidWebhook, err)
	}

	var callback encryptedCallbackXML
	if err := xml.Unmarshal(body, &callback); err != nil {
		return ParseResult{}, fmt.Errorf("%w: decode callback xml: %v", ErrInvalidWebhook, err)
	}
	encryptText := strings.TrimSpace(callback.Encrypt)
	if encryptText == "" {
		return ParseResult{}, ErrInvalidWebhook
	}

	if !verifySignature(a.token, timestamp, nonce, encryptText, signature) {
		return ParseResult{}, ErrWebhookUnauthorized
	}
	plain, err := a.decrypt(encryptText)
	if err != nil {
		return ParseResult{}, err
	}

	var message wecomMessageXML
	if err := xml.Unmarshal(plain, &message); err != nil {
		return ParseResult{}, fmt.Errorf("%w: decode message xml: %v", ErrInvalidWebhook, err)
	}
	if strings.ToLower(strings.TrimSpace(message.MsgType)) != "text" {
		return ParseResult{HasMessage: false}, nil
	}
	content := strings.TrimSpace(message.Content)
	if content == "" {
		return ParseResult{HasMessage: false}, nil
	}
	userID := strings.TrimSpace(message.FromUserName)
	if userID == "" {
		return ParseResult{}, ErrInvalidWebhook
	}

	chatID := firstNonEmpty(message.ChatID, message.ChatId)
	normalized, err := chatiface.NormalizeWeComTextEvent(chatiface.WeComNormalizeInput{
		FromUserID: userID,
		ChatID:     chatID,
		Text:       content,
	})
	if err != nil {
		return ParseResult{}, err
	}

	eventID := strings.TrimSpace(message.MsgID)
	if eventID == "" {
		eventID = strings.TrimSpace(message.MsgId)
	}
	if eventID == "" {
		eventID = buildFallbackEventID(timestamp, nonce, encryptText)
	}

	return ParseResult{
		HasMessage: true,
		Envelope: InboundEnvelope{
			Message:   normalized,
			Target:    Target{ToUser: userID},
			EventID:   eventID,
			Timestamp: timestamp,
		},
	}, nil
}

func (a *Adapter) BindSession(sessionID string, target Target) {
	sessionID = strings.TrimSpace(sessionID)
	target.ToUser = strings.TrimSpace(target.ToUser)
	if sessionID == "" || target.ToUser == "" {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessionTargets[sessionID] = target
}

func (a *Adapter) SendDirect(ctx context.Context, target Target, text string) error {
	target.ToUser = strings.TrimSpace(target.ToUser)
	content := strings.TrimSpace(text)
	if target.ToUser == "" || content == "" {
		return nil
	}

	for _, chunk := range splitText(content, MaxMessageLength) {
		if err := a.sendMessage(ctx, target.ToUser, chunk); err != nil {
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
	return a.sendMessage(ctx, target.ToUser, chunk)
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

func (a *Adapter) sendMessage(ctx context.Context, toUser, text string) error {
	if len([]rune(text)) > MaxMessageLength {
		return ErrMessageTooLong
	}

	backoff := sendMessageBackoff
	var lastErr error
	for attempt := 0; attempt < sendMessageRetries; attempt++ {
		err := a.sendMessageOnce(ctx, toUser, text)
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

func (a *Adapter) sendMessageOnce(ctx context.Context, toUser, text string) error {
	accessToken, err := a.getAccessToken(ctx)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"touser":  strings.TrimSpace(toUser),
		"msgtype": "text",
		"agentid": a.agentID,
		"text": map[string]string{
			"content": text,
		},
		"safe": 0,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", a.baseURL, accessToken)
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
		return fmt.Errorf("wecom send message http status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var apiResp wecomAPIResponse
	if err := json.Unmarshal(responseBody, &apiResp); err != nil {
		return err
	}
	if apiResp.ErrCode != 0 {
		return fmt.Errorf("wecom send message failed: errcode=%d errmsg=%s", apiResp.ErrCode, strings.TrimSpace(apiResp.ErrMsg))
	}
	return nil
}

func (a *Adapter) getAccessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	if strings.TrimSpace(a.accessToken) != "" && time.Now().Before(a.accessTokenExp) {
		token := a.accessToken
		a.mu.Unlock()
		return token, nil
	}
	a.mu.Unlock()

	endpoint := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s", a.baseURL, a.corpID, a.secret)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusInternalServerError {
		return "", fmt.Errorf("wecom get token http status %d", resp.StatusCode)
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var tokenResp wecomAccessTokenResponse
	if err := json.Unmarshal(responseBody, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.ErrCode != 0 || strings.TrimSpace(tokenResp.AccessToken) == "" {
		return "", fmt.Errorf("wecom get token failed: errcode=%d errmsg=%s", tokenResp.ErrCode, strings.TrimSpace(tokenResp.ErrMsg))
	}

	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 7200 * time.Second
	}
	expiresAt := time.Now().Add(expiresIn - 60*time.Second)

	a.mu.Lock()
	a.accessToken = strings.TrimSpace(tokenResp.AccessToken)
	a.accessTokenExp = expiresAt
	token := a.accessToken
	a.mu.Unlock()
	return token, nil
}

func decodeEncodingAESKey(raw string) ([]byte, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, ErrMissingEncodingAESKey
	}
	decoded, err := base64.StdEncoding.DecodeString(trimmed + "=")
	if err != nil || len(decoded) != 32 {
		return nil, ErrInvalidEncodingAESKey
	}
	return decoded, nil
}

func (a *Adapter) decrypt(encrypted string) ([]byte, error) {
	cipherData, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encrypted))
	if err != nil {
		return nil, fmt.Errorf("%w: decode base64", ErrWebhookDecryptFailed)
	}
	if len(cipherData) == 0 || len(cipherData)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("%w: invalid cipher block", ErrWebhookDecryptFailed)
	}

	block, err := aes.NewCipher(a.aesKey)
	if err != nil {
		return nil, fmt.Errorf("%w: init cipher", ErrWebhookDecryptFailed)
	}

	plain := make([]byte, len(cipherData))
	mode := cipher.NewCBCDecrypter(block, a.aesKey[:aes.BlockSize])
	mode.CryptBlocks(plain, cipherData)

	plain, err = pkcs7Unpad(plain, aes.BlockSize)
	if err != nil {
		return nil, fmt.Errorf("%w: unpad", ErrWebhookDecryptFailed)
	}
	if len(plain) < 20 {
		return nil, fmt.Errorf("%w: payload too short", ErrWebhookDecryptFailed)
	}

	msgLen := int(binary.BigEndian.Uint32(plain[16:20]))
	if msgLen < 0 || 20+msgLen > len(plain) {
		return nil, fmt.Errorf("%w: invalid payload length", ErrWebhookDecryptFailed)
	}
	message := plain[20 : 20+msgLen]
	receiveID := strings.TrimSpace(string(plain[20+msgLen:]))
	if receiveID != strings.TrimSpace(a.corpID) {
		return nil, ErrWebhookCorpIDMismatch
	}

	return message, nil
}

func verifySignature(token, timestamp, nonce, encrypted, provided string) bool {
	parts := []string{strings.TrimSpace(token), strings.TrimSpace(timestamp), strings.TrimSpace(nonce), strings.TrimSpace(encrypted)}
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	expected := hex.EncodeToString(h.Sum(nil))
	return subtleEqual(expected, strings.TrimSpace(provided))
}

func subtleEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if len(left) != len(right) {
		return false
	}
	var diff byte
	for i := 0; i < len(left); i++ {
		diff |= left[i] ^ right[i]
	}
	return diff == 0
}

func isFreshTimestamp(raw string, tolerance time.Duration) bool {
	seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return false
	}
	eventTime := time.Unix(seconds, 0)
	delta := time.Since(eventTime)
	if delta < 0 {
		delta = -delta
	}
	return delta <= tolerance
}

func buildFallbackEventID(timestamp, nonce, encryptText string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(strings.TrimSpace(timestamp) + "|" + strings.TrimSpace(nonce) + "|" + strings.TrimSpace(encryptText)))
	return hex.EncodeToString(h.Sum(nil))
}

func isRetryableSendMessageError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() || netErr.Temporary() {
			return true
		}
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	if strings.Contains(message, "connection reset") ||
		strings.Contains(message, "broken pipe") ||
		strings.Contains(message, "timeout") ||
		strings.Contains(message, "status 5") ||
		strings.Contains(message, "server error") {
		return true
	}
	return false
}

func sleepWithContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func splitText(content string, maxLen int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	runes := []rune(content)
	if len(runes) <= maxLen {
		return []string{content}
	}
	chunks := make([]string, 0, (len(runes)/maxLen)+1)
	for start := 0; start < len(runes); start += maxLen {
		end := start + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}

func pkcs7Unpad(data []byte, blockSize int) ([]byte, error) {
	if len(data) == 0 || len(data)%blockSize != 0 {
		return nil, errors.New("invalid padded data")
	}
	pad := int(data[len(data)-1])
	if pad <= 0 || pad > blockSize || pad > len(data) {
		return nil, errors.New("invalid padding size")
	}
	for i := len(data) - pad; i < len(data); i++ {
		if int(data[i]) != pad {
			return nil, errors.New("invalid padding content")
		}
	}
	return data[:len(data)-pad], nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type encryptedCallbackXML struct {
	XMLName xml.Name `xml:"xml"`
	Encrypt string   `xml:"Encrypt"`
}

type wecomMessageXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   string   `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgID"`
	MsgId        string   `xml:"MsgId"`
	ChatID       string   `xml:"ChatID"`
	ChatId       string   `xml:"ChatId"`
	AgentID      string   `xml:"AgentID"`
}

type wecomAccessTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type wecomAPIResponse struct {
	ErrCode int    `json:"errcode"`
	ErrMsg  string `json:"errmsg"`
}
