package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"hark/internal/ipc"
	"hark/internal/settings"
)

type copyTextRequest struct {
	Text string `json:"text"`
}

type pasteTextRequest struct {
	StateID string `json:"state_id"`
	Text    string `json:"text"`
}

type conversationRequest struct {
	ConversationID string `json:"conversation_id"`
}

type stateRequest struct {
	StateID string `json:"state_id"`
}

type historyListRequest struct {
	Limit int `json:"limit"`
}

type historyIDRequest struct {
	ID int64 `json:"id"`
}

type reasoningModesRequest struct {
	Model string `json:"model"`
}

type settingGetRequest struct {
	Key string `json:"key"`
}

type settingSetRequest struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type normalizedSettingRequest struct {
	Key   settings.Key
	Value any
}

func decodeParams(req ipc.Request, method string, target any) error {
	if len(req.Params) == 0 {
		return fmt.Errorf("%s requires params", method)
	}
	decoder := json.NewDecoder(bytes.NewReader(req.Params))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s request: %w", method, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode %s request: multiple JSON values", method)
		}
		return fmt.Errorf("decode %s request: %w", method, err)
	}
	return nil
}

func validateNoParams(req ipc.Request, method string) error {
	if len(bytes.TrimSpace(req.Params)) != 0 && !bytes.Equal(bytes.TrimSpace(req.Params), []byte("null")) {
		return fmt.Errorf("%s does not accept params", method)
	}
	return nil
}

func decodeReasoningModesRequest(req ipc.Request, defaultModel string) (string, error) {
	if len(bytes.TrimSpace(req.Params)) == 0 || bytes.Equal(bytes.TrimSpace(req.Params), []byte("null")) {
		return defaultModel, nil
	}
	var request reasoningModesRequest
	if err := decodeParams(req, "reasoning_modes_list", &request); err != nil {
		return "", err
	}
	if request.Model == "" {
		return defaultModel, nil
	}
	return request.Model, nil
}

func decodeSettingGetRequest(req ipc.Request) (settings.Key, error) {
	var raw settingGetRequest
	if err := decodeParams(req, "settings_get", &raw); err != nil {
		return "", err
	}
	return settings.ParseKey(raw.Key)
}

func decodeSettingSetRequest(req ipc.Request) (normalizedSettingRequest, error) {
	var raw settingSetRequest
	if err := decodeParams(req, "settings_set", &raw); err != nil {
		return normalizedSettingRequest{}, err
	}
	key, err := settings.ParseKey(raw.Key)
	if err != nil {
		return normalizedSettingRequest{}, err
	}
	return normalizedSettingRequest{Key: key, Value: raw.Value}, nil
}

func validateStateID(value, field string) error {
	if value == "" || len(value) > 128 {
		return fmt.Errorf("%s must contain 1 to 128 bytes", field)
	}
	for _, character := range value {
		switch {
		case character >= 'a' && character <= 'z':
		case character >= 'A' && character <= 'Z':
		case character >= '0' && character <= '9':
		case character == '.', character == '_', character == ':', character == '-':
		default:
			return fmt.Errorf("%s contains an unsupported character", field)
		}
	}
	return nil
}

func validateActionText(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("text must not be empty")
	}
	if len(value) > ipc.MaxTextActionBytes {
		return fmt.Errorf("text must be at most %d bytes", ipc.MaxTextActionBytes)
	}
	return nil
}
