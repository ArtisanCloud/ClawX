package wecom

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
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

func TestSendLocalFileUploadsMediaThenSendsFileMessage(t *testing.T) {
	var calledGetToken bool
	var calledUpload bool
	var calledSend bool
	var gotUploadFilename string
	var gotUploadBody string
	var gotMediaIDInSend string

	adapter, err := NewAdapter(Options{
		CorpID:         "ww_test_corp",
		AgentID:        "1000002",
		Secret:         "corp-secret",
		Token:          "verify-token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
		BaseURL:        "https://qyapi.weixin.qq.com",
		HTTPClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch {
				case strings.Contains(req.URL.Path, "/cgi-bin/gettoken"):
					calledGetToken = true
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","access_token":"token-abc","expires_in":7200}`)),
					}, nil
				case strings.Contains(req.URL.Path, "/cgi-bin/media/upload"):
					calledUpload = true
					if err := req.ParseMultipartForm(2 * 1024 * 1024); err != nil {
						t.Fatalf("parse upload multipart: %v", err)
					}
					file, header, err := req.FormFile("media")
					if err != nil {
						t.Fatalf("read upload media file: %v", err)
					}
					defer file.Close()
					body, err := io.ReadAll(file)
					if err != nil {
						t.Fatalf("read upload media body: %v", err)
					}
					gotUploadFilename = header.Filename
					gotUploadBody = string(body)
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok","media_id":"MEDIA123"}`)),
					}, nil
				case strings.Contains(req.URL.Path, "/cgi-bin/message/send"):
					calledSend = true
					payload, err := io.ReadAll(req.Body)
					if err != nil {
						t.Fatalf("read send body: %v", err)
					}
					if !strings.Contains(string(payload), `"msgtype":"file"`) {
						t.Fatalf("unexpected send payload: %s", string(payload))
					}
					if !strings.Contains(string(payload), `"media_id":"MEDIA123"`) {
						t.Fatalf("missing media id in send payload: %s", string(payload))
					}
					gotMediaIDInSend = "MEDIA123"
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"errcode":0,"errmsg":"ok"}`)),
					}, nil
				default:
					return &http.Response{
						StatusCode: http.StatusNotFound,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader(`{"errcode":404,"errmsg":"not found"}`)),
					}, nil
				}
			}),
		},
	})
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test-long.png")
	if err := os.WriteFile(path, []byte("png-bytes"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	if err := adapter.SendLocalFile(context.Background(), Target{ToUser: "zhangsan"}, path, "ignored caption"); err != nil {
		t.Fatalf("send local file: %v", err)
	}

	if !calledGetToken || !calledUpload || !calledSend {
		t.Fatalf("unexpected call chain: getToken=%v upload=%v send=%v", calledGetToken, calledUpload, calledSend)
	}
	if gotUploadFilename != "test-long.png" {
		t.Fatalf("unexpected upload filename: %q", gotUploadFilename)
	}
	if gotUploadBody != "png-bytes" {
		t.Fatalf("unexpected upload body: %q", gotUploadBody)
	}
	if gotMediaIDInSend != "MEDIA123" {
		t.Fatalf("unexpected media id in send: %q", gotMediaIDInSend)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
