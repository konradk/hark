package main

import (
	"context"
	"fmt"
	"os"

	"hark/internal/buildinfo"
	"hark/internal/ipc"
	"hark/internal/settings"
)

type serverMetadata struct {
	SocketPath string
	ConfigPath string
}

func newIPCServer(app *appState, metadata serverMetadata) ipc.Server {
	return ipc.Server{
		SocketPath: metadata.SocketPath,
		Handler: func(ctx context.Context, req ipc.Request) (any, error) {
			switch req.Method {
			case "status":
				if err := validateNoParams(req, "status"); err != nil {
					return nil, err
				}
				cfg := app.snapshotConfig()
				defaultProvider, _ := configuredProviderForModel(cfg, cfg.Provider.DefaultModel)
				return ipc.Status{
					Name:            "hark",
					Version:         buildinfo.Version,
					ProtocolVersion: ipc.ProtocolVersion,
					PID:             os.Getpid(),
					SocketPath:      metadata.SocketPath,
					ConfigPath:      metadata.ConfigPath,
					Provider:        defaultProvider,
					Model:           cfg.Provider.DefaultModel,
				}, nil
			case "models_list":
				if err := validateNoParams(req, "models_list"); err != nil {
					return nil, err
				}
				return app.snapshotConfig().Provider.Models, nil
			case "theme_get":
				if err := validateNoParams(req, "theme_get"); err != nil {
					return nil, err
				}
				cfg := app.snapshotConfig()
				return ipc.Theme{Name: cfg.UI.Theme, Colors: cfg.UI.Colors}, nil
			case "reasoning_modes_list":
				cfg := app.snapshotConfig()
				model, err := decodeReasoningModesRequest(req, cfg.Provider.DefaultModel)
				if err != nil {
					return nil, err
				}
				configured, ok := configuredModel(cfg, model)
				if !ok {
					return nil, fmt.Errorf("model %q is not configured", model)
				}
				return settings.ReasoningModesFor(configured.ReasoningEfforts), nil
			case "ask":
				return nil, ipc.ErrUseStream
			case "providers_list":
				return app.providersList(ctx, req)
			case "providers_add":
				return app.providersAdd(ctx, req)
			case "providers_remove":
				return app.providersRemove(ctx, req)
			case "copy_latest":
				return app.copyLatestRequest(ctx, req)
			case "copy_text":
				return app.copyText(ctx, req)
			case "paste_text":
				return app.pasteText(ctx, req)
			case "paste_latest":
				return app.pasteLatest(ctx, req)
			case "remember_active_window":
				return app.rememberActiveWindow(ctx, req)
			case "active_window":
				return app.activeWindow(ctx, req)
			case "screenshot_region":
				return app.screenshotRegion(ctx, req)
			case "screenshot_active_window":
				return app.screenshotActiveWindow(ctx, req)
			case "history_list":
				return app.historyList(ctx, req)
			case "history_get":
				return app.historyGet(ctx, req)
			case "history_delete":
				return app.historyDelete(ctx, req)
			case "history_clear":
				return app.historyClear(ctx, req)
			case "settings_get":
				return app.settingsGet(ctx, req)
			case "settings_set":
				return app.settingsSet(ctx, req)
			default:
				return nil, fmt.Errorf("unknown method %q", req.Method)
			}
		},
		StreamHandler: func(ctx context.Context, req ipc.Request, send func(any) error) error {
			if req.Method == "ask" {
				return app.ask(ctx, req, send)
			}
			return fmt.Errorf("unknown streaming method %q", req.Method)
		},
	}
}
