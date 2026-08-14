package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"hark/internal/ai"
	"hark/internal/ai/openai"
	"hark/internal/ai/openai_compatible"
	"hark/internal/ai/openrouter"
	"hark/internal/ai/providerkit"
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
	ID      string `json:"id"`
	Label   string `json:"label"`
	BaseURL string `json:"base_url"`
}

type providerRemoveRequest struct {
	ID string `json:"id"`
}

type modelAddRequest struct {
	Provider string `json:"provider"`
	ID       string `json:"id"`
	Label    string `json:"label"`
}

type modelRemoveRequest struct {
	ID string `json:"id"`
}

type fetchModelsRequest struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
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
		sort.Slice(attached, func(i, j int) bool { return attached[i].ID < attached[j].ID })
		entries = append(entries, providerListEntry{
			ID:      provider.ID,
			Label:   provider.Label,
			BaseURL: provider.BaseURL,
			Models:  attached,
		})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })
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

	// A config.lua provider is authoritative and cannot be overridden here.
	cfg := a.snapshotConfig()
	for _, spec := range cfg.Providers {
		if spec.ID == id {
			if !a.isManagedProvider(ctx, id) {
				return nil, fmt.Errorf("provider %q is defined in config.lua and cannot be edited here", id)
			}
			break
		}
	}

	if err := a.history.UpsertProvider(ctx, history.Provider{ID: id, Label: label, BaseURL: baseURL}); err != nil {
		return nil, err
	}
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"saved": true}, nil
}

func (a *appState) isManagedProvider(ctx context.Context, id string) bool {
	managed, err := a.history.ListProviders(ctx)
	if err != nil {
		return false
	}
	for _, provider := range managed {
		if provider.ID == id {
			return true
		}
	}
	return false
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
	if !a.isManagedProvider(ctx, id) {
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

func (a *appState) modelsAdd(ctx context.Context, req ipc.Request) (any, error) {
	var request modelAddRequest
	if err := decodeParams(req, "models_add", &request); err != nil {
		return nil, err
	}

	provider := strings.ToLower(strings.TrimSpace(request.Provider))
	if !a.isManagedProvider(ctx, provider) {
		return nil, fmt.Errorf("provider %q is not managed from the panel", provider)
	}
	id := strings.TrimSpace(request.ID)
	if id == "" || len(id) > 128 {
		return nil, errors.New("model id must contain 1 to 128 bytes")
	}
	label := strings.TrimSpace(request.Label)
	if label == "" {
		label = id
	}

	cfg := a.snapshotConfig()
	for _, model := range cfg.Provider.Models {
		if model.ID == id {
			return nil, fmt.Errorf("model %q already exists", id)
		}
	}

	if err := a.history.UpsertModel(ctx, history.Model{ID: id, Label: label, Provider: provider}); err != nil {
		return nil, err
	}
	if err := a.reload(ctx); err != nil {
		_ = a.history.DeleteModel(ctx, id)
		return nil, err
	}
	return map[string]any{"added": true}, nil
}

func (a *appState) modelsRemove(ctx context.Context, req ipc.Request) (any, error) {
	var request modelRemoveRequest
	if err := decodeParams(req, "models_remove", &request); err != nil {
		return nil, err
	}
	id := strings.TrimSpace(request.ID)
	if id == "" {
		return nil, errors.New("model id must not be empty")
	}
	if err := a.history.DeleteModel(ctx, id); err != nil {
		return nil, err
	}
	if err := a.reload(ctx); err != nil {
		return nil, err
	}
	return map[string]any{"removed": true}, nil
}

func (a *appState) providersFetchModels(ctx context.Context, req ipc.Request) (any, error) {
	var request fetchModelsRequest
	if err := decodeParams(req, "providers_fetch_models", &request); err != nil {
		return nil, err
	}

	baseURL := strings.TrimSpace(request.BaseURL)
	apiKey := request.APIKey
	if provider := strings.ToLower(strings.TrimSpace(request.Provider)); provider != "" {
		managed, err := a.history.ListProviders(ctx)
		if err != nil {
			return nil, err
		}
		var found *history.Provider
		for index := range managed {
			if managed[index].ID == provider {
				found = &managed[index]
				break
			}
		}
		if found == nil {
			return nil, fmt.Errorf("provider %q is not managed from the panel", provider)
		}
		baseURL = found.BaseURL
		key, _, err := secrets.ProviderAPIKey(provider)
		if err != nil {
			return nil, err
		}
		apiKey = key
	}

	models, err := fetchProviderModels(ctx, baseURL, apiKey)
	if err != nil {
		return nil, err
	}
	return map[string]any{"models": models}, nil
}

func fetchProviderModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return nil, errors.New("base_url must be an absolute http or https URL")
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	httpReq.Header.Set("Accept", "application/json")
	if strings.TrimSpace(apiKey) != "" {
		httpReq.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	client := providerkit.NewHTTPClient("provider")
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call provider models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerkit.APIError("provider", resp)
	}

	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	ids := make([]string, 0, len(body.Data))
	seen := make(map[string]struct{}, len(body.Data))
	for _, model := range body.Data {
		id := strings.TrimSpace(model.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}
