package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"hark/internal/ai"
	"hark/internal/clipboard"
	"hark/internal/config"
	"hark/internal/history"
	"hark/internal/hyprland"
	"hark/internal/paste"
	"hark/internal/screenshot"
	"hark/internal/settings"
)

const (
	maxRuntimeStates           = 256
	maxProviderStateBytesTotal = 32 << 20
	runtimeStateTTL            = 24 * time.Hour
)

type historyRepository interface {
	Add(context.Context, history.Entry) (int64, error)
	List(context.Context, int) ([]history.Entry, error)
	Get(context.Context, int64) (history.Entry, error)
	DeleteConversation(context.Context, int64) (history.CleanupResult, error)
	Clear(context.Context) (history.CleanupResult, error)
	PruneBefore(context.Context, time.Time) (history.CleanupResult, error)
	ReferencedAttachmentPaths(context.Context) ([]string, error)
	GetSetting(context.Context, settings.Key) (string, bool, error)
	SetSetting(context.Context, settings.Key, string) error
	ListProviders(context.Context) ([]history.Provider, error)
	UpsertProvider(context.Context, history.Provider) error
	DeleteProvider(context.Context, string) error
	ListModels(context.Context) ([]history.Model, error)
	UpsertModel(context.Context, history.Model) error
	DeleteModel(context.Context, string) error
}

type screenshotCapturer interface {
	CaptureRegion(context.Context) (screenshot.Attachment, error)
	CaptureWindow(context.Context, int, int, int, int) (screenshot.Attachment, error)
}

type screenshotCleaner interface {
	RemoveManaged([]string) (int, error)
	RemoveAll() (int, error)
	RemoveUnreferenced([]string, time.Time) (int, error)
}

type appLogger interface {
	Printf(string, ...any)
}

type runtimeState struct {
	LatestAnswer   string
	PreviousWindow hyprland.Window
	ProviderState  json.RawMessage
	Provider       string
	Model          string
	UpdatedAt      time.Time
}

type appState struct {
	// baseCfg is the config loaded from config.lua. cfg is the effective,
	// merged config (baseCfg plus panel-managed providers and models); both it
	// and providers are replaced atomically on reload, never mutated in place.
	baseCfg   config.Config
	cfg       config.Config
	providers map[string]ai.Provider
	cfgMu     sync.RWMutex

	clip     clipboard.Clipboard
	paster   paste.Paster
	hypr     hyprland.Client
	capturer screenshotCapturer
	cleaner  screenshotCleaner
	history  historyRepository
	logger   appLogger

	// attachmentDir bounds which files an ask request may attach.
	attachmentDir string

	mu     sync.RWMutex
	states map[string]runtimeState
}

// snapshotConfig returns the effective config. The value and its slices are
// never mutated after publication, so callers may use it without holding the
// lock.
func (a *appState) snapshotConfig() config.Config {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg
}

// snapshotProviders returns the live provider map. It is replaced, never
// mutated, on reload.
func (a *appState) snapshotProviders() map[string]ai.Provider {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.providers
}

func (a *appState) setLatestAnswer(stateID, text string) {
	a.updateRuntimeState(stateID, func(state *runtimeState) {
		state.LatestAnswer = text
	})
}

func (a *appState) setPreviousWindow(stateID string, window hyprland.Window) {
	a.updateRuntimeState(stateID, func(state *runtimeState) {
		state.PreviousWindow = window
	})
}

func (a *appState) setProviderState(stateID, provider, model string, value json.RawMessage) {
	a.updateRuntimeState(stateID, func(state *runtimeState) {
		state.ProviderState = append(state.ProviderState[:0], value...)
		state.Provider = provider
		state.Model = model
	})
	a.pruneRuntimeStates(stateID)
}

func (a *appState) runtimeState(stateID string) (runtimeState, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	state, ok := a.states[stateID]
	state.ProviderState = append(json.RawMessage(nil), state.ProviderState...)
	return state, ok
}

