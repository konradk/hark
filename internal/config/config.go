package config

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hark/internal/settings"

	lua "github.com/yuin/gopher-lua"
)

// configEvalTimeout is a variable so tests can shorten it.
var configEvalTimeout = 2 * time.Second

type Config struct {
	UI        UIConfig
	Provider  ProviderConfig
	Providers []ProviderSpec
	Paste     PasteConfig
}

type UIConfig struct {
	Theme  string
	Colors ThemeColors
}

// ThemeColors maps a color role to a hex value or "transparent". Roles are
// listed once, in themeColorRoles, so adding one is a single-line change.
type ThemeColors map[string]string

var themeColorRoles = []string{
	"background", "panel", "panel_border",
	"surface", "surface_elevated", "surface_hover", "surface_active",
	"input",
	"button", "button_disabled", "button_hover", "button_down",
	"primary", "primary_hover", "primary_down", "primary_text",
	"text", "text_strong", "text_muted", "text_disabled",
	"assistant",
	"error", "error_text", "error_surface", "error_border",
	"selection_text",
}

type ProviderConfig struct {
	DefaultModel           string
	DefaultReasoningEffort string
	Models                 []ModelConfig
}

type ModelConfig struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Provider         string   `json:"provider"`
	ReasoningEfforts []string `json:"reasoning_efforts"`
}

// ProviderSpec describes a user-defined OpenAI-compatible provider. Its model
// entries reference it by ID and the daemon talks to its base URL using the
// standard OpenAI Chat Completions API.
type ProviderSpec struct {
	ID      string
	Label   string
	BaseURL string
}

const (
	ProviderOpenAI     = "openai"
	ProviderOpenRouter = "openrouter"
	ProviderXAI        = "xai"
)

// IsBuiltinProvider reports whether name is one of the providers shipped with
// Hark, as opposed to a user-defined OpenAI-compatible provider.
func IsBuiltinProvider(name string) bool {
	switch name {
	case ProviderOpenAI, ProviderOpenRouter, ProviderXAI:
		return true
	default:
		return false
	}
}

type PasteConfig struct {
	RestoreFocus bool
	DelayMS      int
	Shortcut     string
}

func Defaults() Config {
	return Config{
		UI: UIConfig{
			Theme:  "system",
			Colors: DefaultThemeColors(),
		},
		Provider: ProviderConfig{
			DefaultModel:           "gpt-5.6-sol",
			DefaultReasoningEffort: "low",
			Models:                 defaultModels(),
		},
		Paste: PasteConfig{
			RestoreFocus: true,
			DelayMS:      80,
			Shortcut:     "ctrl_shift_v",
		},
	}
}

