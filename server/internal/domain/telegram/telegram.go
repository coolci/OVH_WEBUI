// Package telegram provides pure domain entities and invariants for Telegram bot interactions.
// Pure domain: zero external dependencies outside standard library.
package telegram

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrEmptyCommand      = errors.New("command cannot be empty")
	ErrInvalidCommand    = errors.New("command must start with a slash and contain only alphanumeric/underscore characters")
	ErrSessionExpired    = errors.New("session has expired")
	ErrInvalidChatID     = errors.New("chat id cannot be zero")
	ErrEmptyCallbackData = errors.New("callback data cannot be empty")
)

const (
	DefaultSessionTTL = 15 * time.Minute
	MaxMessageLength  = 4096
)

// Session represents an active conversational interaction wizard.
type Session struct {
	ChatID    int64             `json:"chatId"`
	Step      string            `json:"step"`
	Data      map[string]string `json:"data"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

// NewSession creates and initializes a new conversational wizard session.
func NewSession(chatID int64, initialStep string, now time.Time, ttl time.Duration) (*Session, error) {
	if chatID == 0 {
		return nil, ErrInvalidChatID
	}
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	return &Session{
		ChatID:    chatID,
		Step:      initialStep,
		Data:      make(map[string]string),
		ExpiresAt: now.Add(ttl),
	}, nil
}

// IsExpired checks if the session has passed its expiration time.
func (s *Session) IsExpired(now time.Time) bool {
	return now.After(s.ExpiresAt)
}

// SetStep transitions the wizard to a new step with TTL renewal.
func (s *Session) SetStep(step string, now time.Time, ttl time.Duration) error {
	if s.IsExpired(now) {
		return ErrSessionExpired
	}
	s.Step = step
	if ttl <= 0 {
		ttl = DefaultSessionTTL
	}
	s.ExpiresAt = now.Add(ttl)
	return nil
}

// Command represents a registered bot slash command.
type Command struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AuthReq     bool   `json:"authRequired"`
}

// NewCommand validates and constructs a bot slash command.
func NewCommand(name, description string, authReq bool) (*Command, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyCommand
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	// Validate characters after slash
	slug := strings.TrimPrefix(name, "/")
	if slug == "" {
		return nil, ErrEmptyCommand
	}
	for _, r := range slug {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			return nil, ErrInvalidCommand
		}
	}
	return &Command{
		Name:        name,
		Description: strings.TrimSpace(description),
		AuthReq:     authReq,
	}, nil
}

// TruncateMessage ensures text adheres to Telegram's 4096-character limit.
func TruncateMessage(msg string) string {
	if len(msg) <= MaxMessageLength {
		return msg
	}
	return msg[:MaxMessageLength-3] + "..."
}
