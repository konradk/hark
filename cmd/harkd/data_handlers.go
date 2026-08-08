package main

import (
	"context"
	"fmt"

	"hark/internal/ipc"
	"hark/internal/settings"
)

func (a *appState) historyList(ctx context.Context, req ipc.Request) (any, error) {
	var request historyListRequest
	if err := decodeParams(req, "history_list", &request); err != nil {
		return nil, err
	}
	if request.Limit < 1 || request.Limit > 200 {
		return nil, fmt.Errorf("history_list limit must be between 1 and 200")
	}
	return a.history.List(ctx, request.Limit)
}

func (a *appState) historyGet(ctx context.Context, req ipc.Request) (any, error) {
	id, err := decodeHistoryID(req, "history_get")
	if err != nil {
		return nil, err
	}
	return a.history.Get(ctx, id)
}

func (a *appState) historyDelete(ctx context.Context, req ipc.Request) (any, error) {
	id, err := decodeHistoryID(req, "history_delete")
	if err != nil {
		return nil, err
	}
	result, err := a.history.DeleteConversation(ctx, id)
	if err != nil {
		return nil, err
	}
	removed, cleanupErr := a.removeManagedScreenshots(result)
	response := map[string]any{"deleted": true, "deleted_entries": result.DeletedEntries, "screenshots_removed": removed}
	if cleanupErr != nil {
		response["warning"] = cleanupErr.Error()
	}
	return response, nil
}

func (a *appState) historyClear(ctx context.Context, req ipc.Request) (any, error) {
	if err := validateNoParams(req, "history_clear"); err != nil {
		return nil, err
	}
	result, err := a.history.Clear(ctx)
	if err != nil {
		return nil, err
	}
	a.clearRuntimeStates()
	removed, cleanupErr := a.cleaner.RemoveAll()
	if cleanupErr != nil && a.logger != nil {
		a.logger.Printf("screenshot cleanup after history clear failed: %v", cleanupErr)
	}
	response := map[string]any{"cleared": true, "deleted_entries": result.DeletedEntries, "screenshots_removed": removed}
	if cleanupErr != nil {
		response["warning"] = cleanupErr.Error()
	}
	return response, nil
}

func decodeHistoryID(req ipc.Request, method string) (int64, error) {
	var request historyIDRequest
	if err := decodeParams(req, method, &request); err != nil {
		return 0, err
	}
	if request.ID <= 0 {
		return 0, fmt.Errorf("%s requires a positive id", method)
	}
	return request.ID, nil
}

func (a *appState) settingsGet(ctx context.Context, req ipc.Request) (any, error) {
	key, err := decodeSettingGetRequest(req)
	if err != nil {
		return nil, err
	}
	value, found, err := a.settingValue(ctx, key)
	if err != nil {
		return nil, err
	}
	return map[string]any{"key": key, "value": value, "found": found}, nil
}

func (a *appState) settingsSet(ctx context.Context, req ipc.Request) (any, error) {
	request, err := decodeSettingSetRequest(req)
	if err != nil {
		return nil, err
	}
	stored, typed, err := settings.Normalize(request.Key, request.Value, a.allowedModels())
	if err != nil {
		return nil, err
	}
	if err := a.history.SetSetting(ctx, request.Key, stored); err != nil {
		return nil, err
	}
	result := map[string]any{"key": request.Key, "value": typed}
	if request.Key == settings.HistoryRetentionDays {
		if err := a.runMaintenance(ctx); err != nil {
			if a.logger != nil {
				a.logger.Printf("maintenance after retention update failed: %v", err)
			}
			result["warning"] = err.Error()
		}
	}
	return result, nil
}
