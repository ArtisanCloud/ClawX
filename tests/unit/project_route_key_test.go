package unit

import (
	"testing"

	chatiface "clawx/internal/interfaces/chat"
)

func TestNormalizeInboundMessageBuildsProjectRouteKey(t *testing.T) {
	t.Run("direct_message_uses_default_instance", func(t *testing.T) {
		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "telegram",
			UserID:          "user-route-1",
			Text:            "hello",
			IsDirectMessage: true,
			IsAllowed:       true,
		})
		if err != nil {
			t.Fatalf("normalize message: %v", err)
		}
		if message.RouteKey != "telegram:default:direct:user-route-1" {
			t.Fatalf("unexpected route key: %q", message.RouteKey)
		}
	})

	t.Run("thread_dimension_is_included_when_present", func(t *testing.T) {
		message, err := chatiface.NormalizeInboundMessage(chatiface.NormalizeInput{
			Channel:         "discord",
			InstanceID:      "discord-main",
			UserID:          "user-route-2",
			GuildID:         "guild-42",
			ThreadID:        "thread-88",
			Text:            "hello",
			IsDirectMessage: false,
			IsThread:        true,
			IsAllowed:       true,
		})
		if err != nil {
			t.Fatalf("normalize message: %v", err)
		}
		if message.RouteKey != "discord:discord-main:channel:guild-42:thread:thread-88" {
			t.Fatalf("unexpected route key with thread: %q", message.RouteKey)
		}
	})
}
