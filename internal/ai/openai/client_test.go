package openai

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
		if r.URL.Path != "/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %q", got)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"hel\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"lo\"}\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
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

func TestAskSendsPriorMessages(t *testing.T) {
	var body responsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
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

	if len(body.Input) != 3 {
		t.Fatalf("len(body.Input) = %d, want 3", len(body.Input))
	}
	if body.Input[0].Role != "user" || body.Input[0].Content[0].Text != "tell me about Wroclaw" {
		t.Fatalf("unexpected first message: %#v", body.Input[0])
	}
	if body.Input[1].Role != "assistant" || body.Input[1].Content[0].Type != "output_text" {
		t.Fatalf("unexpected assistant message: %#v", body.Input[1])
	}
	if body.Input[2].Role != "user" || body.Input[2].Content[0].Text != "what about food?" {
		t.Fatalf("unexpected current message: %#v", body.Input[2])
	}
	if len(body.Tools) != 1 || body.Tools[0].Type != "web_search" {
		t.Fatalf("unexpected tools: %#v", body.Tools)
	}
	if body.ToolChoice != "auto" {
		t.Fatalf("tool choice = %q, want auto", body.ToolChoice)
	}
	if len(body.Include) != 1 || body.Include[0] != "web_search_call.action.sources" {
		t.Fatalf("unexpected include fields: %#v", body.Include)
	}
}

