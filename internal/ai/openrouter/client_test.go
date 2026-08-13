package openrouter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"hark/internal/ai"
	"hark/internal/ai/providerkit"
)

func TestAskStreamsTextDeltas(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"hel"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"lo"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(": OPENROUTER PROCESSING\n\n"))
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	events, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	var text string
	var done bool
	for event := range events {
		if event.Type == ai.EventDelta {
			text += event.Text
		}
		if event.Type == ai.EventDone {
			done = true
		}
	}

	if text != "hello" {
		t.Fatalf("unexpected streamed text: %q", text)
	}
	if !done {
		t.Fatal("expected done event")
	}
}

func TestInitialStatusReportsWebSearch(t *testing.T) {
	client := &Client{}
	if status := client.InitialStatus(ai.Request{}); status != "Searching the web..." {
		t.Fatalf("InitialStatus = %q", status)
	}
}

func TestAskSendsPriorMessagesAndWebPlugin(t *testing.T) {
	var body chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	events, err := client.Ask(context.Background(), ai.Request{
		Prompt: "what about food?",
		Model:  "test-model",
		Messages: []ai.Message{
			{Role: "user", Content: "tell me about Wroclaw"},
			{Role: "assistant", Content: "Wroclaw is a city in Poland."},
		},
	})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	for range events {
	}

	if len(body.Messages) != 3 {
		t.Fatalf("len(body.Messages) = %d, want 3", len(body.Messages))
	}
	if body.Messages[0].Role != "user" || body.Messages[0].Content[0].Text != "tell me about Wroclaw" {
		t.Fatalf("unexpected first message: %#v", body.Messages[0])
	}
	if body.Messages[1].Role != "assistant" || body.Messages[1].Content[0].Text != "Wroclaw is a city in Poland." {
		t.Fatalf("unexpected assistant message: %#v", body.Messages[1])
	}
	if body.Messages[2].Role != "user" || body.Messages[2].Content[0].Text != "what about food?" {
		t.Fatalf("unexpected current message: %#v", body.Messages[2])
	}
	if len(body.Plugins) != 1 || body.Plugins[0].ID != "web" {
		t.Fatalf("unexpected plugins: %#v", body.Plugins)
	}
}

func TestAskSendsReasoningEffort(t *testing.T) {
	var body chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	events, err := client.Ask(context.Background(), ai.Request{
		Prompt:          "hello",
		Model:           "test-model",
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	for range events {
	}

	if body.Reasoning == nil {
		t.Fatal("expected reasoning config")
	}
	if body.Reasoning.Effort != "high" {
		t.Fatalf("unexpected reasoning effort: %q", body.Reasoning.Effort)
	}
}

func TestAskOmitsReasoningEffortForUnsupportedValues(t *testing.T) {
	for _, effort := range []string{"", "auto", "invalid"} {
		var body chatRequest
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}))

		client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
		events, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "test-model", ReasoningEffort: effort})
		if err != nil {
			t.Fatalf("Ask returned error: %v", err)
		}
		for range events {
		}
		server.Close()

		if body.Reasoning != nil {
			t.Fatalf("effort %q: expected no reasoning config, got %#v", effort, body.Reasoning)
		}
	}
}

func TestReasoningConfigPassesSupportedGatewayEfforts(t *testing.T) {
	for _, effort := range []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"} {
		config := reasoningConfigFor(effort)
		if config == nil || config.Effort != effort {
			t.Fatalf("reasoningConfigFor(%q) = %#v", effort, config)
		}
	}
}

func TestAskRequiresAPIKey(t *testing.T) {
	client := Client{}
	if _, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "test-model"}); err == nil {
		t.Fatal("expected missing key error")
	}
}

func TestDefaultHTTPClientRejectsRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("redirect target must not be called")
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusTemporaryRedirect)
	}))
	defer redirect.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    redirect.URL,
		HTTPClient: defaultHTTPClient(),
	}
	_, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "redirects are not allowed") {
		t.Fatalf("Ask error = %v, want redirect rejection", err)
	}
}

func TestAskReturnsMidStreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}` + "\n\n"))
		_, _ = w.Write([]byte(`data: {"id":"gen-1","error":{"message":"Provider disconnected unexpectedly"},"choices":[{"delta":{"content":""},"finish_reason":"error"}]}` + "\n\n"))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
	events, err := client.Ask(context.Background(), ai.Request{Prompt: "hello", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	var errorEvent ai.Event
	var sawDone bool
	for event := range events {
		if event.Type == ai.EventError {
			errorEvent = event
		}
		if event.Type == ai.EventDone {
			sawDone = true
		}
	}
	if errorEvent.Error != "Provider disconnected unexpectedly" {
		t.Fatalf("unexpected error event: %#v", errorEvent)
	}
	if sawDone {
		t.Fatal("did not expect a done event after a mid-stream error")
	}
}

func TestInputContentForImageAttachment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, testPNGBytes(), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	content, err := contentPartsFor(ai.Request{
		Prompt: "describe",
		Attachments: []ai.Attachment{{
			Type:     "image",
			Path:     path,
			MIMEType: "image/png",
		}},
	})
	if err != nil {
		t.Fatalf("contentPartsFor returned error: %v", err)
	}

	if len(content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(content))
	}
	if content[1].Type != "image_url" {
		t.Fatalf("unexpected content type: %q", content[1].Type)
	}
	if content[1].ImageURL == nil || !strings.HasPrefix(content[1].ImageURL.URL, "data:image/png;base64,") {
		t.Fatalf("unexpected image url: %#v", content[1].ImageURL)
	}
}

func TestInputContentRejectsTooManyImageAttachments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, testPNGBytes(), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	attachments := make([]ai.Attachment, providerkit.MaxImageAttachments+1)
	for index := range attachments {
		attachments[index] = ai.Attachment{Type: "image", Path: path, MIMEType: "image/png"}
	}
	_, err := contentPartsFor(ai.Request{Prompt: "describe", Attachments: attachments})
	if err == nil || !strings.Contains(err.Error(), "too many image attachments") {
		t.Fatalf("contentPartsFor error = %v, want attachment count error", err)
	}
}

func TestInputContentRejectsSymlinkAttachment(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target.png")
	if err := os.WriteFile(target, testPNGBytes(), 0o600); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(directory, "link.png")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := contentPartsFor(ai.Request{
		Prompt:      "describe",
		Attachments: []ai.Attachment{{Type: "image", Path: link}},
	})
	if err == nil || !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("contentPartsFor error = %v, want regular file error", err)
	}
}

func testPNGBytes() []byte {
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
}

func TestReadStreamUnblocksWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan ai.Event)
	returned := make(chan struct{})
	go func() {
		readStream(ctx, strings.NewReader(""), events)
		close(returned)
	}()

	cancel()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("readStream remained blocked on an abandoned event channel")
	}
}

func TestReadStreamRejectsTruncatedResponse(t *testing.T) {
	events := make(chan ai.Event, 8)
	readStream(context.Background(), strings.NewReader(`data: {"choices":[{"delta":{"content":"partial"},"finish_reason":null}]}`+"\n\n"), events)
	close(events)

	var errorEvent ai.Event
	var sawDone bool
	for event := range events {
		if event.Type == ai.EventError {
			errorEvent = event
		}
		sawDone = sawDone || event.Type == ai.EventDone
	}
	if !strings.Contains(errorEvent.Error, "before [DONE]") {
		t.Fatalf("unexpected error event: %#v", errorEvent)
	}
	if sawDone {
		t.Fatal("did not expect done after a truncated response")
	}
}

func TestAskStreamsClickableCitations(t *testing.T) {
	const (
		answer = "The launch happened today [citation]."
		url    = "https://example.com/news"
		title  = "Example News"
	)
	start := strings.Index(answer, "[citation]")
	end := start + len("[citation]")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		encodedAnswer, _ := json.Marshal(answer)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":` + string(encodedAnswer) + `},"finish_reason":null}]}` + "\n\n"))
		annotation := fmt.Sprintf(`{"type":"url_citation","url_citation":{"url":%q,"title":%q,"start_index":%d,"end_index":%d}}`, url, title, start, end)
		_, _ = w.Write([]byte(`data: {"choices":[{"delta":{"content":"","annotations":[` + annotation + `]},"finish_reason":"stop"}]}` + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
	events, err := client.Ask(context.Background(), ai.Request{Prompt: "latest news", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	var final ai.Event
	for event := range events {
		if event.Type == ai.EventFinal {
			final = event
		}
	}

	if !strings.Contains(final.Text, "[[1]]("+url+")") {
		t.Fatalf("final text does not contain an inline clickable citation: %q", final.Text)
	}
	if !strings.Contains(final.Text, "Sources: [[1]]("+url+") "+title) {
		t.Fatalf("final text does not contain the source list: %q", final.Text)
	}
	if len(final.Sources) != 1 || final.Sources[0].URL != url || final.Sources[0].Title != title {
		t.Fatalf("unexpected final sources: %#v", final.Sources)
	}
}

func TestFormatCitedAnswerRejectsUnsafeSourceURLs(t *testing.T) {
	final, sources := formatCitedAnswer("answer [citation]", []urlCitationBody{{
		StartIndex: 7,
		EndIndex:   17,
		Title:      "Unsafe",
		URL:        "javascript:alert(1)",
	}})

	if final != "answer [citation]" {
		t.Fatalf("unexpected final text: %q", final)
	}
	if len(sources) != 0 {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}
