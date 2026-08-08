package openai

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
	ProviderName   = "openai"
	displayName    = "OpenAI"
	defaultBaseURL = "https://api.openai.com/v1"
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
		return nil, errors.New("OpenAI API key is not set")
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

	input, err := inputItemsFor(req)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(responsesRequest{
		Model:      req.Model,
		Reasoning:  reasoningConfigFor(req.Model, req.ReasoningEffort, len(req.ProviderState) > 0),
		Stream:     true,
		Store:      false,
		Input:      input,
		Tools:      []responseTool{{Type: "web_search"}},
		ToolChoice: "auto",
		Include:    []string{"web_search_call.action.sources"},
	})
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create OpenAI request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call OpenAI: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, providerkit.APIError(displayName, resp)
	}

	events := make(chan ai.Event)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		readStream(ctx, resp.Body, input, events)
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

type responsesRequest struct {
	Model      string              `json:"model"`
	Reasoning  *reasoningConfig    `json:"reasoning,omitempty"`
	Input      []responseInputItem `json:"input"`
	Stream     bool                `json:"stream"`
	Store      bool                `json:"store"`
	Tools      []responseTool      `json:"tools,omitempty"`
	ToolChoice string              `json:"tool_choice,omitempty"`
	Include    []string            `json:"include,omitempty"`
}

type responseTool struct {
	Type string `json:"type"`
}

type reasoningConfig struct {
	Effort  string `json:"effort,omitempty"`
	Context string `json:"context,omitempty"`
}

type responseInputItem struct {
	Role    string          `json:"role"`
	Content []inputContent  `json:"content"`
	Raw     json.RawMessage `json:"-"`
}

func (item responseInputItem) MarshalJSON() ([]byte, error) {
	if len(item.Raw) > 0 {
		return item.Raw, nil
	}
	type plain responseInputItem
	return json.Marshal(plain(item))
}

type inputContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type streamEvent struct {
	Type       string              `json:"type"`
	Delta      string              `json:"delta,omitempty"`
	Text       string              `json:"text,omitempty"`
	Annotation *urlCitation        `json:"annotation,omitempty"`
	Item       *responseOutputItem `json:"item,omitempty"`
	Response   *responsePayload    `json:"response,omitempty"`
	Error      *apiError           `json:"error,omitempty"`
}

type responsePayload struct {
	ID     string            `json:"id"`
	Output []json.RawMessage `json:"output,omitempty"`
}

type responseOutputItem struct {
	Type    string                  `json:"type"`
	Action  *webSearchAction        `json:"action,omitempty"`
	Content []responseOutputContent `json:"content,omitempty"`
}

type responseOutputContent struct {
	Type        string        `json:"type"`
	Text        string        `json:"text,omitempty"`
	Annotations []urlCitation `json:"annotations,omitempty"`
}

type webSearchAction struct {
	Sources []webSearchSource `json:"sources,omitempty"`
}

type webSearchSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type urlCitation struct {
	Type       string `json:"type"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

type apiError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    any    `json:"code"`
}

func inputContentFor(req ai.Request) ([]inputContent, error) {
	content := []inputContent{{Type: "input_text", Text: req.Prompt}}
	err := providerkit.EachImageAttachment(req.Attachments, func(dataURL string) error {
		content = append(content, inputContent{
			Type:     "input_image",
			ImageURL: dataURL,
			Detail:   "auto",
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return content, nil
}

func inputMessagesFor(req ai.Request) ([]responseInputItem, error) {
	messages := make([]responseInputItem, 0, len(req.Messages)+1)
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		if role != "assistant" && role != "user" {
			role = "user"
		}
		messages = append(messages, responseInputItem{
			Role: role,
			Content: []inputContent{{
				Type: inputTextTypeForRole(role),
				Text: text,
			}},
		})
	}

	content, err := inputContentFor(req)
	if err != nil {
		return nil, err
	}
	messages = append(messages, responseInputItem{
		Role:    "user",
		Content: content,
	})
	return messages, nil
}

func inputItemsFor(req ai.Request) ([]responseInputItem, error) {
	if len(req.ProviderState) == 0 {
		return inputMessagesFor(req)
	}

	var history []json.RawMessage
	if err := json.Unmarshal(req.ProviderState, &history); err != nil {
		return nil, fmt.Errorf("decode OpenAI continuation state: %w", err)
	}
	items := make([]responseInputItem, 0, len(history)+1)
	for index, raw := range history {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '{' {
			return nil, fmt.Errorf("decode OpenAI continuation state: item %d is not an object", index)
		}
		items = append(items, responseInputItem{Raw: append(json.RawMessage(nil), raw...)})
	}
	content, err := inputContentFor(req)
	if err != nil {
		return nil, err
	}
	items = append(items, responseInputItem{Role: "user", Content: content})
	return items, nil
}

func inputTextTypeForRole(role string) string {
	if role == "assistant" {
		return "output_text"
	}
	return "input_text"
}

func reasoningConfigFor(model, effort string, hasState bool) *reasoningConfig {
	effort = strings.TrimSpace(effort)
	if (effort == "" || effort == "auto") && !strings.HasPrefix(model, "gpt-5.6") {
		return nil
	}
	config := &reasoningConfig{}
	if effort != "auto" {
		config.Effort = effort
	}
	if strings.HasPrefix(model, "gpt-5.6") {
		if hasState {
			config.Context = "all_turns"
		} else {
			config.Context = "current_turn"
		}
	}
	return config
}

func readStream(ctx context.Context, reader io.Reader, input []responseInputItem, events chan<- ai.Event) {
	if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventStarted}) {
		return
	}

	limited := &io.LimitedReader{R: reader, N: ai.MaxProviderStreamBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var answer ai.ResponseBuffer
	var citations []urlCitation
	var consultedSources []ai.Source
	searchStatusSent := false

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var event streamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode OpenAI stream event: %v", err)})
			return
		}

		switch event.Type {
		case "response.created":
			if event.Response != nil {
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventStarted, MessageID: event.Response.ID}) {
					return
				}
			}
		case "response.web_search_call.in_progress", "response.web_search_call.searching":
			if !searchStatusSent {
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventStatus, Text: "Searching the web..."}) {
					return
				}
				searchStatusSent = true
			}
		case "response.output_text.delta":
			if event.Delta != "" {
				if err := answer.Append(event.Delta); err != nil {
					providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
					return
				}
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventDelta, Text: event.Delta}) {
					return
				}
			}
		case "response.output_text.annotation.added":
			if event.Annotation != nil && event.Annotation.Type == "url_citation" {
				citations = append(citations, *event.Annotation)
			}
		case "response.output_item.done":
			if event.Item != nil {
				collectOutputMetadata(*event.Item, &citations, &consultedSources)
				if answer.Len() == 0 {
					if err := answer.Append(outputText(*event.Item)); err != nil {
						providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
						return
					}
				}
			}
		case "response.completed":
			messageID := ""
			var providerState json.RawMessage
			if event.Response != nil {
				messageID = event.Response.ID
				for _, rawItem := range event.Response.Output {
					var item responseOutputItem
					if err := json.Unmarshal(rawItem, &item); err != nil {
						providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode OpenAI output item: %v", err)})
						return
					}
					collectOutputMetadata(item, &citations, &consultedSources)
					if answer.Len() == 0 {
						if err := answer.Append(outputText(item)); err != nil {
							providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
							return
						}
					}
				}
				var stateErr error
				providerState, stateErr = encodeProviderState(input, event.Response.Output)
				if stateErr != nil {
					if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventWarning, Error: stateErr.Error()}) {
						return
					}
				}
			}
			finalText, sources := formatCitedAnswer(answer.String(), citations, consultedSources)
			if len(finalText) > ai.MaxResponseBytes {
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("provider response exceeds %d bytes", ai.MaxResponseBytes)})
				return
			}
			if finalText != "" {
				if !providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventFinal, Text: finalText, Sources: sources}) {
					return
				}
			}
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventDone, MessageID: messageID, ProviderState: providerState})
			return
		case "response.failed", "error":
			if event.Error != nil && event.Error.Message != "" {
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: event.Error.Message})
			} else {
				providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: event.Type})
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() == nil {
			providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("read OpenAI stream: %v", err)})
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if limited.N <= 0 {
		providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("OpenAI stream exceeds %d bytes", ai.MaxProviderStreamBytes)})
		return
	}
	providerkit.SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: "OpenAI stream ended before response.completed"})
}

func collectOutputMetadata(item responseOutputItem, citations *[]urlCitation, sources *[]ai.Source) {
	for _, content := range item.Content {
		for _, citation := range content.Annotations {
			if citation.Type == "url_citation" {
				*citations = append(*citations, citation)
			}
		}
	}

	if item.Type != "web_search_call" || item.Action == nil {
		return
	}
	for _, source := range item.Action.Sources {
		if source.Type == "" || source.Type == "url" {
			*sources = append(*sources, ai.Source{URL: source.URL})
		}
	}
}

func outputText(item responseOutputItem) string {
	var text strings.Builder
	for _, content := range item.Content {
		if content.Type == "output_text" {
			text.WriteString(content.Text)
		}
	}
	return text.String()
}

func encodeProviderState(input []responseInputItem, output []json.RawMessage) (json.RawMessage, error) {
	items := make([]responseInputItem, 0, len(input)+len(output))
	items = append(items, input...)
	for _, raw := range output {
		items = append(items, responseInputItem{Raw: raw})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode OpenAI continuation state: %w", err)
	}
	if len(encoded) > ai.MaxProviderStateBytes {
		return nil, fmt.Errorf("OpenAI continuation state exceeds %d bytes; the next turn will use visible conversation history", ai.MaxProviderStateBytes)
	}
	return encoded, nil
}

func formatCitedAnswer(answer string, citations []urlCitation, consultedSources []ai.Source) (string, []ai.Source) {
	shared := make([]providerkit.Citation, 0, len(citations))
	for _, citation := range citations {
		shared = append(shared, providerkit.Citation{
			Title:      citation.Title,
			URL:        citation.URL,
			StartIndex: citation.StartIndex,
			EndIndex:   citation.EndIndex,
		})
	}
	return providerkit.FormatCitedAnswer(answer, shared, consultedSources)
}
