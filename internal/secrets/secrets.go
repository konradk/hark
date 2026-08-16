package secrets

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	keyring "github.com/zalando/go-keyring"
)

const serviceName = "hark"

type Source string

const (
	SourceNone         Source = "none"
	SourceSecretServer Source = "secret-service"
	SourceEnvironment  Source = "environment"
)

type Status struct {
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	Source     Source `json:"source"`
}

func ProviderAPIKey(provider string) (string, Source, error) {
	provider, err := normalizeProvider(provider)
	if err != nil {
		return "", SourceNone, err
	}

	value, err := keyring.Get(serviceName, keyName(provider))
	if err == nil {
		return value, SourceSecretServer, nil
	}
	if !errors.Is(err, keyring.ErrNotFound) {
		if env := envAPIKey(provider); env != "" {
			return env, SourceEnvironment, nil
		}
		return "", SourceNone, fmt.Errorf("read %s API key from Secret Service: %w", provider, err)
	}

	if env := envAPIKey(provider); env != "" {
		return env, SourceEnvironment, nil
	}
	return "", SourceNone, nil
}

func ProviderAPIKeyStatus(provider string) (Status, error) {
	key, source, err := ProviderAPIKey(provider)
	if err != nil {
		return Status{}, err
	}
	normalized, err := normalizeProvider(provider)
	if err != nil {
		return Status{}, err
	}
	return Status{
		Provider:   normalized,
		Configured: key != "",
		Source:     source,
	}, nil
}

func SetProviderAPIKey(provider, apiKey string) error {
	provider, err := normalizeProvider(provider)
	if err != nil {
		return err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return errors.New("API key must not be empty")
	}
	if err := keyring.Set(serviceName, keyName(provider), apiKey); err != nil {
		return fmt.Errorf("store %s API key in Secret Service: %w", provider, err)
	}
	return nil
}

func DeleteProviderAPIKey(provider string) error {
	provider, err := normalizeProvider(provider)
	if err != nil {
		return err
	}
	if err := keyring.Delete(serviceName, keyName(provider)); err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("delete %s API key from Secret Service: %w", provider, err)
	}
	return nil
}

// providerNamePattern bounds provider names to a safe keyring-key shape. It
// allows the built-in providers plus user-defined OpenAI-compatible providers.
var providerNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)

// ValidProviderName reports whether name may be used as a provider id.
func ValidProviderName(name string) bool {
	return providerNamePattern.MatchString(name)
}

func normalizeProvider(provider string) (string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		return "", errors.New("provider must not be empty")
	}
	if !providerNamePattern.MatchString(provider) {
		return "", fmt.Errorf("provider %q must match %q", provider, providerNamePattern.String())
	}
	return provider, nil
}

func keyName(provider string) string {
	return provider + "_api_key"
}

func envAPIKey(provider string) string {
	switch provider {
	case "openai":
		return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	case "openrouter":
		return strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	case "xai":
		return strings.TrimSpace(os.Getenv("XAI_API_KEY"))
	default:
		name := strings.ToUpper(strings.NewReplacer("-", "_", ".", "_").Replace(provider)) + "_API_KEY"
		return strings.TrimSpace(os.Getenv(name))
	}
}
