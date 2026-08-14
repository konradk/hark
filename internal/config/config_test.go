package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadMissingConfigUsesDefaults(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.lua"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider.DefaultModel == "" {
		t.Fatal("default model is empty")
	}
}

func TestLoadLuaOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(path, []byte(`
return {
  ui = {
    theme = "dark",
    colors = {
      panel = "#20242c",
      primary = "#8bd5ca",
      text = "#eeeeee",
    },
  },
  provider = {
    default_model = "gpt-test",
    default_reasoning_effort = "high",
    models = {
      { id = "gpt-test", label = "Test", provider = "openrouter", reasoning_efforts = { "auto", "low", "high" } },
      "gpt-other",
    },
  },
  paste = {
    restore_focus = false,
    delay_ms = 120,
    shortcut = "ctrl_v",
  },
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Provider.DefaultModel != "gpt-test" {
		t.Fatalf("unexpected model: %q", cfg.Provider.DefaultModel)
	}
	if cfg.UI.Colors["panel"] != "#20242c" {
		t.Fatalf("unexpected panel color: %q", cfg.UI.Colors["panel"])
	}
	if cfg.UI.Colors["primary"] != "#8bd5ca" {
		t.Fatalf("unexpected primary color: %q", cfg.UI.Colors["primary"])
	}
	if cfg.UI.Colors["text_muted"] != DefaultThemeColors()["text_muted"] {
		t.Fatalf("expected unspecified color fallback, got %q", cfg.UI.Colors["text_muted"])
	}
	if cfg.Provider.DefaultReasoningEffort != "high" {
		t.Fatalf("unexpected reasoning effort: %q", cfg.Provider.DefaultReasoningEffort)
	}
	if len(cfg.Provider.Models) != 2 {
		t.Fatalf("unexpected models: %#v", cfg.Provider.Models)
	}
	if cfg.Provider.Models[0].ID != "gpt-test" || cfg.Provider.Models[0].Label != "Test" || cfg.Provider.Models[0].Provider != "openrouter" {
		t.Fatalf("unexpected first model: %#v", cfg.Provider.Models[0])
	}
	if got := cfg.Provider.Models[0].ReasoningEfforts; len(got) != 3 || got[0] != "auto" || got[2] != "high" {
		t.Fatalf("unexpected first model reasoning efforts: %#v", got)
	}
	if cfg.Provider.Models[1].ID != "gpt-other" || cfg.Provider.Models[1].Provider != "" {
		t.Fatalf("unexpected second model: %#v", cfg.Provider.Models[1])
	}
	if cfg.Paste.RestoreFocus {
		t.Fatal("expected restore_focus to be false")
	}
	if cfg.Paste.DelayMS != 120 {
		t.Fatalf("unexpected delay: %d", cfg.Paste.DelayMS)
	}
	if cfg.Paste.Shortcut != "ctrl_v" {
		t.Fatalf("unexpected shortcut: %q", cfg.Paste.Shortcut)
	}
}

func TestValidateRejectsMissingAndDuplicateDefaultModels(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Models = nil
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted an empty model list")
	}

	cfg = Defaults()
	cfg.Provider.DefaultModel = "missing"
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted a missing default model")
	}

	cfg = Defaults()
	cfg.Provider.Models = append(cfg.Provider.Models, cfg.Provider.Models[0])
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted duplicate model ids")
	}
}

func TestValidateRejectsUnknownModelProvider(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "mystery-model", Provider: "mystery", ReasoningEfforts: []string{"auto"}})
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted an unknown model provider")
	}
}

func TestValidateAcceptsModelWithoutProvider(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "legacy-model", ReasoningEfforts: []string{"auto", "low"}})
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate rejected a model with no provider: %v", err)
	}
}

func TestValidateAcceptsXAIProvider(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "grok-test", Provider: ProviderXAI, ReasoningEfforts: []string{"auto", "low"}})
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate rejected xAI provider: %v", err)
	}
}

func TestValidateRejectsReasoningUnsupportedByDefaultModel(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.DefaultReasoningEffort = "minimal"
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted minimal reasoning for GPT-5.6")
	}
}

func TestDefaultModelsDeclareProviderReasoningEfforts(t *testing.T) {
	cfg := Defaults()
	want := map[string][]string{
		"gpt-5.6-sol":             {"auto", "none", "low", "medium", "high", "xhigh", "max"},
		"gpt-5.5":                 {"auto", "none", "low", "medium", "high", "xhigh"},
		"anthropic/claude-opus-5": {"auto", "none", "low", "medium", "high", "xhigh", "max"},
		"google/gemini-3.6-flash": {"auto", "minimal", "low", "medium", "high"},
		"x-ai/grok-4.6":           {"auto", "low", "medium", "high", "xhigh"},
		"x-ai/grok-4.5":           {"auto", "low", "medium", "high"},
		"grok-4.6":                {"auto", "low", "medium", "high", "xhigh"},
		"grok-4.5":                {"auto", "low", "medium", "high"},
	}
	for _, model := range cfg.Provider.Models {
		expected, ok := want[model.ID]
		if !ok {
			continue
		}
		if len(model.ReasoningEfforts) != len(expected) {
			t.Fatalf("%s efforts = %#v, want %#v", model.ID, model.ReasoningEfforts, expected)
		}
		for index := range expected {
			if model.ReasoningEfforts[index] != expected[index] {
				t.Fatalf("%s efforts = %#v, want %#v", model.ID, model.ReasoningEfforts, expected)
			}
		}
	}
}

func TestLoadLuaRequiresTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(path, []byte(`return "bad"`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected error for non-table config")
	}
}

func TestLoadRejectsInvalidThemeColor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(path, []byte(`
return {
  ui = {
    colors = {
      panel = "not-a-color",
    },
  },
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("expected invalid color error")
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	for name, source := range map[string]string{
		"root":         `return { providre = {} }`,
		"nested":       `return { paste = { delay = 10 } }`,
		"theme color":  `return { ui = { colors = { typo_role = "#ffffff" } } }`,
		"model object": `return { provider = { models = { { id = "gpt-test", typo = true } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.lua")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted an unknown config key")
			}
		})
	}
}

func TestLoadRejectsWrongTypes(t *testing.T) {
	for name, source := range map[string]string{
		"section":        `return { ui = "dark" }`,
		"string":         `return { ui = { theme = true } }`,
		"boolean":        `return { paste = { restore_focus = "yes" } }`,
		"fractional int": `return { paste = { delay_ms = 1.5 } }`,
		"model field":    `return { provider = { models = { { id = 123 } } } }`,
		"missing id":     `return { provider = { models = { { label = "Test" } } } }`,
		"model entry":    `return { provider = { models = { true } } }`,
		"efforts field":  `return { provider = { models = { { id = "gpt-test", reasoning_efforts = "high" } } } }`,
		"effort entry":   `return { provider = { models = { { id = "gpt-test", reasoning_efforts = { true } } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.lua")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted a config value with the wrong type")
			}
		})
	}
}

func TestLoadRejectsEmptyOrSparseModelList(t *testing.T) {
	for name, source := range map[string]string{
		"empty":         `return { provider = { models = {} } }`,
		"sparse":        `return { provider = { models = { [2] = "gpt-test" } } }`,
		"empty efforts": `return { provider = { models = { { id = "gpt-test", reasoning_efforts = {} } } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.lua")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted an invalid model list")
			}
		})
	}
}

func TestLoadRejectsNonTerminatingConfig(t *testing.T) {
	previous := configEvalTimeout
	configEvalTimeout = 100 * time.Millisecond
	t.Cleanup(func() { configEvalTimeout = previous })

	path := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(path, []byte("while true do end\nreturn {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := Load(path)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("looping config unexpectedly loaded")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Load did not return; the config evaluation timeout is not applied")
	}
}

func TestLoadProvidersConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(path, []byte(`
return {
  providers = {
    { id = "local", label = "Local vLLM", base_url = "http://localhost:8000/v1" },
    { id = "corp-gw", base_url = "https://gw.example.com/v1" },
  },
  provider = {
    default_model = "llama-3",
    default_reasoning_effort = "auto",
    models = {
      { id = "llama-3", label = "Llama 3", provider = "local", reasoning_efforts = { "auto", "low" } },
    },
  },
}
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if len(cfg.Providers) != 2 {
		t.Fatalf("unexpected providers: %#v", cfg.Providers)
	}
	if cfg.Providers[0].ID != "local" || cfg.Providers[0].Label != "Local vLLM" || cfg.Providers[0].BaseURL != "http://localhost:8000/v1" {
		t.Fatalf("unexpected first provider: %#v", cfg.Providers[0])
	}
	if cfg.Providers[1].ID != "corp-gw" || cfg.Providers[1].Label != "corp-gw" || cfg.Providers[1].BaseURL != "https://gw.example.com/v1" {
		t.Fatalf("unexpected second provider: %#v", cfg.Providers[1])
	}
	if cfg.Provider.DefaultModel != "llama-3" || cfg.Provider.Models[0].Provider != "local" {
		t.Fatalf("unexpected provider config: %#v", cfg.Provider)
	}
}

func TestValidateAcceptsCustomProvider(t *testing.T) {
	cfg := Defaults()
	cfg.Providers = append(cfg.Providers, ProviderSpec{ID: "local", BaseURL: "http://localhost:8000/v1"})
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "llama-3", Provider: "local", ReasoningEfforts: []string{"auto"}})
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate rejected a custom provider: %v", err)
	}
}

