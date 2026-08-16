package secrets

import (
	"testing"

	keyring "github.com/zalando/go-keyring"
)

func TestProviderAPIKeyUsesSecretService(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OPENAI_API_KEY", "env-key")

	if err := SetProviderAPIKey("openai", "secret-key"); err != nil {
		t.Fatalf("SetProviderAPIKey returned error: %v", err)
	}

	key, source, err := ProviderAPIKey("openai")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "secret-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if source != SourceSecretServer {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestProviderAPIKeyFallsBackToEnv(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OPENAI_API_KEY", "env-key")

	key, source, err := ProviderAPIKey("openai")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if source != SourceEnvironment {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestProviderAPIKeyOpenRouterFallsBackToEnv(t *testing.T) {
	keyring.MockInit()
	t.Setenv("OPENROUTER_API_KEY", "env-key")

	key, source, err := ProviderAPIKey("openrouter")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if source != SourceEnvironment {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestProviderAPIKeyXAIFallsBackToEnv(t *testing.T) {
	keyring.MockInit()
	t.Setenv("XAI_API_KEY", "env-key")

	key, source, err := ProviderAPIKey("xai")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if source != SourceEnvironment {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestDeleteProviderAPIKey(t *testing.T) {
	keyring.MockInit()

	if err := SetProviderAPIKey("openai", "secret-key"); err != nil {
		t.Fatalf("SetProviderAPIKey returned error: %v", err)
	}
	if err := DeleteProviderAPIKey("openai"); err != nil {
		t.Fatalf("DeleteProviderAPIKey returned error: %v", err)
	}

	key, source, err := ProviderAPIKey("openai")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "" || source != SourceNone {
		t.Fatalf("expected no key, got key=%q source=%q", key, source)
	}
}

func TestProviderAPIKeyCustomProviderFallsBackToEnv(t *testing.T) {
	keyring.MockInit()
	t.Setenv("MY_GW_API_KEY", "env-key")

	key, source, err := ProviderAPIKey("my-gw")
	if err != nil {
		t.Fatalf("ProviderAPIKey returned error: %v", err)
	}
	if key != "env-key" {
		t.Fatalf("unexpected key: %q", key)
	}
	if source != SourceEnvironment {
		t.Fatalf("unexpected source: %q", source)
	}
}

func TestNormalizeProviderAcceptsCustomNames(t *testing.T) {
	for _, name := range []string{"my-gw", "local.vllm", "gw_2"} {
		if _, err := normalizeProvider(name); err != nil {
			t.Fatalf("normalizeProvider(%q) returned error: %v", name, err)
		}
	}
}

func TestNormalizeProviderRejectsInvalidNames(t *testing.T) {
	for _, name := range []string{"", "has space", "bad$ymbol", "slash/name"} {
		if _, err := normalizeProvider(name); err == nil {
			t.Fatalf("normalizeProvider(%q) unexpectedly succeeded", name)
		}
	}
}
