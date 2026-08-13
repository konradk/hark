package openrouter

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

const (
	ProviderName   = "openrouter"
	displayName    = "OpenRouter"
	defaultBaseURL = "https://openrouter.ai/api/v1"
	appReferer     = "https://github.com/konradk/hark"
	appTitle       = "hark"
)

type Client struct {
	APIKey         string
	APIKeyProvider func() (string, error)
	BaseURL        string
	HTTPClient     *http.Client
}

func NewWithAPIKeyProvider(provider func() (string, error)) *Client {
	return &Client{
		APIKeyProvider: provider,
		BaseURL:        defaultBaseURL,
		HTTPClient:     defaultHTTPClient(),
	}
}

func (*Client) InitialStatus(ai.Request) string {
	return "Searching the web..."
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
		return nil, errors.New("OpenRouter API key is not set")
	}
	// Resolve into locals: Ask runs concurrently and must not mutate the client.
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
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
		Plugins:   []plugin{{ID: "web"}},
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenRouter request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenRouter request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("HTTP-Referer", appReferer)
	httpReq.Header.Set("X-Title", appTitle)

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call OpenRouter: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerkit.APIError(displayName, resp)
	}

	events := make(chan ai.Event)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		readStream(ctx, resp.Body, events)
	}()

	return events, nil
}

func defaultHTTPClient() *http.Client {
	return providerkit.NewHTTPClient(displayName)
}

func (c *Client) apiKey() (string, error) {
	if c.APIKeyProvider != nil {
		return c.APIKeyProvider()
	}
	return c.APIKey, nil
}

type chatRequest struct {
	Model     string           `json:"model"`
	Messages  []chatMessage    `json:"messages"`
	Stream    bool             `json:"stream"`
	Reasoning *reasoningConfig `json:"reasoning,omitempty"`
	Plugins   []plugin         `json:"plugins,omitempty"`
}

type plugin struct {
	ID string `json:"id"`
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
	Content     string       `json:"content"`
	Annotations []annotation `json:"annotations,omitempty"`
}

type annotation struct {
	Type        string          `json:"type"`
	URLCitation urlCitationBody `json:"url_citation"`
}

type urlCitationBody struct {
	URL        string `json:"url"`
	Title      string `json:"title"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
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

func readStream(ctx context.Context, reader io.Reader, events chan<- ai.Event) {
	if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventStarted}) {
		return
	}

	limited := &io.LimitedReader{R: reader, N: ai.MaxProviderStreamBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var answer ai.ResponseBuffer
	var citations []urlCitationBody

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
			finalText, sources := formatCitedAnswer(answer.String(), citations)
			if len(finalText) > ai.MaxResponseBytes {
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("provider response exceeds %d bytes", ai.MaxResponseBytes)})
				return
			}
			if finalText != "" {
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventFinal, Text: finalText, Sources: sources}) {
					return
				}
			}
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventDone})
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode OpenRouter stream chunk: %v", err)})
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
			for _, item := range choice.Delta.Annotations {
				if item.Type == "url_citation" {
					citations = append(citations, item.URLCitation)
				}
			}
			if choice.FinishReason != nil && *choice.FinishReason == "error" {
				message := "OpenRouter stream ended with an error"
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
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("read OpenRouter stream: %v", err)})
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if limited.N <= 0 {
		providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("OpenRouter stream exceeds %d bytes", ai.MaxProviderStreamBytes)})
		return
	}
	providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: "OpenRouter stream ended before [DONE]"})
}

func formatCitedAnswer(answer string, citations []urlCitationBody) (string, []ai.Source) {
	shared := make([]providerkit.Citation, 0, len(citations))
	for _, citation := range citations {
		shared = append(shared, providerkit.Citation{
			Title:      citation.Title,
			URL:        citation.URL,
			StartIndex: citation.StartIndex,
			EndIndex:   citation.EndIndex,
		})
	}
	return providerkit.FormatCitedAnswer(answer, shared, nil)
}
