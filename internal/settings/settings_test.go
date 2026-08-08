package settings

import (
	"testing"
)

func TestNormalizeTypedSettings(t *testing.T) {
	models := []string{"gpt-test"}
	tests := []struct {
		name   string
		key    Key
		value  any
		stored string
	}{
		{name: "model", key: SelectedModel, value: "gpt-test", stored: "gpt-test"},
		{name: "reasoning", key: SelectedReasoningEffort, value: "high", stored: "high"},
		{name: "show recent", key: ShowRecentChats, value: false, stored: "false"},
		{name: "save history", key: SaveHistory, value: true, stored: "true"},
		{name: "retention", key: HistoryRetentionDays, value: float64(30), stored: "30"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stored, _, err := Normalize(test.key, test.value, models)
			if err != nil {
				t.Fatalf("Normalize returned error: %v", err)
			}
			if stored != test.stored {
				t.Fatalf("stored = %q, want %q", stored, test.stored)
			}
		})
	}
}

func TestNormalizeRejectsInvalidSettings(t *testing.T) {
	tests := []struct {
		name  string
		key   Key
		value any
	}{
		{name: "unconfigured model", key: SelectedModel, value: "other"},
		{name: "reasoning", key: SelectedReasoningEffort, value: "ultra"},
		{name: "boolean string", key: SaveHistory, value: "false"},
		{name: "fractional retention", key: HistoryRetentionDays, value: 1.5},
		{name: "negative retention", key: HistoryRetentionDays, value: -1},
		{name: "excessive retention", key: HistoryRetentionDays, value: 3651},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := Normalize(test.key, test.value, []string{"gpt-test"}); err == nil {
				t.Fatal("Normalize unexpectedly accepted invalid value")
			}
		})
	}
}

func TestParseKeyRejectsUnknownSetting(t *testing.T) {
	if _, err := ParseKey("arbitrary"); err == nil {
		t.Fatal("ParseKey unexpectedly accepted unknown setting")
	}
}

func TestStoredValuesRoundTrip(t *testing.T) {
	for _, key := range []Key{SelectedModel, SelectedReasoningEffort, ShowRecentChats, SaveHistory, HistoryRetentionDays} {
		value := DefaultValue(key, "gpt-test", "low")
		stored, typed, err := Normalize(key, value, []string{"gpt-test"})
		if err != nil {
			t.Fatalf("Normalize(%s) returned error: %v", key, err)
		}
		decoded, err := DecodeStored(key, stored, []string{"gpt-test"})
		if err != nil {
			t.Fatalf("DecodeStored(%s) returned error: %v", key, err)
		}
		if decoded != typed {
			t.Fatalf("DecodeStored(%s) = %#v, want %#v", key, decoded, typed)
		}
	}
}

func TestReasoningModesMatchValidation(t *testing.T) {
	modes := ReasoningModes()
	if len(modes) == 0 {
		t.Fatal("ReasoningModes returned no modes")
	}
	for _, mode := range modes {
		if mode.ID == "" || mode.Label == "" || !ValidReasoningEffort(mode.ID) {
			t.Fatalf("invalid reasoning mode: %#v", mode)
		}
	}
	modes[0].ID = "mutated"
	if ReasoningModes()[0].ID == "mutated" {
		t.Fatal("ReasoningModes exposed mutable package state")
	}
}

func TestReasoningModesForGPT56ExcludeMinimal(t *testing.T) {
	modes := ReasoningModesFor("openai", "gpt-5.6-sol")
	if !SupportsReasoningEffort("openai", "gpt-5.6-sol", "max") {
		t.Fatal("GPT-5.6 modes do not include max")
	}
	for _, mode := range modes {
		if mode.ID == "minimal" {
			t.Fatal("GPT-5.6 modes unexpectedly include minimal")
		}
	}
}

func TestOpenRouterReasoningModesMatchClientSupport(t *testing.T) {
	for _, mode := range ReasoningModesFor("openrouter", "vendor/model") {
		switch mode.ID {
		case "auto", "low", "medium", "high":
		default:
			t.Fatalf("unexpected OpenRouter reasoning mode: %q", mode.ID)
		}
	}
}
