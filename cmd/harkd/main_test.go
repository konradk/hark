package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"hark/internal/ai"
	"hark/internal/config"
	"hark/internal/history"
	"hark/internal/ipc"
	"hark/internal/settings"
)

type fakeProvider struct {
	events []ai.Event
	err    error
}

type providerFunc func(context.Context, ai.Request) (<-chan ai.Event, error)

func (provider providerFunc) Ask(ctx context.Context, request ai.Request) (<-chan ai.Event, error) {
	return provider(ctx, request)
}

func (provider fakeProvider) Ask(context.Context, ai.Request) (<-chan ai.Event, error) {
	if provider.err != nil {
		return nil, provider.err
	}
	events := make(chan ai.Event, len(provider.events))
	for _, event := range provider.events {
		events <- event
	}
	close(events)
	return events, nil
}

type fakeHistory struct {
	mu         sync.Mutex
	added      []history.Entry
	addErr     error
	settings   map[settings.Key]string
	settingErr error
	cleanup    history.CleanupResult
	references []string
}

func (store *fakeHistory) Add(_ context.Context, entry history.Entry) (int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.addErr != nil {
		return 0, store.addErr
	}
	store.added = append(store.added, entry)
	return int64(len(store.added)), nil
}

func (*fakeHistory) List(context.Context, int) ([]history.Entry, error) {
	return []history.Entry{}, nil
}

func (*fakeHistory) Get(context.Context, int64) (history.Entry, error) {
	return history.Entry{}, errors.New("not implemented")
}

func (store *fakeHistory) DeleteConversation(context.Context, int64) (history.CleanupResult, error) {
	return store.cleanup, nil
}

func (store *fakeHistory) Clear(context.Context) (history.CleanupResult, error) {
	return store.cleanup, nil
}

func (store *fakeHistory) PruneBefore(context.Context, time.Time) (history.CleanupResult, error) {
	return store.cleanup, nil
}

func (store *fakeHistory) ReferencedAttachmentPaths(context.Context) ([]string, error) {
	return store.references, nil
}

func (store *fakeHistory) GetSetting(_ context.Context, key settings.Key) (string, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.settingErr != nil {
		return "", false, store.settingErr
	}
	value, ok := store.settings[key]
	return value, ok, nil
}

func (store *fakeHistory) SetSetting(_ context.Context, key settings.Key, value string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.settingErr != nil {
		return store.settingErr
	}
	if store.settings == nil {
		store.settings = make(map[settings.Key]string)
	}
	store.settings[key] = value
	return nil
}

type fakeCleaner struct {
	removed []string
	err     error
}

func (cleaner *fakeCleaner) RemoveManaged(paths []string) (int, error) {
	cleaner.removed = append(cleaner.removed, paths...)
	return len(paths), cleaner.err
}

func (cleaner *fakeCleaner) RemoveAll() (int, error) {
	return 0, cleaner.err
}

func (*fakeCleaner) RemoveUnreferenced([]string, time.Time) (int, error) {
	return 0, nil
}

func newTestApp(store *fakeHistory) *appState {
	cfg := config.Defaults()
	cfg.Provider.DefaultModel = "gpt-test"
	cfg.Provider.Models = []config.ModelConfig{{ID: "gpt-test", Label: "Test"}}
	return &appState{
		cfg: cfg,
		providers: map[string]ai.Provider{
			"openai": fakeProvider{events: []ai.Event{
				{Type: ai.EventFinal, Text: "answer"},
				{Type: ai.EventDone},
			}},
		},
		history: store,
		cleaner: &fakeCleaner{},
		states:  make(map[string]runtimeState),
	}
}

func TestAskReportsHistorySaveFailureAndStillCompletes(t *testing.T) {
	store := &fakeHistory{
		addErr:   errors.New("database is read-only"),
		settings: make(map[settings.Key]string),
	}
	app := newTestApp(store)
	request := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-one",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "low",
	})

	var events []ai.Event
	err := app.ask(context.Background(), request, func(value any) error {
		event, ok := value.(ai.Event)
		if !ok {
			return fmt.Errorf("unexpected event type %T", value)
		}
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatalf("ask returned error: %v", err)
	}

	var warning, done bool
	for _, event := range events {
		warning = warning || event.Type == ai.EventWarning
		done = done || event.Type == ai.EventDone
	}
	if !warning || !done {
		t.Fatalf("events = %#v, want warning and done", events)
	}
	state, ok := app.runtimeState("chat-one")
	if !ok || state.LatestAnswer != "answer" {
		t.Fatalf("runtime state = %#v, %v", state, ok)
	}
}

