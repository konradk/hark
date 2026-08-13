package xai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hark/internal/ai"
	"hark/internal/ai/providerkit"
)

func TestAskStreamsTextAndUsesXAIRequestOptions(t *testing.T) {
	var body providerkit.ResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Fatalf("path = %q, want /responses", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q", got)
		}
		if got := r.Header.Get("x-grok-conv-id"); got != conversationCacheKey("conversation-one") {
			t.Fatalf("x-grok-conv-id = %q", got)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
	events, err := client.Ask(context.Background(), ai.Request{
		ConversationID:  "conversation-one",
		Prompt:          "hello",
		Model:           "grok-4.5",
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}

	var streamed string
	var done bool
	for event := range events {
		if event.Type == ai.EventDelta {
			streamed += event.Text
		}
		if event.Type == ai.EventDone {
			done = true
		}
	}
	if streamed != "hello" || !done {
		t.Fatalf("streamed = %q, done = %v", streamed, done)
	}
	if body.Reasoning == nil || body.Reasoning.Effort != "medium" {
		t.Fatalf("reasoning = %#v", body.Reasoning)
	}
	if len(body.Tools) != 1 || body.Tools[0].Type != "web_search" || body.ToolChoice != "auto" {
		t.Fatalf("tools = %#v, tool_choice = %q", body.Tools, body.ToolChoice)
	}
	if len(body.Include) != 2 || body.Include[0] != "no_inline_citations" || body.Include[1] != "web_search_call.action.sources" {
		t.Fatalf("include = %#v", body.Include)
	}
	if body.Store {
		t.Fatal("store = true, want false")
	}
}

func TestAskSendsConversationHistory(t *testing.T) {
	var body providerkit.ResponsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
	events, err := client.Ask(context.Background(), ai.Request{
		Prompt: "continue",
		Model:  "grok-4.5",
		Messages: []ai.Message{
			{Role: "user", Content: "question"},
			{Role: "assistant", Content: "answer"},
		},
	})
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	for range events {
	}
	if len(body.Input) != 3 || body.Input[1].Role != "assistant" || body.Input[1].Content[0].Type != "output_text" {
		t.Fatalf("input = %#v", body.Input)
	}
}

func TestAskRequiresAPIKey(t *testing.T) {
	client := Client{BaseURL: "https://example.invalid"}
	_, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "grok-4.5"})
	if err == nil || !strings.Contains(err.Error(), "xAI API key is not set") {
		t.Fatalf("error = %v", err)
	}
}

func TestReasoningConfigOmitsUnsupportedEffort(t *testing.T) {
	for _, effort := range []string{"", "auto", "none", "minimal", "max"} {
		if config := reasoningConfigFor(effort); config != nil {
			t.Fatalf("reasoningConfigFor(%q) = %#v, want nil", effort, config)
		}
	}
}

func TestReasoningConfigSupportsXHigh(t *testing.T) {
	config := reasoningConfigFor("xhigh")
	if config == nil || config.Effort != "xhigh" {
		t.Fatalf("config = %#v, want xhigh", config)
	}
}

func TestDefaultHTTPClientRejectsRedirects(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com", http.StatusFound)
	}))
	defer redirect.Close()

	client := Client{APIKey: "test-key", BaseURL: redirect.URL, HTTPClient: defaultHTTPClient()}
	_, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "grok-4.5"})
	if err == nil || !strings.Contains(err.Error(), "redirects are not allowed") {
		t.Fatalf("error = %v", err)
	}
}
