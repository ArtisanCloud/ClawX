package unit

import (
	"testing"

	chatiface "synapsex/internal/interfaces/chat"
)

func TestNormalizeFeishuTextEvent(t *testing.T) {
	t.Run("p2p_message_maps_to_direct_conversation", func(t *testing.T) {
		message, err := chatiface.NormalizeFeishuTextEvent(chatiface.FeishuNormalizeInput{
			ChatID:   "oc_direct",
			ChatType: "p2p",
			UserID:   "ou_user_1",
			Text:     "hello",
		})
		if err != nil {
			t.Fatalf("normalize feishu event: %v", err)
		}
		if message.Channel != "feishu" {
			t.Fatalf("unexpected channel: %q", message.Channel)
		}
		if message.ConversationID != "feishu:-:-:ou_user_1" {
			t.Fatalf("unexpected conversation id: %q", message.ConversationID)
		}
		if !message.ContextFlags.IsDirectMessage {
			t.Fatalf("expected direct message")
		}
	})

	t.Run("group_message_uses_chat_id_as_guild", func(t *testing.T) {
		message, err := chatiface.NormalizeFeishuTextEvent(chatiface.FeishuNormalizeInput{
			ChatID:   "oc_group_1",
			ChatType: "group",
			UserID:   "ou_user_2",
			Text:     "/new",
		})
		if err != nil {
			t.Fatalf("normalize feishu event: %v", err)
		}
		if message.ConversationID != "feishu:oc_group_1:-:ou_user_2" {
			t.Fatalf("unexpected conversation id: %q", message.ConversationID)
		}
		if message.ContextFlags.IsDirectMessage {
			t.Fatalf("expected non-direct message")
		}
	})
}
