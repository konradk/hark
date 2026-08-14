package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"hark/internal/ai"
	"hark/internal/ai/openai"
	"hark/internal/ai/openai_compatible"
	"hark/internal/ai/openrouter"
	"hark/internal/ai/xai"
	"hark/internal/config"
	"hark/internal/history"
	"hark/internal/ipc"
	"hark/internal/secrets"
)

func apiKeyProvider(name string) func() (string, error) {
	return func() (string, error) {
		key, _, err := secrets.ProviderAPIKey(name)
		return key, err
	}
}

// newProviderMap builds the full provider registry for cfg: the three built-in
// providers plus every user-defined OpenAI-compatible provider.
func newProviderMap(cfg config.Config) map[string]ai.Provider {
	providers := map[string]ai.Provider{
		openai.ProviderName:     openai.NewWithAPIKeyProvider(apiKeyProvider(openai.ProviderName)),
		openrouter.ProviderName: openrouter.NewWithAPIKeyProvider(apiKeyProvider(openrouter.ProviderName)),
		xai.ProviderName:        xai.NewWithAPIKeyProvider(apiKeyProvider(xai.ProviderName)),
	}
	for _, spec := range cfg.Providers {
		providers[spec.ID] = openai_compatible.New(spec.Label, spec.BaseURL, apiKeyProvider(spec.ID))
	}
	return providers
}

// mergedConfig overlays the panel-managed providers and models on top of the
// config.lua config. Config.lua entries win on ID collisions.
func (a *appState) mergedConfig(ctx context.Context) (config.Config, error) {
	cfg := a.baseCfg

	storedProviders, err := a.history.ListProviders(ctx)
	if err != nil {
		return cfg, err
	}
	storedModels, err := a.history.ListModels(ctx)
	if err != nil {
		return cfg, err
	}

	configProviderIDs := make(map[string]struct{}, len(cfg.Providers))
	for _, spec := range cfg.Providers {
		configProviderIDs[spec.ID] = struct{}{}
	}
	for _, provider := range storedProviders {
		if _, exists := configProviderIDs[provider.ID]; exists {
			continue
		}
		cfg.Providers = append(cfg.Providers, config.ProviderSpec{
			ID:      provider.ID,
			Label:   provider.Label,
			BaseURL: provider.BaseURL,
		})
	}

	configModelIDs := make(map[string]struct{}, len(cfg.Provider.Models))
	for _, model := range cfg.Provider.Models {
		configModelIDs[model.ID] = struct{}{}
	}
	for _, model := range storedModels {
		if _, exists := configModelIDs[model.ID]; exists {
			continue
		}
		cfg.Provider.Models = append(cfg.Provider.Models, config.ModelConfig{
			ID:               model.ID,
			Label:            model.Label,
			Provider:         model.Provider,
			ReasoningEfforts: []string{"auto", "low", "medium", "high"},
		})
	}

	if err := config.Validate(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// reload recomputes the effective config and provider map from config.lua plus
// the panel-managed store, then swaps them into place atomically.
func (a *appState) reload(ctx context.Context) error {
	cfg, err := a.mergedConfig(ctx)
	if err != nil {
		return err
	}

	a.cfgMu.Lock()
	a.cfg = cfg
	a.providers = newProviderMap(cfg)
	a.cfgMu.Unlock()
	return nil
}

type providerListModel struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type providerListEntry struct {
	ID      string              `json:"id"`
	Label   string              `json:"label"`
	BaseURL string              `json:"base_url"`
	Models  []providerListModel `json:"models"`
}

type providerAddRequest struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	BaseURL    string `json:"base_url"`
	ModelID    string `json:"model_id"`
	ModelLabel string `json:"model_label"`
}

type providerRemoveRequest struct {
	ID string `json:"id"`
}

func (a *appState) providersList(ctx context.Context, req ipc.Request) (any, error) {
	if err := validateNoParams(req, "providers_list"); err != nil {
		return nil, err
	}
	providers, err := a.history.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	models, err := a.history.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	modelsByProvider := make(map[string][]providerListModel, len(providers))
	for _, model := range models {
		modelsByProvider[model.Provider] = append(modelsByProvider[model.Provider], providerListModel{ID: model.ID, Label: model.Label})
	}

	entries := make([]providerListEntry, 0, len(providers))
	for _, provider := range providers {
		attached := modelsByProvider[provider.ID]
		if attached == nil {
			attached = []providerListModel{}
		}
		entries = append(entries, providerListEntry{
			ID:      provider.ID,
			Label:   provider.Label,
			BaseURL: provider.BaseURL,
			Models:  attached,
		})
	}
	return entries, nil
}

func (a *appState) providersAdd(ctx context.Context, req ipc.Request) (any, error) {
	var request providerAddRequest
	if err := decodeParams(req, "providers_add", &request); err != nil {
		return nil, err
	}

	id := strings.ToLower(strings.TrimSpace(request.ID))
	if !secrets.ValidProviderName(id) {
		return nil, fmt.Errorf("provider id %q must be 1-64 characters of a-z, 0-9, '.', '_' or '-'", request.ID)
	}
	if config.IsBuiltinProvider(id) {
		return nil, fmt.Errorf("provider id %q conflicts with a built-in provider", id)
	}
	label := strings.TrimSpace(request.Label)
	if label == "" {
		label = id
	}
	baseURL := strings.TrimSpace(request.BaseURL)
	if parsed, err := url.Parse(baseURL); err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, errors.New("base_url must be an absolute http or https URL")
	}
	modelID := strings.TrimSpace(request.ModelID)
	if modelID == "" || len(modelID) > 128 {
		return nil, errors.New("model_id must contain 1 to 128 bytes")
	}
	modelLabel := strings.TrimSpace(request.ModelLabel)
	if modelLabel == "" {
		modelLabel = modelID
	}

	cfg := a.snapshotConfig()
	for _, spec := range cfg.Providers {
		if spec.ID == id {
			return nil, fmt.Errorf("provider %q already exists", id)
		}
	}
	for _, model := range cfg.Provider.Models {
		if model.ID == modelID {
			return nil, fmt.Errorf("model %q already exists", modelID)
		}
	}

	if err := a.history.UpsertProvider(ctx, history.Provider{ID: id, Label: label, BaseURL: baseURL}); err != nil {
		return nil, err
	}
	if err := a.history.UpsertModel(ctx, history.Model{ID: modelID, Label: modelLabel, Provider: id}); err != nil {
		_ = a.history.DeleteProvider(ctx, id)
		return nil, err
	}
	if err := a.reload(ctx); err != nil {
		_ = a.history.DeleteProvider(ctx, id)
		return nil, err
	}
	return map[string]any{"added": true}, nil
}

func (a *appState) providersRemove(ctx context.Context, req ipc.Request) (any, error) {
	var request providerRemoveRequest
	if err := decodeParams(req, "providers_remove", &request); err != nil {
		return nil, err
	}
	id := strings.ToLower(strings.TrimSpace(request.ID))
	if id == "" {
		return nil, errors.New("provider id must not be empty")
	}

	managed, err := a.history.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, provider := range managed {
		if provider.ID == id {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("provider %q is not managed from the panel", id)
	}

	if err := a.history.DeleteProvider(ctx, id); err != nil {
		return nil, err
	}
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"removed": true}, nil
}
