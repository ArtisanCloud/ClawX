package wecom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseWebhookRequestURLVerificationSuccess(t *testing.T) {
	adapter := newTestAdapter(t)
	aesKey, err := decodeEncodingAESKey("abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	if err != nil {
		t.Fatalf("decode aes key: %v", err)
	}

	echoPlain := []byte("echo-ok")
	echoEncrypted, err := encryptForTest(aesKey, "ww_test_corp", echoPlain)
	if err != nil {
		t.Fatalf("encrypt echo: %v", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-1"
	signature := signWeCom("verify-token", timestamp, nonce, echoEncrypted)
	query := url.Values{}
	query.Set("msg_signature", signature)
	query.Set("timestamp", timestamp)
	query.Set("nonce", nonce)
	query.Set("echostr", echoEncrypted)
	req := httptest.NewRequest(http.MethodGet, "/webhooks/wecom?"+query.Encode(), nil)

	result, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("parse webhook request: %v", err)
	}
	if !result.IsURLVerification {
		t.Fatalf("expected url verification flow")
	}
	if result.URLVerification != "echo-ok" {
		t.Fatalf("unexpected verification text: %q", result.URLVerification)
	}
}

func TestParseWebhookRequestSignatureMismatch(t *testing.T) {
	adapter := newTestAdapter(t)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	query := url.Values{}
	query.Set("msg_signature", "invalid")
	query.Set("timestamp", timestamp)
	query.Set("nonce", "nonce-2")
	query.Set("echostr", "abc")
	req := httptest.NewRequest(http.MethodGet, "/webhooks/wecom?"+query.Encode(), nil)

	_, err := adapter.ParseWebhookRequest(req)
	if err != ErrWebhookUnauthorized {
		t.Fatalf("expected ErrWebhookUnauthorized, got %v", err)
	}
}

func TestParseWebhookRequestTextMessageSuccess(t *testing.T) {
	adapter := newTestAdapter(t)
	aesKey, err := decodeEncodingAESKey("abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG")
	if err != nil {
		t.Fatalf("decode aes key: %v", err)
	}

	plain := wecomMessageXML{
		ToUserName:   "ww_test_corp",
		FromUserName: "zhangsan",
		CreateTime:   strconv.FormatInt(time.Now().Unix(), 10),
		MsgType:      "text",
		Content:      "/new",
		MsgID:        "msg-1",
	}
	plainXML, err := xml.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain xml: %v", err)
	}
	encrypted, err := encryptForTest(aesKey, "ww_test_corp", plainXML)
	if err != nil {
		t.Fatalf("encrypt message: %v", err)
	}

	body := fmt.Sprintf("<xml><Encrypt><![CDATA[%s]]></Encrypt></xml>", encrypted)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-3"
	signature := signWeCom("verify-token", timestamp, nonce, encrypted)
	query := url.Values{}
	query.Set("msg_signature", signature)
	query.Set("timestamp", timestamp)
	query.Set("nonce", nonce)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/wecom?"+query.Encode(), strings.NewReader(body))

	result, err := adapter.ParseWebhookRequest(req)
	if err != nil {
		t.Fatalf("parse webhook request: %v", err)
	}
	if !result.HasMessage {
		t.Fatalf("expected message event")
	}
	if result.Envelope.Message.Channel != "wecom" {
		t.Fatalf("unexpected channel: %q", result.Envelope.Message.Channel)
	}
	if result.Envelope.Message.Text != "/new" {
		t.Fatalf("unexpected text: %q", result.Envelope.Message.Text)
	}
	if result.Envelope.Target.ToUser != "zhangsan" {
		t.Fatalf("unexpected target user: %q", result.Envelope.Target.ToUser)
	}
}

func newTestAdapter(t *testing.T) *Adapter {
	t.Helper()
	adapter, err := NewAdapter(Options{
		CorpID:         "ww_test_corp",
		AgentID:        "1000002",
		Secret:         "corp-secret",
		Token:          "verify-token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	return adapter
}

func signWeCom(token, timestamp, nonce, encrypted string) string {
	items := []string{token, timestamp, nonce, encrypted}
	sort.Strings(items)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(items, "")))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func encryptForTest(aesKey []byte, corpID string, plain []byte) (string, error) {
	randomPrefix := make([]byte, 16)
	if _, err := rand.Read(randomPrefix); err != nil {
		return "", err
	}

	payload := make([]byte, 0, 20+len(plain)+len(corpID))
	payload = append(payload, randomPrefix...)
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, uint32(len(plain)))
	payload = append(payload, lengthBytes...)
	payload = append(payload, plain...)
	payload = append(payload, []byte(corpID)...)

	padded := pkcs7Pad(payload, aes.BlockSize)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, aesKey[:aes.BlockSize]).CryptBlocks(cipherText, padded)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - (len(data) % blockSize)
	if padding == 0 {
		padding = blockSize
	}
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}
