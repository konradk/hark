package providerkit

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
)

type ResponsesClient struct {
	ProviderName   string
	APIKey         string
	APIKeyProvider func() (string, error)
	BaseURL        string
	HTTPClient     *http.Client
	Reasoning      func(ai.Request, bool) *ResponsesReasoning
	Tools          []ResponsesTool
	ToolChoice     string
	Include        []string
	SetHeaders     func(*http.Request, ai.Request)
}

type ResponsesRequest struct {
	Model      string               `json:"model"`
	Reasoning  *ResponsesReasoning  `json:"reasoning,omitempty"`
	Input      []ResponsesInputItem `json:"input"`
	Stream     bool                 `json:"stream"`
	Store      bool                 `json:"store"`
	Tools      []ResponsesTool      `json:"tools,omitempty"`
	ToolChoice string               `json:"tool_choice,omitempty"`
	Include    []string             `json:"include,omitempty"`
}

type ResponsesTool struct {
	Type string `json:"type"`
}

type ResponsesReasoning struct {
	Effort  string `json:"effort,omitempty"`
	Context string `json:"context,omitempty"`
}

type ResponsesInputItem struct {
	Role    string                  `json:"role"`
	Content []ResponsesInputContent `json:"content"`
	Raw     json.RawMessage         `json:"-"`
}

func (item ResponsesInputItem) MarshalJSON() ([]byte, error) {
	if len(item.Raw) > 0 {
		return item.Raw, nil
	}
	type plain ResponsesInputItem
	return json.Marshal(plain(item))
}

type ResponsesInputContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Detail   string `json:"detail,omitempty"`
}

type ResponsesURLCitation struct {
	Type       string `json:"type"`
	StartIndex int    `json:"start_index"`
	EndIndex   int    `json:"end_index"`
	Title      string `json:"title"`
	URL        string `json:"url"`
}

type responsesStreamEvent struct {
	Type       string                `json:"type"`
	Delta      string                `json:"delta,omitempty"`
	Annotation *ResponsesURLCitation `json:"annotation,omitempty"`
	Item       *responsesOutputItem  `json:"item,omitempty"`
	Response   *responsesPayload     `json:"response,omitempty"`
	Error      *responsesAPIError    `json:"error,omitempty"`
}

type responsesPayload struct {
	ID     string            `json:"id"`
	Output []json.RawMessage `json:"output,omitempty"`
}

type responsesOutputItem struct {
	Type    string                    `json:"type"`
	Action  *responsesWebSearchAction `json:"action,omitempty"`
	Content []responsesOutputContent  `json:"content,omitempty"`
}

type responsesOutputContent struct {
	Type        string                 `json:"type"`
	Text        string                 `json:"text,omitempty"`
	Annotations []ResponsesURLCitation `json:"annotations,omitempty"`
}

type responsesWebSearchAction struct {
	Sources []responsesWebSearchSource `json:"sources,omitempty"`
}

type responsesWebSearchSource struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type responsesAPIError struct {
	Message string `json:"message"`
}

func (c *ResponsesClient) Ask(ctx context.Context, req ai.Request) (<-chan ai.Event, error) {
	providerName := strings.TrimSpace(c.ProviderName)
	if providerName == "" {
		providerName = "Provider"
	}
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
		return nil, fmt.Errorf("%s API key is not set", providerName)
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		return nil, fmt.Errorf("%s API base URL is not set", providerName)
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = NewHTTPClient(providerName)
	}

	input, err := ResponsesInputItemsFor(req, providerName)
	if err != nil {
		return nil, err
	}
	var reasoning *ResponsesReasoning
	if c.Reasoning != nil {
		reasoning = c.Reasoning(req, len(req.ProviderState) > 0)
	}
	body, err := json.Marshal(ResponsesRequest{
		Model:      req.Model,
		Reasoning:  reasoning,
		Stream:     true,
		Store:      false,
		Input:      input,
		Tools:      c.Tools,
		ToolChoice: c.ToolChoice,
		Include:    c.Include,
	})
	if err != nil {
		return nil, fmt.Errorf("encode %s request: %w", providerName, err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(c.BaseURL, "/")+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create %s request: %w", providerName, err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.SetHeaders != nil {
		c.SetHeaders(httpReq, req)
	}

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call %s: %w", providerName, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, APIError(providerName, resp)
	}

	events := make(chan ai.Event)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		ReadResponsesStream(ctx, resp.Body, providerName, input, events)
	}()
	return events, nil
}