func (a *appState) updateRuntimeState(stateID string, update func(*runtimeState)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.states == nil {
		a.states = make(map[string]runtimeState)
	}
	state, existed := a.states[stateID]
	update(&state)
	state.UpdatedAt = time.Now()
	a.states[stateID] = state
	// Streaming updates an existing key once per token; only a new key can push
	// the map past its bounds, so pruning every time would scan it per token.
	if !existed {
		a.pruneRuntimeStatesLocked(stateID)
	}
}

func (a *appState) clearRuntimeStates() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.states = make(map[string]runtimeState)
}

func (a *appState) pruneRuntimeStates(preserve string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.pruneRuntimeStatesLocked(preserve)
}

func (a *appState) pruneRuntimeStatesLocked(preserve string) {
	cutoff := time.Now().Add(-runtimeStateTTL)
	for stateID, state := range a.states {
		if stateID != preserve && state.UpdatedAt.Before(cutoff) {
			delete(a.states, stateID)
		}
	}
	for len(a.states) > maxRuntimeStates || a.providerStateBytesLocked() > maxProviderStateBytesTotal {
		var oldestID string
		var oldest time.Time
		for stateID, state := range a.states {
			if stateID == preserve {
				continue
			}
			if oldestID == "" || state.UpdatedAt.Before(oldest) {
				oldestID = stateID
				oldest = state.UpdatedAt
			}
		}
		if oldestID == "" {
			return
		}
		delete(a.states, oldestID)
	}
}

func (a *appState) providerStateBytesLocked() int {
	total := 0
	for _, state := range a.states {
		total += len(state.ProviderState)
	}
	return total
}

func (a *appState) allowedModels() []string {
	return allowedModelIDs(a.snapshotConfig())
}

func allowedModelIDs(cfg config.Config) []string {
	models := make([]string, 0, len(cfg.Provider.Models))
	for _, model := range cfg.Provider.Models {
		models = append(models, model.ID)
	}
	return models
}

func (a *appState) providerNameForModel(model string) string {
	provider, ok := configuredProviderForModel(a.snapshotConfig(), model)
	if ok {
		return provider
	}
	return config.ProviderOpenAI
}

func configuredProviderForModel(cfg config.Config, model string) (string, bool) {
	configured, ok := configuredModel(cfg, model)
	if !ok {
		return "", false
	}
	if configured.Provider == "" {
		return config.ProviderOpenAI, true
	}
	return configured.Provider, true
}

func configuredModel(cfg config.Config, model string) (config.ModelConfig, bool) {
	for _, configured := range cfg.Provider.Models {
		if configured.ID == model {
			return configured, true
		}
	}
	return config.ModelConfig{}, false
}

func (a *appState) settingValue(ctx context.Context, key settings.Key) (any, bool, error) {
	stored, found, err := a.history.GetSetting(ctx, key)
	if err != nil {
		return nil, false, err
	}
	cfg := a.snapshotConfig()
	if !found {
		return settings.DefaultValue(key, cfg.Provider.DefaultModel, cfg.Provider.DefaultReasoningEffort), false, nil
	}
	value, err := settings.DecodeStored(key, stored, allowedModelIDs(cfg))
	if err != nil {
		return nil, true, fmt.Errorf("decode stored setting %q: %w", key, err)
	}
	return value, true, nil
}

func (a *appState) saveHistoryEnabled(ctx context.Context) (bool, error) {
	value, _, err := a.settingValue(ctx, settings.SaveHistory)
	if err != nil {
		return false, err
	}
	enabled, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("save_history has unexpected type %T", value)
	}
	return enabled, nil
}

func (a *appState) retentionDays(ctx context.Context) (int, error) {
	value, _, err := a.settingValue(ctx, settings.HistoryRetentionDays)
	if err != nil {
		return 0, err
	}
	days, ok := value.(int)
	if !ok {
		return 0, fmt.Errorf("history_retention_days has unexpected type %T", value)
	}
	return days, nil
}
