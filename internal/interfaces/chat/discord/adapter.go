package discord

import (
	"context"
	"errors"
	"strings"
	"sync"
)

const MaxMessageLength = 2000

var ErrMessageTooLong = errors.New("discord message exceeds limit")

type Adapter struct {
	mu       sync.Mutex
	messages map[string][]string
	errors   map[string][]string
}

func NewAdapter() *Adapter {
	return &Adapter{
		messages: make(map[string][]string),
		errors:   make(map[string][]string),
	}
}

func (a *Adapter) SendText(_ context.Context, sessionID, chunk string, _ bool) error {
	if len([]rune(chunk)) > MaxMessageLength {
		return ErrMessageTooLong
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.messages[sessionID] = append(a.messages[sessionID], chunk)
	return nil
}

func (a *Adapter) SendError(_ context.Context, sessionID, message string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.errors[sessionID] = append(a.errors[sessionID], strings.TrimSpace(message))
	return nil
}

func (a *Adapter) Messages(sessionID string) []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.messages[sessionID]...)
}

