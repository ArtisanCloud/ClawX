package chat

import "context"

type Attachment struct {
	Name string
	URL  string
}

type ContextFlags struct {
	IsDirectMessage bool
	IsThread        bool
	IsAllowed       bool
}

type Message struct {
	ConversationID string
	WindowID       string
	UserID         string
	Text           string
	ReplyTo        *string
	Attachments    []Attachment
	Channel        string
	InstanceID     string
	RouteKey       string
	ContextFlags   ContextFlags
}

type Sender interface {
	SendText(ctx context.Context, sessionID, chunk string, isFinal bool) error
	SendError(ctx context.Context, sessionID, message string) error
}
