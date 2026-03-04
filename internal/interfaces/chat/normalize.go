package chat

import (
	"strings"

	"synapsex/internal/domain/conversation"
)

type NormalizeInput struct {
	Channel         string
	UserID          string
	GuildID         string
	ThreadID        string
	Text            string
	ReplyTo         *string
	Attachments     []Attachment
	IsDirectMessage bool
	IsThread        bool
	IsAllowed       bool
}

func NormalizeInboundMessage(input NormalizeInput) (Message, error) {
	conversationID, err := conversation.BuildID(conversation.Parts{
		Channel:  input.Channel,
		GuildID:  input.GuildID,
		ThreadID: input.ThreadID,
		UserID:   input.UserID,
	})
	if err != nil {
		return Message{}, err
	}

	return Message{
		ConversationID: conversationID,
		UserID:         strings.TrimSpace(input.UserID),
		Text:           strings.TrimSpace(input.Text),
		ReplyTo:        input.ReplyTo,
		Attachments:    append([]Attachment(nil), input.Attachments...),
		Channel:        strings.TrimSpace(input.Channel),
		ContextFlags: ContextFlags{
			IsDirectMessage: input.IsDirectMessage,
			IsThread:        input.IsThread,
			IsAllowed:       input.IsAllowed,
		},
	}, nil
}