func defaultModels() []ModelConfig {
	return []ModelConfig{
		{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh", "max"}},
		{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh", "max"}},
		{ID: "gpt-5.6-luna", Label: "GPT-5.6 Luna", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh", "max"}},
		{ID: "gpt-5.5", Label: "GPT-5.5", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{ID: "gpt-5.4", Label: "GPT-5.4", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{ID: "gpt-5.4-mini", Label: "GPT-5.4 Mini", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{ID: "gpt-5.4-nano", Label: "GPT-5.4 Nano", Provider: ProviderOpenAI, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh"}},
		{ID: "anthropic/claude-opus-5", Label: "Claude Opus 5 (OpenRouter)", Provider: ProviderOpenRouter, ReasoningEfforts: []string{"auto", "none", "low", "medium", "high", "xhigh", "max"}},
		{ID: "google/gemini-3.6-flash", Label: "Gemini 3.6 Flash (OpenRouter)", Provider: ProviderOpenRouter, ReasoningEfforts: []string{"auto", "minimal", "low", "medium", "high"}},
		{ID: "x-ai/grok-4.6", Label: "Grok 4.6 (OpenRouter)", Provider: ProviderOpenRouter, ReasoningEfforts: []string{"auto", "low", "medium", "high", "xhigh"}},
		{ID: "x-ai/grok-4.5", Label: "Grok 4.5 (OpenRouter)", Provider: ProviderOpenRouter, ReasoningEfforts: []string{"auto", "low", "medium", "high"}},
		{ID: "grok-4.6", Label: "Grok 4.6 (xAI)", Provider: ProviderXAI, ReasoningEfforts: []string{"auto", "low", "medium", "high", "xhigh"}},
		{ID: "grok-4.5", Label: "Grok 4.5 (xAI)", Provider: ProviderXAI, ReasoningEfforts: []string{"auto", "low", "medium", "high"}},
	}
}

func DefaultThemeColors() ThemeColors {
	return ThemeColors{
		"background":       "transparent",
		"panel":            "#15171e",
		"panel_border":     "#2b303b",
		"surface":          "#11141a",
		"surface_elevated": "#1b1f28",
		"surface_hover":    "#222733",
		"surface_active":   "#2a3040",
		"input":            "#0d1016",
		"button":           "#1e232e",
		"button_disabled":  "#171b23",
		"button_hover":     "#262c39",
		"button_down":      "#2e3543",
		"primary":          "#a7c7ff",
		"primary_hover":    "#b9d3ff",
		"primary_down":     "#8fb6f2",
		"primary_text":     "#0b1220",
		"text":             "#d3d8e2",
		"text_strong":      "#f2f4f8",
		"text_muted":       "#8a93a3",
		"text_disabled":    "#5b6373",
		"assistant":        "#8fbce8",
		"error":            "#e07878",
		"error_text":       "#ffb4b4",
		"error_surface":    "#251a1d",
		"error_border":     "#5d3338",
		"selection_text":   "#10131a",
	}
}

func DefaultPath() string {
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "hark", "config.lua")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".config", "hark", "config.lua")
	}

	return filepath.Join(home, ".config", "hark", "config.lua")
}

func Load(path string) (Config, error) {
	cfg := Defaults()
	if path == "" {
		path = DefaultPath()
	}

	source, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	table, err := loadLuaTable(string(source), path)
	if err != nil {
		return Config{}, err
	}
	if err := validateLuaConfig(table); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", path, err)
	}

	applyTable(&cfg, table)
	if err := Validate(cfg); err != nil {
		return Config{}, fmt.Errorf("invalid config %q: %w", path, err)
	}

	return cfg, nil
}

func Validate(cfg Config) error {
	if cfg.UI.Theme == "" {
		return errors.New("ui.theme must not be empty")
	}
	if err := validateThemeColors(cfg.UI.Colors); err != nil {
		return err
	}
	if cfg.Provider.DefaultModel == "" {
		return errors.New("provider.default_model must not be empty")
	}
	if !settings.ValidReasoningEffort(cfg.Provider.DefaultReasoningEffort) {
		return errors.New("provider.default_reasoning_effort must be one of auto, none, minimal, low, medium, high, xhigh, max")
	}
	if len(cfg.Provider.Models) == 0 {
		return errors.New("provider.models must contain at least one model")
	}
	seenProviders := make(map[string]struct{}, len(cfg.Providers))
	for _, provider := range cfg.Providers {
		if provider.ID == "" {
			return errors.New("providers entries must have an id")
		}
		if IsBuiltinProvider(provider.ID) {
			return fmt.Errorf("providers entry %q conflicts with a built-in provider", provider.ID)
		}
		if _, exists := seenProviders[provider.ID]; exists {
			return fmt.Errorf("providers contains duplicate id %q", provider.ID)
		}
		seenProviders[provider.ID] = struct{}{}
		baseURL, err := url.Parse(provider.BaseURL)
		if err != nil || (baseURL.Scheme != "https" && baseURL.Scheme != "http") || baseURL.Host == "" {
			return fmt.Errorf("providers entry %q base_url must be an absolute http or https URL", provider.ID)
		}
	}
	defaultModelFound := false
	var defaultModel ModelConfig
	seenModels := make(map[string]struct{}, len(cfg.Provider.Models))
	for _, model := range cfg.Provider.Models {
		if model.ID == "" {
			return errors.New("provider.models entries must have an id")
		}
		if _, exists := seenModels[model.ID]; exists {
			return fmt.Errorf("provider.models contains duplicate id %q", model.ID)
		}
		seenModels[model.ID] = struct{}{}
		if model.Provider != "" && !IsBuiltinProvider(model.Provider) {
			if _, ok := seenProviders[model.Provider]; !ok {
				return fmt.Errorf("provider.models entry %q has unsupported provider %q", model.ID, model.Provider)
			}
		}
		defaultModelFound = defaultModelFound || model.ID == cfg.Provider.DefaultModel
		if model.ID == cfg.Provider.DefaultModel {
			defaultModel = model
		}
		if len(model.ReasoningEfforts) == 0 {
			return fmt.Errorf("provider.models entry %q must contain at least one reasoning_effort", model.ID)
		}
		seenEfforts := make(map[string]struct{}, len(model.ReasoningEfforts))
		for _, effort := range model.ReasoningEfforts {
			if !settings.ValidReasoningEffort(effort) {
				return fmt.Errorf("provider.models entry %q has unsupported reasoning_effort %q", model.ID, effort)
			}
			if _, exists := seenEfforts[effort]; exists {
				return fmt.Errorf("provider.models entry %q contains duplicate reasoning_effort %q", model.ID, effort)
			}
			seenEfforts[effort] = struct{}{}
		}
	}
	if !defaultModelFound {
		return fmt.Errorf("provider.default_model %q is not present in provider.models", cfg.Provider.DefaultModel)
	}
	if !settings.SupportsReasoningEffort(defaultModel.ReasoningEfforts, cfg.Provider.DefaultReasoningEffort) {
		return fmt.Errorf("provider.default_reasoning_effort %q is not supported by default model %q", cfg.Provider.DefaultReasoningEffort, cfg.Provider.DefaultModel)
	}
	if cfg.Paste.DelayMS < 0 {
		return errors.New("paste.delay_ms must not be negative")
	}
	switch cfg.Paste.Shortcut {
	case "ctrl_shift_v", "ctrl_v", "shift_insert":
	default:
		return errors.New("paste.shortcut must be one of ctrl_shift_v, ctrl_v, shift_insert")
	}
	return nil
}

func loadLuaTable(source, name string) (*lua.LTable, error) {
	L := lua.NewState(lua.Options{SkipOpenLibs: true})
	defer L.Close()

	// Without a deadline a looping config file hangs daemon startup forever.
	ctx, cancel := context.WithTimeout(context.Background(), configEvalTimeout)
	defer cancel()
	L.SetContext(ctx)

	fn, err := L.LoadString(source)
	if err != nil {
		return nil, fmt.Errorf("parse Lua config %q: %w", name, err)
	}

	if err := L.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return nil, fmt.Errorf("execute Lua config %q: %w", name, err)
	}

	value := L.Get(-1)
	table, ok := value.(*lua.LTable)
	if !ok {
		return nil, fmt.Errorf("lua config %q must return a table", name)
	}

	return table, nil
}

func applyTable(cfg *Config, root *lua.LTable) {
	if ui := table(root, "ui"); ui != nil {
		cfg.UI.Theme = stringValue(ui, "theme", cfg.UI.Theme)
		if colors := table(ui, "colors"); colors != nil {
			applyThemeColors(cfg.UI.Colors, colors)
		}
	}

	if provider := table(root, "provider"); provider != nil {
		cfg.Provider.DefaultModel = stringValue(provider, "default_model", cfg.Provider.DefaultModel)
		cfg.Provider.DefaultReasoningEffort = stringValue(provider, "default_reasoning_effort", cfg.Provider.DefaultReasoningEffort)
		if models := table(provider, "models"); models != nil {
			cfg.Provider.Models = modelList(models)
		}
	}

	if providers := table(root, "providers"); providers != nil {
		cfg.Providers = providerList(providers)
	}

	if paste := table(root, "paste"); paste != nil {
		cfg.Paste.RestoreFocus = boolValue(paste, "restore_focus", cfg.Paste.RestoreFocus)
		cfg.Paste.DelayMS = intValue(paste, "delay_ms", cfg.Paste.DelayMS)
		cfg.Paste.Shortcut = stringValue(paste, "shortcut", cfg.Paste.Shortcut)
	}

}

func validateLuaConfig(root *lua.LTable) error {
	if err := validateKeys(root, "config", "ui", "provider", "providers", "paste"); err != nil {
		return err
	}
	if err := validateOptionalTable(root, "ui", "config.ui", func(ui *lua.LTable) error {
		if err := validateKeys(ui, "ui", "theme", "colors"); err != nil {
			return err
		}
		if err := validateOptionalType(ui, "theme", "ui.theme", lua.LTString); err != nil {
			return err
		}
		return validateOptionalTable(ui, "colors", "ui.colors", func(colors *lua.LTable) error {
			if err := validateKeys(colors, "ui.colors", themeColorRoles...); err != nil {
				return err
			}
			for _, role := range themeColorRoles {
				if err := validateOptionalType(colors, role, "ui.colors."+role, lua.LTString); err != nil {
					return err
				}
			}
			return nil
		})
	}); err != nil {
		return err
	}
	if err := validateOptionalTable(root, "provider", "config.provider", func(provider *lua.LTable) error {
		if err := validateKeys(provider, "provider", "default_model", "default_reasoning_effort", "models"); err != nil {
			return err
		}
		for _, field := range []string{"default_model", "default_reasoning_effort"} {
			if err := validateOptionalType(provider, field, "provider."+field, lua.LTString); err != nil {
				return err
			}
		}
		return validateOptionalTable(provider, "models", "provider.models", validateModelList)
	}); err != nil {
		return err
	}
	if err := validateOptionalTable(root, "providers", "config.providers", validateProviderList); err != nil {
		return err
	}
	return validateOptionalTable(root, "paste", "config.paste", func(paste *lua.LTable) error {
		if err := validateKeys(paste, "paste", "restore_focus", "delay_ms", "shortcut"); err != nil {
			return err
		}
		if err := validateOptionalType(paste, "restore_focus", "paste.restore_focus", lua.LTBool); err != nil {
			return err
		}
		if err := validateOptionalType(paste, "shortcut", "paste.shortcut", lua.LTString); err != nil {
			return err
		}
		value := paste.RawGetString("delay_ms")
		if value == lua.LNil {
			return nil
		}
		number, ok := value.(lua.LNumber)
		if !ok || float64(int(number)) != float64(number) {
			return fmt.Errorf("paste.delay_ms must be an integer, got %s", value.Type())
		}
		return nil
	})
}

func validateModelList(models *lua.LTable) error {
	count := 0
	var validationErr error
	models.ForEach(func(key, value lua.LValue) {
		if validationErr != nil {
			return
		}
		index, ok := key.(lua.LNumber)
		if !ok || index < 1 || float64(int(index)) != float64(index) {
			validationErr = fmt.Errorf("provider.models must be a contiguous array, got key %q", key.String())
			return
		}
		count++
		switch model := value.(type) {
		case lua.LString:
		case *lua.LTable:
			if err := validateKeys(model, fmt.Sprintf("provider.models[%d]", int(index)), "id", "label", "provider", "reasoning_efforts"); err != nil {
				validationErr = err
				return
			}
			for _, field := range []string{"id", "label", "provider"} {
				if err := validateOptionalType(model, field, fmt.Sprintf("provider.models[%d].%s", int(index), field), lua.LTString); err != nil {
					validationErr = err
					return
				}
			}
			id, ok := model.RawGetString("id").(lua.LString)
			if !ok || strings.TrimSpace(string(id)) == "" {
				validationErr = fmt.Errorf("provider.models[%d].id must be a non-empty string", int(index))
				return
			}
			if err := validateOptionalTable(model, "reasoning_efforts", fmt.Sprintf("provider.models[%d].reasoning_efforts", int(index)), validateReasoningEffortList); err != nil {
				validationErr = err
			}
		default:
			validationErr = fmt.Errorf("provider.models[%d] must be a string or table, got %s", int(index), value.Type())
		}
	})
	if validationErr != nil {
		return validationErr
	}
	for index := 1; index <= count; index++ {
		if models.RawGetInt(index) == lua.LNil {
			return fmt.Errorf("provider.models must be a contiguous array; index %d is missing", index)
		}
	}
	return nil
}

func validateReasoningEffortList(efforts *lua.LTable) error {
	count := 0
	var validationErr error
	efforts.ForEach(func(key, value lua.LValue) {
		if validationErr != nil {
			return
		}
		index, ok := key.(lua.LNumber)
		if !ok || index < 1 || float64(int(index)) != float64(index) {
			validationErr = fmt.Errorf("reasoning_efforts must be a contiguous array, got key %q", key.String())
			return
		}
		count++
		if value.Type() != lua.LTString {
			validationErr = fmt.Errorf("reasoning_efforts[%d] must be a string, got %s", int(index), value.Type())
		}
	})
	if validationErr != nil {
		return validationErr
	}
	if count == 0 {
		return errors.New("reasoning_efforts must contain at least one effort")
	}
	for index := 1; index <= count; index++ {
		if efforts.RawGetInt(index) == lua.LNil {
			return fmt.Errorf("reasoning_efforts must be a contiguous array; index %d is missing", index)
		}
	}
	return nil
}

func validateProviderList(providers *lua.LTable) error {
	count := 0
	var validationErr error
	providers.ForEach(func(key, value lua.LValue) {
		if validationErr != nil {
			return
		}
		index, ok := key.(lua.LNumber)
		if !ok || index < 1 || float64(int(index)) != float64(index) {
			validationErr = fmt.Errorf("providers must be a contiguous array, got key %q", key.String())
			return
		}
		count++
		entry, ok := value.(*lua.LTable)
		if !ok {
			validationErr = fmt.Errorf("providers[%d] must be a table, got %s", int(index), value.Type())
			return
		}
		if err := validateKeys(entry, fmt.Sprintf("providers[%d]", int(index)), "id", "label", "base_url"); err != nil {
			validationErr = err
			return
		}
		for _, field := range []string{"id", "label", "base_url"} {
			if err := validateOptionalType(entry, field, fmt.Sprintf("providers[%d].%s", int(index), field), lua.LTString); err != nil {
				validationErr = err
				return
			}
		}
		id, ok := entry.RawGetString("id").(lua.LString)
		if !ok || strings.TrimSpace(string(id)) == "" {
			validationErr = fmt.Errorf("providers[%d].id must be a non-empty string", int(index))
			return
		}
	})
	if validationErr != nil {
		return validationErr
	}
	for index := 1; index <= count; index++ {
		if providers.RawGetInt(index) == lua.LNil {
			return fmt.Errorf("providers must be a contiguous array; index %d is missing", index)
		}
	}
	return nil
}

func validateKeys(tbl *lua.LTable, path string, allowed ...string) error {
	known := make(map[string]struct{}, len(allowed))
	for _, key := range allowed {
		known[key] = struct{}{}
	}
	var validationErr error
	tbl.ForEach(func(key, _ lua.LValue) {
		if validationErr != nil {
			return
		}
		name, ok := key.(lua.LString)
		if !ok {
			validationErr = fmt.Errorf("%s contains non-string key %q", path, key.String())
			return
		}
		if _, ok := known[string(name)]; !ok {
			validationErr = fmt.Errorf("%s contains unknown key %q", path, string(name))
		}
	})
	return validationErr
}

func validateOptionalTable(parent *lua.LTable, key, path string, validate func(*lua.LTable) error) error {
	value := parent.RawGetString(key)
	if value == lua.LNil {
		return nil
	}
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return fmt.Errorf("%s must be a table, got %s", path, value.Type())
	}
	return validate(tbl)
}

func validateOptionalType(tbl *lua.LTable, key, path string, expected lua.LValueType) error {
	value := tbl.RawGetString(key)
	if value == lua.LNil || value.Type() == expected {
		return nil
	}
	return fmt.Errorf("%s must be %s, got %s", path, expected, value.Type())
}

func validateThemeColors(colors ThemeColors) error {
	for _, role := range themeColorRoles {
		if !validColor(colors[role]) {
			return fmt.Errorf("ui.colors.%s must be a hex color like #aabbcc or transparent", role)
		}
	}
	return nil
}

func validColor(value string) bool {
	if value == "transparent" {
		return true
	}
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func applyThemeColors(colors ThemeColors, tbl *lua.LTable) {
	for _, role := range themeColorRoles {
		colors[role] = stringValue(tbl, role, colors[role])
	}
}

func table(tbl *lua.LTable, key string) *lua.LTable {
	value := tbl.RawGetString(key)
	if value == lua.LNil {
		return nil
	}
	nested, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	return nested
}

func stringValue(tbl *lua.LTable, key, fallback string) string {
	value := tbl.RawGetString(key)
	if str, ok := value.(lua.LString); ok {
		return string(str)
	}
	return fallback
}

func boolValue(tbl *lua.LTable, key string, fallback bool) bool {
	value := tbl.RawGetString(key)
	if b, ok := value.(lua.LBool); ok {
		return bool(b)
	}
	return fallback
}

func intValue(tbl *lua.LTable, key string, fallback int) int {
	value := tbl.RawGetString(key)
	if n, ok := value.(lua.LNumber); ok {
		return int(n)
	}
	return fallback
}

func modelList(tbl *lua.LTable) []ModelConfig {
	var models []ModelConfig
	tbl.ForEach(func(_, value lua.LValue) {
		switch v := value.(type) {
		case lua.LString:
			id := string(v)
			models = append(models, ModelConfig{ID: id, Label: id, ReasoningEfforts: defaultReasoningEffortsFor(id)})
		case *lua.LTable:
			id := stringValue(v, "id", "")
			label := stringValue(v, "label", id)
			provider := stringValue(v, "provider", "")
			reasoningEfforts := defaultReasoningEffortsFor(id)
			if configured := table(v, "reasoning_efforts"); configured != nil {
				reasoningEfforts = stringList(configured)
			}
			if id != "" {
				models = append(models, ModelConfig{ID: id, Label: label, Provider: provider, ReasoningEfforts: reasoningEfforts})
			}
		}
	})
	return models
}

func providerList(tbl *lua.LTable) []ProviderSpec {
	var providers []ProviderSpec
	tbl.ForEach(func(_, value lua.LValue) {
		entry, ok := value.(*lua.LTable)
		if !ok {
			return
		}
		id := stringValue(entry, "id", "")
		if id == "" {
			return
		}
		providers = append(providers, ProviderSpec{
			ID:      id,
			Label:   stringValue(entry, "label", id),
			BaseURL: stringValue(entry, "base_url", ""),
		})
	})
	return providers
}

func defaultReasoningEffortsFor(id string) []string {
	for _, model := range defaultModels() {
		if model.ID == id {
			return append([]string(nil), model.ReasoningEfforts...)
		}
	}
	return []string{"auto", "low", "medium", "high"}
}

func stringList(tbl *lua.LTable) []string {
	values := make([]string, 0, tbl.Len())
	for index := 1; index <= tbl.Len(); index++ {
		values = append(values, string(tbl.RawGetInt(index).(lua.LString)))
	}
	return values
}
