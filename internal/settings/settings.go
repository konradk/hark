package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

type Key string

const (
	SelectedModel           Key = "selected_model"
	SelectedReasoningEffort Key = "selected_reasoning_effort"
	ShowRecentChats         Key = "show_recent_chats"
	SaveHistory             Key = "save_history"
	HistoryRetentionDays    Key = "history_retention_days"
)

var knownKeys = map[Key]struct{}{
	SelectedModel:           {},
	SelectedReasoningEffort: {},
	ShowRecentChats:         {},
	SaveHistory:             {},
	HistoryRetentionDays:    {},
}

type ReasoningMode struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var reasoningModes = []ReasoningMode{
	{ID: "auto", Label: "Auto"},
	{ID: "none", Label: "None"},
	{ID: "minimal", Label: "Minimal"},
	{ID: "low", Label: "Low"},
	{ID: "medium", Label: "Medium"},
	{ID: "high", Label: "High"},
	{ID: "xhigh", Label: "XHigh"},
	{ID: "max", Label: "Max"},
}

func ReasoningModes() []ReasoningMode {
	return cloneReasoningModes(reasoningModes)
}

func ReasoningModesFor(efforts []string) []ReasoningMode {
	modesByID := make(map[string]ReasoningMode, len(reasoningModes))
	for _, mode := range reasoningModes {
		modesByID[mode.ID] = mode
	}
	modes := make([]ReasoningMode, 0, len(efforts))
	for _, effort := range efforts {
		if mode, ok := modesByID[effort]; ok {
			modes = append(modes, mode)
		}
	}
	return modes
}

func SupportsReasoningEffort(efforts []string, effort string) bool {
	for _, supported := range efforts {
		if supported == effort {
			return true
		}
	}
	return false
}

func cloneReasoningModes(source []ReasoningMode) []ReasoningMode {
	result := make([]ReasoningMode, len(source))
	copy(result, source)
	return result
}

func ParseKey(value string) (Key, error) {
	key := Key(strings.TrimSpace(value))
	if _, ok := knownKeys[key]; !ok {
		return "", fmt.Errorf("unknown setting %q", value)
	}
	return key, nil
}

func DefaultValue(key Key, defaultModel, defaultReasoning string) any {
	switch key {
	case SelectedModel:
		return defaultModel
	case SelectedReasoningEffort:
		return defaultReasoning
	case ShowRecentChats, SaveHistory:
		return true
	case HistoryRetentionDays:
		return 0
	default:
		return nil
	}
}

func Normalize(key Key, value any, allowedModels []string) (string, any, error) {
	if _, ok := knownKeys[key]; !ok {
		return "", nil, fmt.Errorf("unknown setting %q", key)
	}

	switch key {
	case SelectedModel:
		model, ok := value.(string)
		if !ok {
			return "", nil, errors.New("selected_model must be a string")
		}
		model = strings.TrimSpace(model)
		if model == "" || len(model) > 128 {
			return "", nil, errors.New("selected_model must contain 1 to 128 bytes")
		}
		if len(allowedModels) > 0 && !contains(allowedModels, model) {
			return "", nil, fmt.Errorf("selected_model %q is not configured", model)
		}
		return model, model, nil

	case SelectedReasoningEffort:
		effort, ok := value.(string)
		if !ok {
			return "", nil, errors.New("selected_reasoning_effort must be a string")
		}
		effort = strings.TrimSpace(effort)
		if !ValidReasoningEffort(effort) {
			return "", nil, errors.New("selected_reasoning_effort must be one of auto, none, minimal, low, medium, high, xhigh, max")
		}
		return effort, effort, nil

	case ShowRecentChats, SaveHistory:
		enabled, ok := value.(bool)
		if !ok {
			return "", nil, fmt.Errorf("%s must be a boolean", key)
		}
		return strconv.FormatBool(enabled), enabled, nil

	case HistoryRetentionDays:
		days, err := integerValue(value)
		if err != nil {
			return "", nil, fmt.Errorf("history_retention_days: %w", err)
		}
		if days < 0 || days > 3650 {
			return "", nil, errors.New("history_retention_days must be between 0 and 3650")
		}
		return strconv.Itoa(days), days, nil
	}
	return "", nil, fmt.Errorf("unsupported setting %q", key)
}

func DecodeStored(key Key, value string, allowedModels []string) (any, error) {
	switch key {
	case SelectedModel, SelectedReasoningEffort:
		_, typed, err := Normalize(key, value, allowedModels)
		return typed, err
	case ShowRecentChats, SaveHistory:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", key, err)
		}
		_, typed, err := Normalize(key, parsed, allowedModels)
		return typed, err
	case HistoryRetentionDays:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("decode history_retention_days: %w", err)
		}
		_, typed, err := Normalize(key, parsed, allowedModels)
		return typed, err
	default:
		return nil, fmt.Errorf("unknown setting %q", key)
	}
}

func ParseCLIValue(key Key, value string) (any, error) {
	switch key {
	case SelectedModel, SelectedReasoningEffort:
		return value, nil
	case ShowRecentChats, SaveHistory:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("%s must be true or false", key)
		}
		return parsed, nil
	case HistoryRetentionDays:
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return nil, errors.New("history_retention_days must be an integer")
		}
		return parsed, nil
	default:
		return nil, fmt.Errorf("unknown setting %q", key)
	}
}

func ValidReasoningEffort(value string) bool {
	for _, mode := range reasoningModes {
		if mode.ID == value {
			return true
		}
	}
	return false
}

func integerValue(value any) (int, error) {
	switch number := value.(type) {
	case int:
		return number, nil
	case int64:
		if number < math.MinInt || number > math.MaxInt {
			return 0, errors.New("must fit in an integer")
		}
		return int(number), nil
	case float64:
		if math.Trunc(number) != number || number < math.MinInt || number > math.MaxInt {
			return 0, errors.New("must be an integer")
		}
		return int(number), nil
	case json.Number:
		parsed, err := strconv.Atoi(string(number))
		if err != nil {
			return 0, errors.New("must be an integer")
		}
		return parsed, nil
	default:
		return 0, errors.New("must be an integer")
	}
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
