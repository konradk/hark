package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"hark/internal/settings"
)

const (
	MaxPromptBytes         = 32 << 10
	MaxConversationBytes   = 128
	MaxMessages            = 100
	MaxMessageBytes        = 32 << 10
	MaxMessageTotalBytes   = 256 << 10
	MaxAttachments         = 4
	MaxAttachmentPath      = 4096
	MaxResponseBytes       = 1 << 20
	MaxProviderStreamBytes = 8 << 20
	MaxProviderStateBytes  = 8 << 20
)

type ResponseBuffer struct {
	text strings.Builder
}

func (b *ResponseBuffer) Append(text string) error {
	if len(text) > MaxResponseBytes-b.text.Len() {
		return fmt.Errorf("provider response exceeds %d bytes", MaxResponseBytes)
	}
	b.text.WriteString(text)
	return nil
}

func (b *ResponseBuffer) Replace(text string) error {
	if len(text) > MaxResponseBytes {
		return fmt.Errorf("provider response exceeds %d bytes", MaxResponseBytes)
	}
	b.text.Reset()
	b.text.WriteString(text)
	return nil
}

func (b *ResponseBuffer) Len() int {
	return b.text.Len()
}

func (b *ResponseBuffer) String() string {
	return b.text.String()
}

type Request struct {
	ConversationID  string          `json:"conversation_id,omitempty"`
	Prompt          string          `json:"prompt"`
	Model           string          `json:"model,omitempty"`
	ReasoningEffort string          `json:"reasoning_effort,omitempty"`
	Messages        []Message       `json:"messages,omitempty"`
	Attachments     []Attachment    `json:"attachments,omitempty"`
	ProviderState   json.RawMessage `json:"-"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Attachment struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	MIMEType string `json:"mime_type"`
}

type Event struct {
	Type          EventType       `json:"type"`
	Text          string          `json:"text,omitempty"`
	MessageID     string          `json:"message_id,omitempty"`
	Error         string          `json:"error,omitempty"`
	Sources       []Source        `json:"sources,omitempty"`
	ProviderState json.RawMessage `json:"-"`
}

type Source struct {
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

type EventType string

const (
	EventStarted EventType = "started"
	EventStatus  EventType = "status"
	EventDelta   EventType = "delta"
	EventFinal   EventType = "final"
	EventDone    EventType = "done"
	EventError   EventType = "error"
	EventWarning EventType = "warning"
)

type Provider interface {
	Ask(ctx context.Context, req Request) (<-chan Event, error)
}

func ValidateRequest(req Request, allowedModels []string) error {
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		return errors.New("prompt must not be empty")
	}
	if !utf8.ValidString(req.Prompt) || len(req.Prompt) > MaxPromptBytes {
		return fmt.Errorf("prompt must be valid UTF-8 and at most %d bytes", MaxPromptBytes)
	}
	if !validIdentifier(req.ConversationID) {
		return fmt.Errorf("conversation_id must contain 1 to %d ASCII letters, digits, dots, underscores, colons, or hyphens", MaxConversationBytes)
	}
	if len(req.Model) == 0 || len(req.Model) > 128 || !contains(allowedModels, req.Model) {
		return fmt.Errorf("model %q is not configured", req.Model)
	}
	if !settings.ValidReasoningEffort(req.ReasoningEffort) {
		return errors.New("reasoning_effort must be one of auto, none, minimal, low, medium, high, xhigh, max")
	}
	if len(req.Messages) > MaxMessages {
		return fmt.Errorf("messages must contain at most %d items", MaxMessages)
	}
	totalMessageBytes := 0
	for index, message := range req.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return fmt.Errorf("messages[%d].role must be user or assistant", index)
		}
		if !utf8.ValidString(message.Content) || strings.TrimSpace(message.Content) == "" || len(message.Content) > MaxMessageBytes {
			return fmt.Errorf("messages[%d].content must be valid, non-empty UTF-8 and at most %d bytes", index, MaxMessageBytes)
		}
		totalMessageBytes += len(message.Content)
	}
	if totalMessageBytes > MaxMessageTotalBytes {
		return fmt.Errorf("messages content must total at most %d bytes", MaxMessageTotalBytes)
	}
	if len(req.Attachments) > MaxAttachments {
		return fmt.Errorf("attachments must contain at most %d items", MaxAttachments)
	}
	for index, attachment := range req.Attachments {
		if attachment.Type != "image" {
			return fmt.Errorf("attachments[%d].type must be image", index)
		}
		if strings.TrimSpace(attachment.Path) == "" || len(attachment.Path) > MaxAttachmentPath {
			return fmt.Errorf("attachments[%d].path must contain 1 to %d bytes", index, MaxAttachmentPath)
		}
		switch attachment.MIMEType {
		case "", "image/png":
		default:
			return fmt.Errorf("attachments[%d].mime_type is not supported", index)
		}
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > MaxConversationBytes {
		return false
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '.', character == '_', character == ':', character == '-':
		default:
			return false
		}
	}
	return true
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