func TestAskRejectsAttachmentsOutsideTheScreenshotDirectory(t *testing.T) {
	screenshotDir := t.TempDir()
	app := newTestApp(&fakeHistory{settings: make(map[settings.Key]string)})
	app.attachmentDir = screenshotDir

	for name, path := range map[string]string{
		"unrelated file":      "/etc/passwd",
		"traversal":           filepath.Join(screenshotDir, "..", "region-1.png"),
		"nested directory":    filepath.Join(screenshotDir, "nested", "region-1.png"),
		"unmanaged file name": filepath.Join(screenshotDir, "secret.png"),
	} {
		request := requestWithParams(t, "ask", ai.Request{
			ConversationID:  "chat-attach",
			Prompt:          "describe",
			Model:           "gpt-test",
			ReasoningEffort: "low",
			Attachments:     []ai.Attachment{{Type: "image", Path: path}},
		})
		err := app.ask(context.Background(), request, func(any) error { return nil })
		if err == nil {
			t.Fatalf("%s: ask accepted attachment %q", name, path)
		}
	}

	accepted := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-attach",
		Prompt:          "describe",
		Model:           "gpt-test",
		ReasoningEffort: "low",
		Attachments:     []ai.Attachment{{Type: "image", Path: filepath.Join(screenshotDir, "region-1.png")}},
	})
	if err := app.ask(context.Background(), accepted, func(any) error { return nil }); err != nil {
		t.Fatalf("ask rejected a managed screenshot: %v", err)
	}
}

func TestAskDoesNotPersistWhenSaveHistoryIsDisabled(t *testing.T) {
	store := &fakeHistory{settings: map[settings.Key]string{settings.SaveHistory: "false"}}
	app := newTestApp(store)
	request := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-private",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "low",
	})
	if err := app.ask(context.Background(), request, func(any) error { return nil }); err != nil {
		t.Fatalf("ask returned error: %v", err)
	}
	if len(store.added) != 0 {
		t.Fatalf("history entries = %#v, want none", store.added)
	}
}

func TestAskKeepsProviderStateInMemoryForMatchingConversationAndModel(t *testing.T) {
	store := &fakeHistory{settings: map[settings.Key]string{settings.SaveHistory: "false"}}
	app := newTestApp(store)
	var received []json.RawMessage
	app.providers["openai"] = providerFunc(func(_ context.Context, request ai.Request) (<-chan ai.Event, error) {
		received = append(received, append(json.RawMessage(nil), request.ProviderState...))
		events := make(chan ai.Event, 2)
		events <- ai.Event{Type: ai.EventFinal, Text: "answer"}
		events <- ai.Event{Type: ai.EventDone, ProviderState: json.RawMessage(`[{"type":"reasoning","encrypted_content":"opaque"}]`)}
		close(events)
		return events, nil
	})

	request := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-state",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "low",
	})
	if err := app.ask(context.Background(), request, func(any) error { return nil }); err != nil {
		t.Fatalf("first ask returned error: %v", err)
	}
	if err := app.ask(context.Background(), request, func(any) error { return nil }); err != nil {
		t.Fatalf("second ask returned error: %v", err)
	}
	if len(received) != 2 || len(received[0]) != 0 || !strings.Contains(string(received[1]), "opaque") {
		t.Fatalf("received provider states = %q", received)
	}
	state, ok := app.runtimeState("chat-state")
	if !ok || state.Provider != "openai" || state.Model != "gpt-test" || !strings.Contains(string(state.ProviderState), "opaque") {
		t.Fatalf("runtime state = %#v, %v", state, ok)
	}
}

