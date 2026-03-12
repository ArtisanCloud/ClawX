package integration

import (
	"context"
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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"clawx/internal/application/service"
	wecomchat "clawx/internal/interfaces/chat/wecom"
)

func TestWeComControlFlow(t *testing.T) {
	stack := newTestRuntime(t)
	adapter, err := wecomchat.NewAdapter(wecomchat.Options{
		CorpID:         "ww_test_corp",
		AgentID:        "1000002",
		Secret:         "corp-secret",
		Token:          "verify-token",
		EncodingAESKey: "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG",
	})
	if err != nil {
		t.Fatalf("new wecom adapter: %v", err)
	}

	t.Run("control_commands_keep_semantics_consistent", func(t *testing.T) {
		commandIndex := 0
		execute := func(commandText string) service.ControlFlowResult {
			commandIndex++
			msgID := fmt.Sprintf("msg-%d", commandIndex)
			timestamp := strconv.FormatInt(time.Now().Unix(), 10)
			nonce := fmt.Sprintf("nonce-%d", commandIndex)
			body, signature := buildSignedWeComEvent(t, "verify-token", "ww_test_corp", "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFG", timestamp, nonce, msgID, commandText)
			url := fmt.Sprintf("/webhooks/wecom?msg_signature=%s&timestamp=%s&nonce=%s", signature, timestamp, nonce)
			req := httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))

			result, err := adapter.ParseWebhookRequest(req)
			if err != nil {
				t.Fatalf("parse webhook: %v", err)
			}
			if !result.HasMessage {
				t.Fatalf("expected message payload")
			}

			decision, err := stack.router.Route(context.Background(), result.Envelope.Message)
			if err != nil {
				t.Fatalf("route wecom message: %v", err)
			}
			if decision.Kind != service.DecisionControl {
				t.Fatalf("unexpected decision kind: %s", decision.Kind)
			}

			control, err := stack.router.HandleControlCommand(context.Background(), decision.Command, decision.ConversationID, decision.WindowID)
			if err != nil {
				t.Fatalf("handle control command: %v", err)
			}
			return control
		}

		current0 := execute("/current")
		if !current0.CurrentChecked || current0.CurrentSession != nil {
			t.Fatalf("expected no current session before /new")
		}

		createdA := execute("/new")
		if createdA.CreatedSessionID == "" {
			t.Fatalf("expected created session id A")
		}
		createdB := execute("/new")
		if createdB.CreatedSessionID == "" || createdB.CreatedSessionID == createdA.CreatedSessionID {
			t.Fatalf("expected distinct created session id B")
		}

		listed := execute("/list")
		if len(listed.Sessions) < 2 {
			t.Fatalf("expected at least two sessions in /list")
		}
		if listed.CurrentSession == nil || listed.CurrentSession.ID != createdB.CreatedSessionID {
			t.Fatalf("expected current session to be B after second /new")
		}

		switched := execute("/switch " + createdA.CreatedSessionID)
		if switched.SwitchedSessionID != createdA.CreatedSessionID {
			t.Fatalf("unexpected switch target: got=%q want=%q", switched.SwitchedSessionID, createdA.CreatedSessionID)
		}

		current1 := execute("/current")
		if current1.CurrentSession == nil || current1.CurrentSession.ID != createdA.CreatedSessionID {
			t.Fatalf("expected current session to be A after /switch")
		}

		resumed := execute("/resume " + createdB.CreatedSessionID)
		if resumed.ResumedSessionID != createdB.CreatedSessionID {
			t.Fatalf("unexpected resumed session: got=%q want=%q", resumed.ResumedSessionID, createdB.CreatedSessionID)
		}

		current2 := execute("/current")
		if current2.CurrentSession == nil || current2.CurrentSession.ID != createdB.CreatedSessionID {
			t.Fatalf("expected current session to be B after /resume")
		}

		cancelled := execute("/cancel")
		if cancelled.CancelledSessionID != createdB.CreatedSessionID {
			t.Fatalf("unexpected cancelled session: got=%q want=%q", cancelled.CancelledSessionID, createdB.CreatedSessionID)
		}
		if !cancelled.CancelNoop {
			t.Fatalf("expected /cancel noop on idle session")
		}
	})
}

func buildSignedWeComEvent(t *testing.T, token, corpID, encodingAESKey, timestamp, nonce, msgID, text string) (string, string) {
	t.Helper()

	plain := weComTextMessageXML{
		ToUserName:   corpID,
		FromUserName: "zhangsan",
		CreateTime:   timestamp,
		MsgType:      "text",
		Content:      text,
		MsgID:        msgID,
	}
	plainXML, err := xml.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal plain xml: %v", err)
	}

	aesKey, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		t.Fatalf("decode aes key: %v", err)
	}
	encrypted, err := encryptWeComPayload(aesKey, corpID, plainXML)
	if err != nil {
		t.Fatalf("encrypt payload: %v", err)
	}

	signature := signWeComPayload(token, timestamp, nonce, encrypted)
	body := fmt.Sprintf("<xml><Encrypt><![CDATA[%s]]></Encrypt></xml>", encrypted)
	return body, signature
}

func signWeComPayload(token, timestamp, nonce, encrypted string) string {
	parts := []string{token, timestamp, nonce, encrypted}
	sort.Strings(parts)
	h := sha1.New()
	_, _ = h.Write([]byte(strings.Join(parts, "")))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func encryptWeComPayload(aesKey []byte, corpID string, plain []byte) (string, error) {
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

	padding := aes.BlockSize - (len(payload) % aes.BlockSize)
	if padding == 0 {
		padding = aes.BlockSize
	}
	for i := 0; i < padding; i++ {
		payload = append(payload, byte(padding))
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return "", err
	}
	cipherText := make([]byte, len(payload))
	cipher.NewCBCEncrypter(block, aesKey[:aes.BlockSize]).CryptBlocks(cipherText, payload)
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

type weComTextMessageXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   string   `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgID        string   `xml:"MsgID"`
}