func TestAskSendsReasoningEffort(t *testing.T) {
	var body responsesRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
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

func TestAskReplaysEncryptedReasoningStateWithoutStorage(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		bodies = append(bodies, body)

		w.Header().Set("Content-Type", "text/event-stream")
		if len(bodies) == 1 {
			_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"id":"resp_one","output":[{"id":"rs_one","type":"reasoning","encrypted_content":"opaque-state"},{"id":"msg_one","type":"message","role":"assistant","content":[{"type":"output_text","text":"first answer"}]}]}}` + "\n\n"))
			return
		}
		_, _ = w.Write([]byte(`data: {"type":"response.completed","response":{"id":"resp_two","output":[{"id":"msg_two","type":"message","role":"assistant","content":[{"type":"output_text","text":"second answer"}]}]}}` + "\n\n"))
	}))
	defer server.Close()

	client := Client{APIKey: "test-key", BaseURL: server.URL, HTTPClient: server.Client()}
	firstEvents, err := client.Ask(context.Background(), ai.Request{
		Prompt:          "first question",
		Model:           "gpt-5.6-sol",
		ReasoningEffort: "medium",
	})
	if err != nil {
		t.Fatalf("first Ask returned error: %v", err)
	}
	var state json.RawMessage
	for event := range firstEvents {
		if event.Type == ai.EventDone {
			state = event.ProviderState
		}
	}
	if len(state) == 0 || !strings.Contains(string(state), `"encrypted_content":"opaque-state"`) {
		t.Fatalf("continuation state = %s, want encrypted reasoning item", state)
	}

	secondEvents, err := client.Ask(context.Background(), ai.Request{
		Prompt:          "follow up",
		Model:           "gpt-5.6-sol",
		ReasoningEffort: "medium",
		ProviderState:   state,
	})
	if err != nil {
		t.Fatalf("second Ask returned error: %v", err)
	}
	for range secondEvents {
	}

	if len(bodies) != 2 {
		t.Fatalf("request count = %d, want 2", len(bodies))
	}
	if bodies[0]["store"] != false || bodies[1]["store"] != false {
		t.Fatalf("store fields = %#v, %#v; want false", bodies[0]["store"], bodies[1]["store"])
	}
	firstReasoning := bodies[0]["reasoning"].(map[string]any)
	secondReasoning := bodies[1]["reasoning"].(map[string]any)
	if firstReasoning["context"] != "current_turn" || secondReasoning["context"] != "all_turns" {
		t.Fatalf("reasoning contexts = %#v, %#v", firstReasoning, secondReasoning)
	}
	secondInput := bodies[1]["input"].([]any)
	if len(secondInput) != 4 {
		t.Fatalf("second input has %d items, want prior user, reasoning, assistant, and current user", len(secondInput))
	}
	reasoningItem := secondInput[1].(map[string]any)
	if reasoningItem["type"] != "reasoning" || reasoningItem["encrypted_content"] != "opaque-state" {
		t.Fatalf("replayed reasoning item = %#v", reasoningItem)
	}
}

func TestInputItemsRejectsMalformedContinuationState(t *testing.T) {
	_, err := inputItemsFor(ai.Request{
		Prompt:        "follow up",
		ProviderState: json.RawMessage(`["not-an-object"]`),
	})
	if err == nil || !strings.Contains(err.Error(), "not an object") {
		t.Fatalf("inputItemsFor error = %v, want invalid state error", err)
	}
}

func TestEncodeProviderStateEnforcesLimit(t *testing.T) {
	oversized := json.RawMessage(`{"type":"reasoning","encrypted_content":"` + strings.Repeat("x", ai.MaxProviderStateBytes) + `"}`)
	_, err := encodeProviderState(nil, []json.RawMessage{oversized})
	if err == nil || !strings.Contains(err.Error(), "continuation state exceeds") {
		t.Fatalf("encodeProviderState error = %v, want size error", err)
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

func TestInputContentForImageAttachment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shot.png")
	if err := os.WriteFile(path, testPNGBytes(), 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}

	content, err := inputContentFor(ai.Request{
		Prompt: "describe",
		Attachments: []ai.Attachment{{
			Type:     "image",
			Path:     path,
			MIMEType: "image/png",
		}},
	})
	if err != nil {
		t.Fatalf("inputContentFor returned error: %v", err)
	}

	if len(content) != 2 {
		t.Fatalf("len(content) = %d, want 2", len(content))
	}
	if content[1].Type != "input_image" {
		t.Fatalf("unexpected content type: %q", content[1].Type)
	}
	if !strings.HasPrefix(content[1].ImageURL, "data:image/png;base64,") {
		t.Fatalf("unexpected image url: %q", content[1].ImageURL)
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
	_, err := inputContentFor(ai.Request{Prompt: "describe", Attachments: attachments})
	if err == nil || !strings.Contains(err.Error(), "too many image attachments") {
		t.Fatalf("inputContentFor error = %v, want attachment count error", err)
	}
}

func TestInputContentRejectsOversizedImage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create image: %v", err)
	}
	if err := file.Truncate(providerkit.MaxImageAttachmentBytes + 1); err != nil {
		_ = file.Close()
		t.Fatalf("truncate image: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close image: %v", err)
	}

	_, err = inputContentFor(ai.Request{
		Prompt:      "describe",
		Attachments: []ai.Attachment{{Type: "image", Path: path}},
	})
	if err == nil || !strings.Contains(err.Error(), "exceeds size limit") {
		t.Fatalf("inputContentFor error = %v, want size limit error", err)
	}
}

func TestInputContentRejectsAttachmentsExceedingTotalSizeLimit(t *testing.T) {
	directory := t.TempDir()
	attachments := make([]ai.Attachment, 3)
	for index := range attachments {
		path := filepath.Join(directory, fmt.Sprintf("shot-%d.png", index))
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create image: %v", err)
		}
		if _, err := file.Write(testPNGBytes()); err != nil {
			_ = file.Close()
			t.Fatalf("write image: %v", err)
		}
		if err := file.Truncate(providerkit.MaxImageAttachmentBytes); err != nil {
			_ = file.Close()
			t.Fatalf("truncate image: %v", err)
		}
		if err := file.Close(); err != nil {
			t.Fatalf("close image: %v", err)
		}
		attachments[index] = ai.Attachment{Type: "image", Path: path}
	}

	_, err := inputContentFor(ai.Request{Prompt: "describe", Attachments: attachments})
	if err == nil || !strings.Contains(err.Error(), "exceed total size limit") {
		t.Fatalf("inputContentFor error = %v, want total size limit error", err)
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

	_, err := inputContentFor(ai.Request{
		Prompt:      "describe",
		Attachments: []ai.Attachment{{Type: "image", Path: link}},
	})
	if err == nil || !strings.Contains(err.Error(), "must be a regular file") {
		t.Fatalf("inputContentFor error = %v, want regular file error", err)
	}
}

func TestInputContentValidatesImageContentType(t *testing.T) {
	directory := t.TempDir()
	notImage := filepath.Join(directory, "not-an-image.png")
	if err := os.WriteFile(notImage, []byte("plain text"), 0o600); err != nil {
		t.Fatalf("write text file: %v", err)
	}

	_, err := inputContentFor(ai.Request{
		Prompt:      "describe",
		Attachments: []ai.Attachment{{Type: "image", Path: notImage, MIMEType: "image/png"}},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported content type") {
		t.Fatalf("inputContentFor error = %v, want content type error", err)
	}

	pngPath := filepath.Join(directory, "actual.png")
	if err := os.WriteFile(pngPath, testPNGBytes(), 0o600); err != nil {
		t.Fatalf("write PNG: %v", err)
	}
	_, err = inputContentFor(ai.Request{
		Prompt:      "describe",
		Attachments: []ai.Attachment{{Type: "image", Path: pngPath, MIMEType: "image/jpeg"}},
	})
	if err == nil || !strings.Contains(err.Error(), `content type is "image/png", not "image/jpeg"`) {
		t.Fatalf("inputContentFor error = %v, want MIME mismatch error", err)
	}
}

func TestReadStreamUnblocksWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := make(chan ai.Event)
	returned := make(chan struct{})
	go func() {
		readStream(ctx, strings.NewReader(""), nil, events)
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
	readStream(context.Background(), strings.NewReader("data: {\"type\":\"response.output_text.delta\",\"delta\":\"partial\"}\n\n"), nil, events)
	close(events)

	var errorEvent ai.Event
	var sawDone bool
	for event := range events {
		if event.Type == ai.EventError {
			errorEvent = event
		}
		sawDone = sawDone || event.Type == ai.EventDone
	}
	if !strings.Contains(errorEvent.Error, "before response.completed") {
		t.Fatalf("unexpected error event: %#v", errorEvent)
	}
	if sawDone {
		t.Fatal("did not expect done after a truncated response")
	}
}

func TestAskCancellationClosesHTTPStream(t *testing.T) {
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := client.Ask(ctx, ai.Request{Prompt: "hello", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}
	cancel()
	for range events {
	}

	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("HTTP request context was not canceled")
	}
}

func testPNGBytes() []byte {
	return []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
}

func TestAskStreamsWebSearchStatusAndClickableCitations(t *testing.T) {
	const (
		answer = "The launch happened today [citation]."
		url    = "https://example.com/news"
		title  = "Example News"
	)
	start := strings.Index(answer, "[citation]")
	end := start + len("[citation]")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.web_search_call.searching\",\"item_id\":\"ws_test\"}\n\n"))
		encodedAnswer, _ := json.Marshal(answer)
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":" + string(encodedAnswer) + "}\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.output_text.annotation.added","annotation":{"type":"url_citation","start_index":` +
			fmt.Sprint(start) + `,"end_index":` + fmt.Sprint(end) + `,"url":"` + url + `","title":"` + title + `"}}` + "\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	events, err := client.Ask(context.Background(), ai.Request{Prompt: "latest news", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	var status string
	var final ai.Event
	for event := range events {
		if event.Type == ai.EventStatus {
			status = event.Text
		}
		if event.Type == ai.EventFinal {
			final = event
		}
	}

	if status != "Searching the web..." {
		t.Fatalf("unexpected status: %q", status)
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

func TestAskUsesConsultedSourcesWhenAnnotationsAreMissing(t *testing.T) {
	const url = "https://example.com/source"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"answer\"}\n\n"))
		_, _ = w.Write([]byte(`data: {"type":"response.output_item.done","item":{"type":"web_search_call","action":{"type":"search","sources":[{"type":"url","url":"` + url + `"}]}}}` + "\n\n"))
		_, _ = w.Write([]byte("data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_test\"}}\n\n"))
	}))
	defer server.Close()

	client := Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	events, err := client.Ask(context.Background(), ai.Request{Prompt: "latest", Model: "test-model"})
	if err != nil {
		t.Fatalf("Ask returned error: %v", err)
	}

	var final ai.Event
	for event := range events {
		if event.Type == ai.EventFinal {
			final = event
		}
	}

	if !strings.Contains(final.Text, "Sources: [[1]]("+url+") example.com") {
		t.Fatalf("final text does not contain consulted source: %q", final.Text)
	}
	if len(final.Sources) != 1 || final.Sources[0].URL != url {
		t.Fatalf("unexpected final sources: %#v", final.Sources)
	}
}

func TestFormatCitedAnswerRejectsUnsafeSourceURLs(t *testing.T) {
	final, sources := formatCitedAnswer("answer [citation]", []urlCitation{{
		Type:       "url_citation",
		StartIndex: 7,
		EndIndex:   17,
		Title:      "Unsafe",
		URL:        "javascript:alert(1)",
	}}, nil)

	if final != "answer [citation]" {
		t.Fatalf("unexpected final text: %q", final)
	}
	if len(sources) != 0 {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}

func TestFormatCitedAnswerOmitsUncitedConsultedSources(t *testing.T) {
	final, sources := formatCitedAnswer("answer [citation]", []urlCitation{{
		Type:       "url_citation",
		StartIndex: 7,
		EndIndex:   17,
		Title:      "Cited",
		URL:        "https://example.com/cited",
	}}, []ai.Source{{
		URL: "https://example.com/consulted-only",
	}})

	if strings.Contains(final, "consulted-only") {
		t.Fatalf("final text contains an uncited source: %q", final)
	}
	if len(sources) != 1 || sources[0].URL != "https://example.com/cited" {
		t.Fatalf("unexpected sources: %#v", sources)
	}
}
