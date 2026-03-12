package unit

import (
	"testing"

	chatiface "clawx/internal/interfaces/chat"
)

func TestNormalizeWeComTextEvent(t *testing.T) {
	t.Run("direct_message_maps_to_user_conversation", func(t *testing.T) {
		message, err := chatiface.NormalizeWeComTextEvent(chatiface.WeComNormalizeInput{
			FromUserID: "zhangsan",
			Text:       "hello",
		})
		if err != nil {
			t.Fatalf("normalize wecom direct message: %v", err)
		}

		if message.ConversationID != "wecom:-:-:zhangsan" {
			t.Fatalf("unexpected conversation id: %q", message.ConversationID)
		}
		if message.WindowID != "compat:wecom:-:-:zhangsan" {
			t.Fatalf("unexpected window id: %q", message.WindowID)
		}
	})

	t.Run("group_message_uses_chat_id_as_guild", func(t *testing.T) {
		message, err := chatiface.NormalizeWeComTextEvent(chatiface.WeComNormalizeInput{
			FromUserID: "lisi",
			ChatID:     "chat-001",
			Text:       "hello group",
		})
		if err != nil {
			t.Fatalf("normalize wecom group message: %v", err)
		}

		if message.ConversationID != "wecom:chat-001:-:lisi" {
			t.Fatalf("unexpected conversation id: %q", message.ConversationID)
		}
	})
}