func (c *ResponsesClient) apiKey() (string, error) {
	if c.APIKeyProvider != nil {
		return c.APIKeyProvider()
	}
	return c.APIKey, nil
}

func ResponsesInputContentFor(req ai.Request) ([]ResponsesInputContent, error) {
	content := []ResponsesInputContent{{Type: "input_text", Text: req.Prompt}}
	err := EachImageAttachment(req.Attachments, func(dataURL string) error {
		content = append(content, ResponsesInputContent{
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

func ResponsesInputItemsFor(req ai.Request, providerName string) ([]ResponsesInputItem, error) {
	if len(req.ProviderState) == 0 {
		return responsesInputMessagesFor(req)
	}

	var history []json.RawMessage
	if err := json.Unmarshal(req.ProviderState, &history); err != nil {
		return nil, fmt.Errorf("decode %s continuation state: %w", providerName, err)
	}
	items := make([]ResponsesInputItem, 0, len(history)+1)
	for index, raw := range history {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || raw[0] != '{' {
			return nil, fmt.Errorf("decode %s continuation state: item %d is not an object", providerName, index)
		}
		items = append(items, ResponsesInputItem{Raw: append(json.RawMessage(nil), raw...)})
	}
	content, err := ResponsesInputContentFor(req)
	if err != nil {
		return nil, err
	}
	items = append(items, ResponsesInputItem{Role: "user", Content: content})
	return items, nil
}

func responsesInputMessagesFor(req ai.Request) ([]ResponsesInputItem, error) {
	messages := make([]ResponsesInputItem, 0, len(req.Messages)+1)
	for _, message := range req.Messages {
		role := strings.TrimSpace(message.Role)
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		if role != "assistant" && role != "user" {
			role = "user"
		}
		contentType := "input_text"
		if role == "assistant" {
			contentType = "output_text"
		}
		messages = append(messages, ResponsesInputItem{
			Role: role,
			Content: []ResponsesInputContent{{
				Type: contentType,
				Text: text,
			}},
		})
	}

	content, err := ResponsesInputContentFor(req)
	if err != nil {
		return nil, err
	}
	messages = append(messages, ResponsesInputItem{Role: "user", Content: content})
	return messages, nil
}

func ReadResponsesStream(ctx context.Context, reader io.Reader, providerName string, input []ResponsesInputItem, events chan<- ai.Event) {
	if !SendEvent(ctx, events, ai.Event{Type: ai.EventStarted}) {
		return
	}

	limited := &io.LimitedReader{R: reader, N: ai.MaxProviderStreamBytes + 1}
	scanner := bufio.NewScanner(limited)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var answer ai.ResponseBuffer
	var citations []ResponsesURLCitation
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

		var event responsesStreamEvent
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode %s stream event: %v", providerName, err)})
			return
		}

		switch event.Type {
		case "response.created":
			if event.Response != nil {
				if !SendEvent(ctx, events, ai.Event{Type: ai.EventStarted, MessageID: event.Response.ID}) {
					return
				}
			}
		case "response.web_search_call.in_progress", "response.web_search_call.searching":
			if !searchStatusSent {
				if !SendEvent(ctx, events, ai.Event{Type: ai.EventStatus, Text: "Searching the web..."}) {
					return
				}
				searchStatusSent = true
			}
		case "response.output_text.delta":
			if event.Delta != "" {
				if err := answer.Append(event.Delta); err != nil {
					SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
					return
				}
				if !SendEvent(ctx, events, ai.Event{Type: ai.EventDelta, Text: event.Delta}) {
					return
				}
			}
		case "response.output_text.annotation.added":
			if event.Annotation != nil && event.Annotation.Type == "url_citation" {
				citations = append(citations, *event.Annotation)
			}
		case "response.output_item.done":
			if event.Item != nil {
				collectResponsesOutputMetadata(*event.Item, &citations, &consultedSources)
				if answer.Len() == 0 {
					if err := answer.Append(responsesOutputText(*event.Item)); err != nil {
						SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
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
					var item responsesOutputItem
					if err := json.Unmarshal(rawItem, &item); err != nil {
						SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("decode %s output item: %v", providerName, err)})
						return
					}
					collectResponsesOutputMetadata(item, &citations, &consultedSources)
					if answer.Len() == 0 {
						if err := answer.Append(responsesOutputText(item)); err != nil {
							SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: err.Error()})
							return
						}
					}
				}
				var stateErr error
				providerState, stateErr = EncodeResponsesProviderState(providerName, input, event.Response.Output)
				if stateErr != nil {
					if !SendEvent(ctx, events, ai.Event{Type: ai.EventWarning, Error: stateErr.Error()}) {
						return
					}
				}
			}
			finalText, sources := FormatResponsesCitedAnswer(answer.String(), citations, consultedSources)
			if len(finalText) > ai.MaxResponseBytes {
				SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("provider response exceeds %d bytes", ai.MaxResponseBytes)})
				return
			}
			if finalText != "" {
				if !SendEvent(ctx, events, ai.Event{Type: ai.EventFinal, Text: finalText, Sources: sources}) {
					return
				}
			}
			SendEvent(ctx, events, ai.Event{Type: ai.EventDone, MessageID: messageID, ProviderState: providerState})
			return
		case "response.failed", "error":
			if event.Error != nil && event.Error.Message != "" {
				SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: event.Error.Message})
			} else {
				SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: event.Type})
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() == nil {
			SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("read %s stream: %v", providerName, err)})
		}
		return
	}
	if ctx.Err() != nil {
		return
	}
	if limited.N <= 0 {
		SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("%s stream exceeds %d bytes", providerName, ai.MaxProviderStreamBytes)})
		return
	}
	SendEvent(ctx, events, ai.Event{Type: ai.EventError, Error: fmt.Sprintf("%s stream ended before response.completed", providerName)})
}

