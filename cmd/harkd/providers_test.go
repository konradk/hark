package main

import (
	"context"
	"testing"

	"hark/internal/config"
)

func saveProvider(t *testing.T, app *appState, id, baseURL string, models ...string) {
	t.Helper()
	if _, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: id, Label: id, BaseURL: baseURL, Models: models,
	})); err != nil {
		t.Fatalf("providersSave returned error: %v", err)
	}
}

func TestProvidersSaveRegistersProviderAndModels(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	saveProvider(t, app, "local", "http://localhost:8000/v1", "llama-3", "llama-2")

	cfg := app.snapshotConfig()
	if !hasProvider(cfg, "local", "http://localhost:8000/v1") {
		t.Fatalf("merged providers = %#v, want local", cfg.Providers)
	}
	if !hasModel(cfg, "llama-3", "local") || !hasModel(cfg, "llama-2", "local") {
		t.Fatalf("merged models = %#v, want llama-3 and llama-2 -> local", cfg.Provider.Models)
	}
	if _, ok := app.snapshotProviders()["local"]; !ok {
		t.Fatal("providers map does not contain local")
	}
}

func TestProvidersSaveDefaultsLabelToID(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	if _, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: "corp-gw", BaseURL: "https://gw.example.com/v1",
	})); err != nil {
		t.Fatalf("providersSave returned error: %v", err)
	}

	cfg := app.snapshotConfig()
	for _, spec := range cfg.Providers {
		if spec.ID == "corp-gw" && spec.Label != "corp-gw" {
			t.Fatalf("label = %q, want default %q", spec.Label, "corp-gw")
		}
	}
}

func TestProvidersSaveUpdatesBaseURL(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	saveProvider(t, app, "local", "http://localhost:8000/v1", "llama-3")
	saveProvider(t, app, "local", "http://localhost:9000/v1", "llama-3")

	cfg := app.snapshotConfig()
	if !hasProvider(cfg, "local", "http://localhost:9000/v1") {
		t.Fatalf("merged providers = %#v, want updated base URL", cfg.Providers)
	}
	if !hasModel(cfg, "llama-3", "local") {
		t.Fatal("model was dropped during an in-place edit")
	}
}

func TestProvidersSaveRemovesDroppedModels(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	saveProvider(t, app, "local", "http://localhost:8000/v1", "llama-3", "llama-2")
	saveProvider(t, app, "local", "http://localhost:8000/v1", "llama-3")

	cfg := app.snapshotConfig()
	if !hasModel(cfg, "llama-3", "local") {
		t.Fatal("kept model is missing")
	}
	if hasModel(cfg, "llama-2", "local") {
		t.Fatal("dropped model still present")
	}
}

func TestProvidersSaveRejectsBuiltinID(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: "openai", BaseURL: "http://localhost/v1",
	}))
	if err == nil {
		t.Fatal("expected error for a built-in provider id")
	}
}

func TestProvidersSaveRejectsInvalidBaseURL(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: "local", BaseURL: "not-a-url",
	}))
	if err == nil {
		t.Fatal("expected error for an invalid base URL")
	}
}

func TestProvidersSaveRejectsConfigProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	app.baseCfg.Providers = append(app.baseCfg.Providers, config.ProviderSpec{ID: "cfg-provider", BaseURL: "http://cfg/v1"})
	app.cfg = app.baseCfg

	_, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: "cfg-provider", BaseURL: "http://elsewhere/v1",
	}))
	if err == nil {
		t.Fatal("expected error editing a config.lua provider")
	}
}

func TestProvidersSaveRejectsModelOwnedElsewhere(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	saveProvider(t, app, "one", "http://localhost:8000/v1", "shared-model")

	_, err := app.providersSave(context.Background(), requestWithParams(t, "providers_save", providerSaveRequest{
		ID: "two", BaseURL: "http://localhost:9000/v1", Models: []string{"shared-model"},
	}))
	if err == nil {
		t.Fatal("expected error for a model owned by another provider")
	}
}

func TestProvidersRemoveDeletesProviderAndModels(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	saveProvider(t, app, "local", "http://localhost:8000/v1", "llama-3")

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
