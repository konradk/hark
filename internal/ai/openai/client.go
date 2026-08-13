package openai

import (
	"context"
	"encoding/json"
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
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = defaultHTTPClient()
	}
	client := providerkit.ResponsesClient{
		ProviderName:   displayName,
		APIKey:         c.APIKey,
		APIKeyProvider: c.APIKeyProvider,
		BaseURL:        baseURL,
		HTTPClient:     httpClient,
		Reasoning: func(request ai.Request, hasState bool) *providerkit.ResponsesReasoning {
			return reasoningConfigFor(request.Model, request.ReasoningEffort, hasState)
		},
		Tools:      []providerkit.ResponsesTool{{Type: "web_search"}},
		ToolChoice: "auto",
		Include:    []string{"web_search_call.action.sources"},
	}
	return client.Ask(ctx, req)
}

func defaultHTTPClient() *http.Client {
	return providerkit.NewHTTPClient(displayName)
}

type responsesRequest = providerkit.ResponsesRequest
type reasoningConfig = providerkit.ResponsesReasoning
type responseInputItem = providerkit.ResponsesInputItem
type inputContent = providerkit.ResponsesInputContent
type urlCitation = providerkit.ResponsesURLCitation

func inputContentFor(req ai.Request) ([]inputContent, error) {
	return providerkit.ResponsesInputContentFor(req)
}

func inputItemsFor(req ai.Request) ([]responseInputItem, error) {
	return providerkit.ResponsesInputItemsFor(req, displayName)
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
	providerkit.ReadResponsesStream(ctx, reader, displayName, input, events)
}

func encodeProviderState(input []responseInputItem, output []json.RawMessage) (json.RawMessage, error) {
	return providerkit.EncodeResponsesProviderState(displayName, input, output)
}

func formatCitedAnswer(answer string, citations []urlCitation, consultedSources []ai.Source) (string, []ai.Source) {
	return providerkit.FormatResponsesCitedAnswer(answer, citations, consultedSources)
}