func TestAskRoutesByModelProviderAndRecordsItInHistory(t *testing.T) {
	store := &fakeHistory{settings: map[settings.Key]string{settings.SaveHistory: "true"}}
	cfg := config.Defaults()
	cfg.Provider.DefaultModel = "gpt-test"
	cfg.Provider.Models = []config.ModelConfig{
		{ID: "gpt-test", Label: "Test", Provider: "openai"},
		{ID: "or-test", Label: "OpenRouter Test", Provider: "openrouter"},
	}
	app := &appState{
		cfg: cfg,
		providers: map[string]ai.Provider{
			"openai": fakeProvider{events: []ai.Event{
				{Type: ai.EventFinal, Text: "openai-answer"},
				{Type: ai.EventDone},
			}},
			"openrouter": fakeProvider{events: []ai.Event{
				{Type: ai.EventFinal, Text: "openrouter-answer"},
				{Type: ai.EventDone},
			}},
		},
		history: store,
		cleaner: &fakeCleaner{},
		states:  make(map[string]runtimeState),
	}

	openaiRequest := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-openai",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "low",
	})
	if err := app.ask(context.Background(), openaiRequest, func(any) error { return nil }); err != nil {
		t.Fatalf("ask returned error: %v", err)
	}

	openRouterRequest := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-openrouter",
		Prompt:          "question",
		Model:           "or-test",
		ReasoningEffort: "low",
	})
	if err := app.ask(context.Background(), openRouterRequest, func(any) error { return nil }); err != nil {
		t.Fatalf("ask returned error: %v", err)
	}

	if len(store.added) != 2 {
		t.Fatalf("history entries = %#v, want 2", store.added)
	}
	if store.added[0].Provider != "openai" || store.added[0].Response != "openai-answer" {
		t.Fatalf("unexpected first entry: %#v", store.added[0])
	}
	if store.added[1].Provider != "openrouter" || store.added[1].Response != "openrouter-answer" {
		t.Fatalf("unexpected second entry: %#v", store.added[1])
	}
}

func TestAskFailsForModelWithoutConfiguredProvider(t *testing.T) {
	store := &fakeHistory{settings: make(map[settings.Key]string)}
	cfg := config.Defaults()
	cfg.Provider.DefaultModel = "gpt-test"
	cfg.Provider.Models = []config.ModelConfig{{ID: "gpt-test", Label: "Test", Provider: "openai"}}
	app := &appState{
		cfg:       cfg,
		providers: map[string]ai.Provider{},
		history:   store,
		cleaner:   &fakeCleaner{},
		states:    make(map[string]runtimeState),
	}

	request := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-missing",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "low",
	})
	if err := app.ask(context.Background(), request, func(any) error { return nil }); err == nil {
		t.Fatal("expected error for model without a configured provider")
	}
}

func TestAskRejectsReasoningUnsupportedByModel(t *testing.T) {
	app := newTestApp(&fakeHistory{settings: make(map[settings.Key]string)})
	request := requestWithParams(t, "ask", ai.Request{
		ConversationID:  "chat-reasoning",
		Prompt:          "question",
		Model:           "gpt-test",
		ReasoningEffort: "xhigh",
	})
	if err := app.ask(context.Background(), request, func(any) error { return nil }); err == nil {
		t.Fatal("ask accepted reasoning effort unsupported by the model")
	}
}

func TestRuntimeStateIsIsolatedAndBounded(t *testing.T) {
	app := &appState{states: make(map[string]runtimeState)}
	app.setLatestAnswer("conversation-a", "answer-a")
	app.setLatestAnswer("conversation-b", "answer-b")
	if state, _ := app.runtimeState("conversation-a"); state.LatestAnswer != "answer-a" {
		t.Fatalf("conversation-a state = %#v", state)
	}
	if state, _ := app.runtimeState("conversation-b"); state.LatestAnswer != "answer-b" {
		t.Fatalf("conversation-b state = %#v", state)
	}

	for index := 0; index < maxRuntimeStates+20; index++ {
		app.setLatestAnswer(fmt.Sprintf("state-%d", index), "answer")
	}
	if len(app.states) > maxRuntimeStates {
		t.Fatalf("runtime state count = %d, want at most %d", len(app.states), maxRuntimeStates)
	}
}

func TestRuntimeProviderStateHasGlobalMemoryBudget(t *testing.T) {
	app := &appState{states: make(map[string]runtimeState)}
	state := make(json.RawMessage, ai.MaxProviderStateBytes)
	for index := 0; index < maxProviderStateBytesTotal/ai.MaxProviderStateBytes+2; index++ {
		app.setProviderState(fmt.Sprintf("conversation-%d", index), "openai", "gpt-test", state)
	}

	total := 0
	for _, runtime := range app.states {
		total += len(runtime.ProviderState)
	}
	if total > maxProviderStateBytesTotal {
		t.Fatalf("provider state bytes = %d, want at most %d", total, maxProviderStateBytesTotal)
	}
}

