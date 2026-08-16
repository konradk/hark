// Package openai_compatible implements a provider client for any
// OpenAI-compatible Chat Completions endpoint. It is used by user-defined
// providers (see config.ProviderSpec) so Hark can talk to self-hosted and
// third-party gateways by setting only a base URL.
package openai_compatible

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"hark/internal/ai"
	"hark/internal/ai/providerkit"
)

// Client streams Chat Completions responses from a configurable base URL.
type Client struct {
	Label          string
	BaseURL        string
	APIKey         string
	APIKeyProvider func() (string, error)
	HTTPClient     *http.Client
}

// New returns a client for the given base URL. The API key is resolved through
// apiKeyProvider, which is consulted on every request.
func New(label, baseURL string, apiKeyProvider func() (string, error)) *Client {
	if strings.TrimSpace(label) == "" {
		label = "OpenAI-compatible"
	}
	return &Client{
		Label:          label,
		BaseURL:        baseURL,
		APIKeyProvider: apiKeyProvider,
		HTTPClient:     defaultHTTPClient(),
	}
}

func (c *Client) Ask(ctx context.Context, req ai.Request) (<-chan ai.Event, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, errors.New("prompt must not be empty")
	}
	if req.Model == "" {
		return nil, errors.New("model must not be empty")
	}
	apiKey, err := c.apiKey()
	if err != nil {
		return nil, err
	}
	if apiKey == "" {
		return nil, fmt.Errorf("%s API key is not set", c.displayName())
	}
	// Resolve into locals: Ask runs concurrently and must not mutate the client.
	baseURL := strings.TrimSpace(c.BaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("%s API base URL is not set", c.displayName())
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}

	messages, err := chatMessagesFor(req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(chatRequest{
		Model:     req.Model,
		Messages:  messages,
		Stream:    true,
		Reasoning: reasoningConfigFor(req.ReasoningEffort),
	})
	if err != nil {
		return nil, fmt.Errorf("encode %s request: %w", c.displayName(), err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", c.displayName(), err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", c.displayName(), err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerkit.APIError(c.displayName(), resp)
	}

	events := make(chan ai.Event)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		readStream(ctx, c.displayName(), resp.Body, events)
	}()

	return events, nil
}

func (c *Client) apiKey() (string, error) {
	if c.APIKeyProvider != nil {
		return c.APIKeyProvider()
	}
	return c.APIKey, nil
}

func (c *Client) displayName() string {
	if strings.TrimSpace(c.Label) != "" {
		return c.Label
	}
	return "OpenAI-compatible"
}

func defaultHTTPClient() *http.Client {
	return providerkit.NewHTTPClient("OpenAI-compatible")
}

type chatRequest struct {
	Model     string           `json:"model"`
	Messages  []chatMessage    `json:"messages"`
	Stream    bool             `json:"stream"`
	Reasoning *reasoningConfig `json:"reasoning,omitempty"`
}

type reasoningConfig struct {
	Effort string `json:"effort"`
}

type chatMessage struct {
	Role    string        `json:"role"`
	Content []contentPart `json:"content"`
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type streamChunk struct {
	Choices []streamChoice `json:"choices"`
	Error   *apiError      `json:"error,omitempty"`
}

type streamChoice struct {
	Delta        chunkDelta `json:"delta"`
	FinishReason *string    `json:"finish_reason"`
}

type chunkDelta struct {
	Content string `json:"content"`
}

type apiError struct {
	Message string `json:"message"`
	Code    any    `json:"code"`
}

func reasoningConfigFor(effort string) *reasoningConfig {
	switch strings.TrimSpace(effort) {
	case "none", "minimal", "low", "medium", "high", "xhigh", "max":
		return &reasoningConfig{Effort: effort}
	default:
		return nil
	}
}

func chatMessagesFor(req ai.Request) ([]chatMessage, error) {
	messages := make([]chatMessage, 0, len(req.Messages)+1)
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		if role != "assistant" && role != "user" {
			role = "user"
		}
		messages = append(messages, chatMessage{
			Role:    role,
			Content: []contentPart{{Type: "text", Text: text}},
		})
	}

	content, err := contentPartsFor(req)
	if err != nil {
		return nil, err
	}
	messages = append(messages, chatMessage{Role: "user", Content: content})
	return messages, nil
}

func contentPartsFor(req ai.Request) ([]contentPart, error) {
	content := []contentPart{{Type: "text", Text: req.Prompt}}
	err := providerkit.EachImageAttachment(req.Attachments, func(dataURL string) error {
		content = append(content, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: dataURL},
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return content, nil
}

func readStream(ctx context.Context, providerName string, reader io.Reader, events chan<- ai.Event) {
	if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventStarted}) {
		return
	}

	limited := &io.LimitedReader{R: reader, N: ai.MaxProviderStreamBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var answer ai.ResponseBuffer

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			if answer.Len() > ai.MaxResponseBytes {
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("provider response exceeds %d bytes", ai.MaxResponseBytes)})
				return
			}
			if answer.Len() > 0 {
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventFinal, Text: answer.String()}) {
					return
				}
			}
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventDone})
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode %s stream chunk: %v", providerName, err)})
			return
		}

		if chunk.Error != nil && chunk.Error.Message != "" {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: chunk.Error.Message})
			return
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if err := answer.Append(choice.Delta.Content); err != nil {
					providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
					return
				}
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventDelta, Text: choice.Delta.Content}) {
					return
				}
			}
			if choice.FinishReason != nil && *choice.FinishReason == "error" {
				message := fmt.Sprintf("%s stream ended with an error", providerName)
				if chunk.Error != nil && chunk.Error.Message != "" {
					message = chunk.Error.Message
				}
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: message})
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() == nil {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("read %s stream: %v", providerName, err)})
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if limited.N <= 0 {
		providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("%s stream exceeds %d bytes", providerName, ai.MaxProviderStreamBytes)})
		return
	}
	providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("%s stream ended before [DONE]", providerName)})
}
