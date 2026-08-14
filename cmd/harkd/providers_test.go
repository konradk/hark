package main

import (
	"context"
	"testing"

	"hark/internal/config"
)

func TestProvidersAddRegistersProviderAndModel(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	if _, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "local", Label: "Local vLLM", BaseURL: "http://localhost:8000/v1", ModelID: "llama-3", ModelLabel: "Llama 3",
	})); err != nil {
		t.Fatalf("providersAdd returned error: %v", err)
	}

	cfg := app.snapshotConfig()
	if !hasProvider(cfg, "local", "http://localhost:8000/v1") {
		t.Fatalf("merged providers = %#v, want local", cfg.Providers)
	}
	if !hasModel(cfg, "llama-3", "local") {
		t.Fatalf("merged models = %#v, want llama-3 -> local", cfg.Provider.Models)
	}
	if _, ok := app.snapshotProviders()["local"]; !ok {
		t.Fatal("providers map does not contain local")
	}
}

func TestProvidersAddDefaultsLabelToID(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	if _, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "corp-gw", BaseURL: "https://gw.example.com/v1", ModelID: "m1",
	})); err != nil {
		t.Fatalf("providersAdd returned error: %v", err)
	}

	cfg := app.snapshotConfig()
	for _, spec := range cfg.Providers {
		if spec.ID == "corp-gw" && spec.Label != "corp-gw" {
			t.Fatalf("label = %q, want default %q", spec.Label, "corp-gw")
		}
	}
}

func TestProvidersAddRejectsBuiltinID(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "openai", BaseURL: "http://localhost/v1", ModelID: "m1",
	}))
	if err == nil {
		t.Fatal("expected error for a built-in provider id")
	}
}

func TestProvidersAddRejectsInvalidBaseURL(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "local", BaseURL: "not-a-url", ModelID: "m1",
	}))
	if err == nil {
		t.Fatal("expected error for an invalid base URL")
	}
}

func TestProvidersAddRejectsDuplicateModel(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "local", BaseURL: "http://localhost/v1", ModelID: "gpt-test",
	}))
	if err == nil {
		t.Fatal("expected error for a duplicate model id")
	}
}

func TestProvidersRemoveDeletesProviderAndModel(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	if _, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "local", BaseURL: "http://localhost:8000/v1", ModelID: "llama-3",
	})); err != nil {
		t.Fatalf("providersAdd returned error: %v", err)
	}

	if _, err := app.providersRemove(context.Background(), requestWithParams(t, "providers_remove", providerRemoveRequest{ID: "local"})); err != nil {
		t.Fatalf("providersRemove returned error: %v", err)
	}

	cfg := app.snapshotConfig()
	if hasProvider(cfg, "local", "") || hasModel(cfg, "llama-3", "local") {
		t.Fatalf("provider/model not removed: %#v %#v", cfg.Providers, cfg.Provider.Models)
	}
	if _, ok := app.snapshotProviders()["local"]; ok {
		t.Fatal("providers map still contains local")
	}
}

func TestProvidersRemoveRejectsUnmanagedProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersRemove(context.Background(), requestWithParams(t, "providers_remove", providerRemoveRequest{ID: "openai"}))
	if err == nil {
		t.Fatal("expected error removing a provider not managed from the panel")
	}
}

func hasProvider(cfg config.Config, id, baseURL string) bool {
	for _, spec := range cfg.Providers {
		if spec.ID == id && (baseURL == "" || spec.BaseURL == baseURL) {
			return true
		}
	}
	return false
}

func hasModel(cfg config.Config, id, provider string) bool {
	for _, model := range cfg.Provider.Models {
		if model.ID == id && model.Provider == provider {
			return true
		}
	}
	return false
}
