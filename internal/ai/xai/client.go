package xai

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"hark/internal/ai"
	"hark/internal/ai/providerkit"
)

const (
	ProviderName   = "xai"
	displayName    = "xAI"
	defaultBaseURL = "https://api.x.ai/v1"
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
		Reasoning: func(request ai.Request, _ bool) *providerkit.ResponsesReasoning {
			return reasoningConfigFor(request.ReasoningEffort)
		},
		Tools:      []providerkit.ResponsesTool{{Type: "web_search"}},
		ToolChoice: "auto",
		Include:    []string{"no_inline_citations", "web_search_call.action.sources"},
		SetHeaders: func(httpReq *http.Request, request ai.Request) {
			if request.ConversationID != "" {
				httpReq.Header.Set("x-grok-conv-id", conversationCacheKey(request.ConversationID))
			}
		},
	}
	return client.Ask(ctx, req)
}

func defaultHTTPClient() *http.Client {
	return providerkit.NewHTTPClient(displayName)
}

func reasoningConfigFor(effort string) *providerkit.ResponsesReasoning {
	switch strings.TrimSpace(effort) {
	case "low", "medium", "high", "xhigh":
		return &providerkit.ResponsesReasoning{Effort: effort}
	default:
		return nil
	}
}

func conversationCacheKey(conversationID string) string {
	digest := sha256.Sum256([]byte(conversationID))
	return hex.EncodeToString(digest[:])
}
