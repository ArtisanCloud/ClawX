package chat

import (
	"errors"
	"fmt"
	"strings"

	"synapsex/internal/domain/conversation"
)

var (
	ErrInvalidNormalizeInput = errors.New("invalid normalize input")
	ErrInvalidWindowContext  = errors.New("invalid window context")
)

type NormalizeInput struct {
	Channel         string
	UserID          string
	GuildID         string
	ThreadID        string
	WindowID        string
	Text            string
	ReplyTo         *string
	Attachments     []Attachment
	IsDirectMessage bool
	IsThread        bool
	IsAllowed       bool
}

type FeishuNormalizeInput struct {
	ChatID   string
	ChatType string
	UserID   string
	Text     string
}

type WeComNormalizeInput struct {
	FromUserID string
	ChatID     string
	Text       string
}

func NormalizeInboundMessage(input NormalizeInput) (Message, error) {
	conversationID, err := conversation.BuildID(conversation.Parts{
		Channel:  input.Channel,
		GuildID:  input.GuildID,
		ThreadID: input.ThreadID,
		UserID:   input.UserID,
	})
	if err != nil {
		return Message{}, fmt.Errorf("%w: %w", ErrInvalidNormalizeInput, err)
	}

	windowID, err := normalizeWindowID(conversationID, input.WindowID)
	if err != nil {
		return Message{}, fmt.Errorf("%w: %w", ErrInvalidNormalizeInput, err)
	}

	return Message{
		ConversationID: conversationID,
		WindowID:       windowID,
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

func NormalizeFeishuTextEvent(input FeishuNormalizeInput) (Message, error) {
	chatID := strings.TrimSpace(input.ChatID)
	chatType := strings.ToLower(strings.TrimSpace(input.ChatType))
	isDirect := chatType == "p2p"

	guildID := ""
	if !isDirect {
		guildID = chatID
	}

	return NormalizeInboundMessage(NormalizeInput{
		Channel:         "feishu",
		UserID:          strings.TrimSpace(input.UserID),
		GuildID:         guildID,
		ThreadID:        "",
		Text:            strings.TrimSpace(input.Text),
		IsDirectMessage: isDirect,
		IsThread:        false,
		IsAllowed:       true,
	})
}

func NormalizeWeComTextEvent(input WeComNormalizeInput) (Message, error) {
	fromUserID := strings.TrimSpace(input.FromUserID)
	chatID := strings.TrimSpace(input.ChatID)
	isDirect := chatID == ""

	guildID := ""
	if !isDirect {
		guildID = chatID
	}

	return NormalizeInboundMessage(NormalizeInput{
		Channel:         "wecom",
		UserID:          fromUserID,
		GuildID:         guildID,
		ThreadID:        "",
		Text:            strings.TrimSpace(input.Text),
		IsDirectMessage: isDirect,
		IsThread:        false,
		IsAllowed:       true,
	})
}

func normalizeWindowID(conversationID, rawWindowID string) (string, error) {
	windowID := strings.TrimSpace(rawWindowID)
	if windowID != "" {
		return windowID, nil
	}
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return "", ErrInvalidWindowContext
	}
	return "compat:" + conversationID, nil
}
