package main

import (
	"context"
	"testing"

	"hark/internal/config"
)

func addProvider(t *testing.T, app *appState, id, baseURL string) {
	t.Helper()
	if _, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: id, Label: id, BaseURL: baseURL,
	})); err != nil {
		t.Fatalf("providersAdd returned error: %v", err)
	}
}

func addModel(t *testing.T, app *appState, provider, id string) {
	t.Helper()
	if _, err := app.modelsAdd(context.Background(), requestWithParams(t, "models_add", modelAddRequest{
		Provider: provider, ID: id,
	})); err != nil {
		t.Fatalf("modelsAdd returned error: %v", err)
	}
}

func TestProvidersAddRegistersProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	addProvider(t, app, "local", "http://localhost:8000/v1")

	cfg := app.snapshotConfig()
	if !hasProvider(cfg, "local", "http://localhost:8000/v1") {
		t.Fatalf("merged providers = %#v, want local", cfg.Providers)
	}
	if _, ok := app.snapshotProviders()["local"]; !ok {
		t.Fatal("providers map does not contain local")
	}
}

func TestProvidersAddDefaultsLabelToID(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	if _, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "corp-gw", BaseURL: "https://gw.example.com/v1",
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

func TestProvidersAddUpdatesExistingProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})

	addProvider(t, app, "local", "http://localhost:8000/v1")
	addProvider(t, app, "local", "http://localhost:9000/v1")

	cfg := app.snapshotConfig()
	if !hasProvider(cfg, "local", "http://localhost:9000/v1") {
		t.Fatalf("merged providers = %#v, want updated base URL", cfg.Providers)
	}
	if hasProvider(cfg, "local", "http://localhost:8000/v1") {
		t.Fatal("stale base URL still present")
	}
}

func TestProvidersAddRejectsBuiltinID(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "openai", BaseURL: "http://localhost/v1",
	}))
	if err == nil {
		t.Fatal("expected error for a built-in provider id")
	}
}

func TestProvidersAddRejectsInvalidBaseURL(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "local", BaseURL: "not-a-url",
	}))
	if err == nil {
		t.Fatal("expected error for an invalid base URL")
	}
}

func TestProvidersAddRejectsConfigProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	app.baseCfg.Providers = append(app.baseCfg.Providers, config.ProviderSpec{ID: "cfg-provider", BaseURL: "http://cfg/v1"})
	app.cfg = app.baseCfg

	_, err := app.providersAdd(context.Background(), requestWithParams(t, "providers_add", providerAddRequest{
		ID: "cfg-provider", BaseURL: "http://elsewhere/v1",
	}))
	if err == nil {
		t.Fatal("expected error editing a config.lua provider")
	}
}

func TestModelsAddAppearsInMergedConfig(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	addProvider(t, app, "local", "http://localhost:8000/v1")
	addModel(t, app, "local", "llama-3")

	cfg := app.snapshotConfig()
	if !hasModel(cfg, "llama-3", "local") {
		t.Fatalf("merged models = %#v, want llama-3 -> local", cfg.Provider.Models)
	}
}

func TestModelsAddRejectsDuplicateModel(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	addProvider(t, app, "local", "http://localhost:8000/v1")
	addModel(t, app, "local", "llama-3")

	_, err := app.modelsAdd(context.Background(), requestWithParams(t, "models_add", modelAddRequest{
		Provider: "local", ID: "llama-3",
	}))
	if err == nil {
		t.Fatal("expected error for a duplicate model id")
	}
}

func TestModelsAddRejectsUnmanagedProvider(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	_, err := app.modelsAdd(context.Background(), requestWithParams(t, "models_add", modelAddRequest{
		Provider: "openai", ID: "gpt-999",
	}))
	if err == nil {
		t.Fatal("expected error for an unmanaged provider")
	}
}

func TestModelsRemoveDeletesModel(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	addProvider(t, app, "local", "http://localhost:8000/v1")
	addModel(t, app, "local", "llama-3")

	if _, err := app.modelsRemove(context.Background(), requestWithParams(t, "models_remove", modelRemoveRequest{ID: "llama-3"})); err != nil {
		t.Fatalf("modelsRemove returned error: %v", err)
	}

	cfg := app.snapshotConfig()
	if hasModel(cfg, "llama-3", "local") {
		t.Fatal("model not removed")
	}
}

func TestProvidersRemoveDeletesProviderAndModels(t *testing.T) {
	app := newTestApp(&fakeHistory{})
	addProvider(t, app, "local", "http://localhost:8000/v1")
	addModel(t, app, "local", "llama-3")

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
