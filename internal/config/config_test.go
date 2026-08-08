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
      { id = "gpt-test", label = "Test", provider = "openrouter" },
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
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "mystery-model", Provider: "mystery"})
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted an unknown model provider")
	}
}

func TestValidateAcceptsModelWithoutProvider(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.Models = append(cfg.Provider.Models, ModelConfig{ID: "legacy-model"})
	if err := Validate(cfg); err != nil {
		t.Fatalf("Validate rejected a model with no provider: %v", err)
	}
}

func TestValidateRejectsReasoningUnsupportedByDefaultModel(t *testing.T) {
	cfg := Defaults()
	cfg.Provider.DefaultReasoningEffort = "minimal"
	if err := Validate(cfg); err == nil {
		t.Fatal("Validate accepted minimal reasoning for GPT-5.6")
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
		"empty":  `return { provider = { models = {} } }`,
		"sparse": `return { provider = { models = { [2] = "gpt-test" } } }`,
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
