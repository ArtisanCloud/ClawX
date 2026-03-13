package wecom

import (
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParseWebhookRequestRejectsReplayEvent(t *testing.T) {
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
		MsgID:        "msg-replay-1",
	}
	plainXML, err := xml.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain xml: %v", err)
	}
	encrypted, err := encryptForTest(aesKey, "ww_test_corp", plainXML)
	if err != nil {
		t.Fatalf("encrypt message: %v", err)
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "nonce-replay"
	signature := signWeCom("verify-token", timestamp, nonce, encrypted)
	query := url.Values{}
	query.Set("msg_signature", signature)
	query.Set("timestamp", timestamp)
	query.Set("nonce", nonce)
	body := fmt.Sprintf("<xml><Encrypt><![CDATA[%s]]></Encrypt></xml>", encrypted)

	firstReq := httptest.NewRequest(http.MethodPost, "/webhooks/wecom?"+query.Encode(), strings.NewReader(body))
	first, err := adapter.ParseWebhookRequest(firstReq)
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	if !first.HasMessage {
		t.Fatalf("expected first message event")
	}
	if first.Envelope.EventID != "msg-replay-1" {
		t.Fatalf("unexpected event id: %q", first.Envelope.EventID)
	}

	replayReq := httptest.NewRequest(http.MethodPost, "/webhooks/wecom?"+query.Encode(), strings.NewReader(body))
	_, err = adapter.ParseWebhookRequest(replayReq)
	if !errors.Is(err, ErrWebhookReplayRejected) {
		t.Fatalf("expected ErrWebhookReplayRejected, got %v", err)
	}
}