func TestConcurrentAsksKeepAnswersInTheirConversations(t *testing.T) {
	store := &fakeHistory{settings: make(map[settings.Key]string)}
	app := newTestApp(store)
	app.providers["openai"] = providerFunc(func(_ context.Context, request ai.Request) (<-chan ai.Event, error) {
		events := make(chan ai.Event, 2)
		events <- ai.Event{Type: ai.EventFinal, Text: "answer-for-" + request.ConversationID}
		events <- ai.Event{Type: ai.EventDone}
		close(events)
		return events, nil
	})

	var wait sync.WaitGroup
	errorsByRequest := make(chan error, 2)
	for _, conversationID := range []string{"chat-a", "chat-b"} {
		conversationID := conversationID
		wait.Add(1)
		go func() {
			defer wait.Done()
			request := requestWithParams(t, "ask", ai.Request{
				ConversationID:  conversationID,
				Prompt:          "question",
				Model:           "gpt-test",
				ReasoningEffort: "low",
			})
			errorsByRequest <- app.ask(context.Background(), request, func(any) error { return nil })
		}()
	}
	wait.Wait()
	close(errorsByRequest)
	for err := range errorsByRequest {
		if err != nil {
			t.Fatalf("ask returned error: %v", err)
		}
	}
	for _, conversationID := range []string{"chat-a", "chat-b"} {
		state, ok := app.runtimeState(conversationID)
		expected := "answer-for-" + conversationID
		if !ok || state.LatestAnswer != expected {
			t.Fatalf("%s state = %#v, %v; want %q", conversationID, state, ok, expected)
		}
	}
}

func TestSettingsSetUsesTypedAllowlist(t *testing.T) {
	store := &fakeHistory{settings: make(map[settings.Key]string)}
	app := newTestApp(store)

	invalid := requestWithParams(t, "settings_set", map[string]any{
		"key":   "save_history",
		"value": "false",
	})
	if _, err := app.settingsSet(context.Background(), invalid); err == nil {
		t.Fatal("settingsSet accepted a string boolean")
	}

	valid := requestWithParams(t, "settings_set", map[string]any{
		"key":   "save_history",
		"value": false,
	})
	if _, err := app.settingsSet(context.Background(), valid); err != nil {
		t.Fatalf("settingsSet returned error: %v", err)
	}
	if got := store.settings[settings.SaveHistory]; got != "false" {
		t.Fatalf("stored save_history = %q, want false", got)
	}

	unknown := requestWithParams(t, "settings_set", map[string]any{
		"key":   "arbitrary",
		"value": true,
	})
	if _, err := app.settingsSet(context.Background(), unknown); err == nil {
		t.Fatal("settingsSet accepted an unknown key")
	}
}

func TestDecodeParamsRejectsUnknownFields(t *testing.T) {
	request := requestWithParams(t, "copy_text", map[string]any{
		"text":       "hello",
		"unexpected": true,
	})
	var decoded copyTextRequest
	if err := decodeParams(request, "copy_text", &decoded); err == nil {
		t.Fatal("decodeParams accepted an unknown field")
	}
}

func TestSettingsGetRejectsSetOnlyValue(t *testing.T) {
	store := &fakeHistory{settings: make(map[settings.Key]string)}
	app := newTestApp(store)
	request := requestWithParams(t, "settings_get", map[string]any{
		"key":   "save_history",
		"value": false,
	})
	if _, err := app.settingsGet(context.Background(), request); err == nil {
		t.Fatal("settingsGet accepted a value field")
	}
}

func TestHistoryClearReportsScreenshotCleanupWarning(t *testing.T) {
	store := &fakeHistory{cleanup: history.CleanupResult{
		DeletedEntries:  2,
		AttachmentPaths: []string{"/cache/window-test.png"},
	}}
	app := newTestApp(store)
	app.cleaner = &fakeCleaner{err: errors.New("permission denied")}
	app.setLatestAnswer("chat-one", "answer")

	result, err := app.historyClear(context.Background(), ipc.Request{Method: "history_clear"})
	if err != nil {
		t.Fatalf("historyClear returned error: %v", err)
	}
	response, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("historyClear result type = %T", result)
	}
	if response["deleted_entries"] != int64(2) || response["warning"] == "" {
		t.Fatalf("historyClear result = %#v, want successful clear with warning", response)
	}
	if _, ok := app.runtimeState("chat-one"); ok {
		t.Fatal("historyClear retained runtime conversation state")
	}
}

func requestWithParams(t *testing.T, method string, params any) ipc.Request {
	t.Helper()
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	return ipc.Request{Method: method, Params: encoded}
}
