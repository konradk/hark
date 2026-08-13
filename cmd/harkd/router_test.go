package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"hark/internal/config"
	"hark/internal/ipc"
	"hark/internal/settings"
)

func TestRouterExposesStatusAndRejectsUnknownMethods(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	server := newIPCServer(nil, cfg, serverMetadata{
		SocketPath: "/tmp/hark-test.sock",
		ConfigPath: "/tmp/hark-test.lua",
	})

	result, err := server.Handler(context.Background(), ipc.Request{Method: "status"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status, ok := result.(ipc.Status)
	if !ok {
		t.Fatalf("status result type = %T, want ipc.Status", result)
	}
	if status.ProtocolVersion != ipc.ProtocolVersion {
		t.Fatalf("protocol version = %d, want %d", status.ProtocolVersion, ipc.ProtocolVersion)
	}
	if status.Model != cfg.Provider.DefaultModel {
		t.Fatalf("model = %q, want %q", status.Model, cfg.Provider.DefaultModel)
	}
	if status.Provider != config.ProviderOpenAI {
		t.Fatalf("provider = %q, want %q", status.Provider, config.ProviderOpenAI)
	}

	_, err = server.Handler(context.Background(), ipc.Request{Method: "unknown"})
	if err == nil || !strings.Contains(err.Error(), "unknown method") {
		t.Fatalf("unknown method error = %v", err)
	}
}

func TestRouterStatusUsesDefaultModelProvider(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Provider.DefaultModel = "anthropic/claude-opus-5"
	server := newIPCServer(nil, cfg, serverMetadata{})
	result, err := server.Handler(context.Background(), ipc.Request{Method: "status"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := result.(ipc.Status)
	if status.Provider != config.ProviderOpenRouter {
		t.Fatalf("provider = %q, want %q", status.Provider, config.ProviderOpenRouter)
	}
}

func TestRouterStatusUsesXAIModelProvider(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	cfg.Provider.DefaultModel = "grok-4.5"
	server := newIPCServer(nil, cfg, serverMetadata{})
	result, err := server.Handler(context.Background(), ipc.Request{Method: "status"})
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	status := result.(ipc.Status)
	if status.Provider != config.ProviderXAI {
		t.Fatalf("provider = %q, want %q", status.Provider, config.ProviderXAI)
	}
}

func TestRouterReturnsReasoningModesForRequestedModel(t *testing.T) {
	t.Parallel()

	cfg := config.Defaults()
	server := newIPCServer(nil, cfg, serverMetadata{})
	params, err := json.Marshal(map[string]string{"model": "anthropic/claude-opus-5"})
	if err != nil {
		t.Fatalf("marshal params: %v", err)
	}
	result, err := server.Handler(context.Background(), ipc.Request{Method: "reasoning_modes_list", Params: params})
	if err != nil {
		t.Fatalf("reasoning_modes_list: %v", err)
	}
	modes, ok := result.([]settings.ReasoningMode)
	if !ok {
		t.Fatalf("result type = %T, want []settings.ReasoningMode", result)
	}
	want := []string{"auto", "none", "low", "medium", "high", "xhigh", "max"}
	if len(modes) != len(want) {
		t.Fatalf("modes = %#v, want %v", modes, want)
	}
	for index, mode := range modes {
		if mode.ID != want[index] {
			t.Fatalf("mode %d = %q, want %q", index, mode.ID, want[index])
		}
	}
}

func TestRouterRejectsUnknownStreamingMethod(t *testing.T) {
	t.Parallel()

	server := newIPCServer(nil, config.Defaults(), serverMetadata{})
	err := server.StreamHandler(
		context.Background(),
		ipc.Request{Method: "unknown"},
		func(any) error { return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "unknown streaming method") {
		t.Fatalf("unknown stream error = %v", err)
	}
}