func collectResponsesOutputMetadata(item responsesOutputItem, citations *[]ResponsesURLCitation, sources *[]ai.Source) {
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

func responsesOutputText(item responsesOutputItem) string {
	var text strings.Builder
	for _, content := range item.Content {
		if content.Type == "output_text" {
			text.WriteString(content.Text)
		}
	}
	return text.String()
}

func EncodeResponsesProviderState(providerName string, input []ResponsesInputItem, output []json.RawMessage) (json.RawMessage, error) {
	items := make([]ResponsesInputItem, 0, len(input)+len(output))
	items = append(items, input...)
	for _, raw := range output {
		items = append(items, ResponsesInputItem{Raw: raw})
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		return nil, fmt.Errorf("encode %s continuation state: %w", providerName, err)
	}
	if len(encoded) > ai.MaxProviderStateBytes {
		return nil, fmt.Errorf("%s continuation state exceeds %d bytes; the next turn will use visible conversation history", providerName, ai.MaxProviderStateBytes)
	}
	return encoded, nil
}

func FormatResponsesCitedAnswer(answer string, citations []ResponsesURLCitation, consultedSources []ai.Source) (string, []ai.Source) {
	shared := make([]Citation, 0, len(citations))
	for _, citation := range citations {
		shared = append(shared, Citation{
			Title:      citation.Title,
			URL:        citation.URL,
			StartIndex: citation.StartIndex,
			EndIndex:   citation.EndIndex,
		})
	}
	return FormatCitedAnswer(answer, shared, consultedSources)
}
