package integration

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"synapsex/internal/application/service"
	feishuchat "synapsex/internal/interfaces/chat/feishu"
)

func TestFeishuControlFlow(t *testing.T) {
	stack := newTestRuntime(t)
	adapter, err := feishuchat.NewAdapter(feishuchat.Options{
		AppID:             "cli_xxx",
		AppSecret:         "app-secret",
		VerificationToken: "verify-token",
	})
	if err != nil {
		t.Fatalf("new feishu adapter: %v", err)
	}

	t.Run("control_commands_keep_semantics_consistent", func(t *testing.T) {
		commandIndex := 0
		execute := func(commandText string) service.ControlFlowResult {
			commandIndex++
			body := buildFeishuTextEventBody(
				fmt.Sprintf("evt-%d", commandIndex),
				fmt.Sprintf("om-%d", commandIndex),
				"oc_1",
				"p2p",
				"ou_user_1",
				"verify-token",
				commandText,
			)
			req := buildFeishuSignedRequest("app-secret", body)
			result, err := adapter.ParseWebhookRequest(req)
			if err != nil {
				t.Fatalf("parse webhook: %v", err)
			}
			if !result.HasMessage {
				t.Fatalf("expected message payload")
			}

			decision, err := stack.router.Route(context.Background(), result.Envelope.Message)
			if err != nil {
				t.Fatalf("route feishu message: %v", err)
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

		list := execute("/list")
		if len(list.Sessions) < 2 {
			t.Fatalf("expected at least two sessions in /list")
		}
		if list.CurrentSession == nil || list.CurrentSession.ID != createdB.CreatedSessionID {
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

func buildFeishuSignedRequest(secret string, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/feishu", strings.NewReader(body))
	timestamp := "1700000000"
	nonce := "nonce-1"
	req.Header.Set("X-Lark-Request-Timestamp", timestamp)
	req.Header.Set("X-Lark-Request-Nonce", nonce)
	req.Header.Set("X-Lark-Signature", signFeishuBody(secret, timestamp, nonce, body))
	return req
}

func signFeishuBody(secret, timestamp, nonce, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp + nonce + body))
	return hex.EncodeToString(mac.Sum(nil))
}

func buildFeishuTextEventBody(eventID, messageID, chatID, chatType, openID, token, text string) string {
	payload := map[string]any{
		"header": map[string]any{
			"event_id":   eventID,
			"event_type": "im.message.receive_v1",
			"token":      token,
		},
		"event": map[string]any{
			"sender": map[string]any{
				"sender_id": map[string]any{
					"open_id": openID,
				},
			},
			"message": map[string]any{
				"message_id":   messageID,
				"message_type": "text",
				"chat_id":      chatID,
				"chat_type":    chatType,
				"content":      fmt.Sprintf(`{"text":%q}`, text),
			},
		},
	}
	raw, _ := json.Marshal(payload)
	return string(raw)
}