func TestValidateRejectsReservedProviderID(t *testing.T) {
	cfg := Defaults()
	cfg.Providers = append(cfg.Providers, ProviderSpec{ID: ProviderOpenAI, BaseURL: "http://localhost:8000/v1"})
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted a provider id that collides with a built-in")
	}
}

func TestValidateRejectsDuplicateProviderID(t *testing.T) {
	cfg := Defaults()
	cfg.Providers = append(cfg.Providers,
		ProviderSpec{ID: "local", BaseURL: "http://localhost:8000/v1"},
		ProviderSpec{ID: "local", BaseURL: "http://localhost:9000/v1"},
	)
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted duplicate provider ids")
	}
}

func TestValidateRejectsInvalidProviderBaseURL(t *testing.T) {
	for name, baseURL := range map[string]string{
		"empty":    "",
		"relative": "/v1",
		"scheme":   "file:///tmp/v1",
		"no host":  "http:///v1",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Defaults()
			cfg.Providers = append(cfg.Providers, ProviderSpec{ID: "local", BaseURL: baseURL})
			if err := Validate(cfg); err == nil {
				t.Fatalf("Validate accepted base_url %q", baseURL)
			}
		})
	}
}

func TestLoadRejectsInvalidProviderEntries(t *testing.T) {
	for name, source := range map[string]string{
		"wrong type":  `return { providers = { "local" } }`,
		"field type":  `return { providers = { { id = "local", base_url = 123 } } }`,
		"missing id":  `return { providers = { { base_url = "http://localhost/v1" } } }`,
		"unknown key": `return { providers = { { id = "local", url = "http://localhost/v1" } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.lua")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}
			if _, err := Load(path); err == nil {
				t.Fatal("Load accepted an invalid provider entry")
			}
		})
	}
}
