package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hark/internal/ai"
	"hark/internal/ai/openai"
	"hark/internal/ai/openrouter"
	"hark/internal/ai/xai"
	"hark/internal/buildinfo"
	"hark/internal/clipboard"
	"hark/internal/config"
	"hark/internal/history"
	"hark/internal/hyprland"
	"hark/internal/ipc"
	"hark/internal/logging"
	"hark/internal/paste"
	"hark/internal/screenshot"
	"hark/internal/secrets"

	"golang.org/x/sys/unix"
)

func main() {
	unix.Umask(0o077)

	configPath := flag.String("config", "", "path to Lua config file")
	socketPath := flag.String("socket", ipc.DefaultSocketPath(), "path to harkd Unix socket")
	printVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *printVersion {
		fmt.Println(buildinfo.String())
		return
	}

	logger := logging.New()
	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("load config: %v", err)
	}
	if *configPath == "" {
		*configPath = config.DefaultPath()
	}

	historyStore, err := history.Open("")
	if err != nil {
		logger.Fatalf("open history: %v", err)
	}
	defer func() {
		if err := historyStore.Close(); err != nil {
			logger.Printf("close history: %v", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	providers := map[string]ai.Provider{
		openai.ProviderName: openai.NewWithAPIKeyProvider(func() (string, error) {
			key, _, err := secrets.ProviderAPIKey(openai.ProviderName)
			return key, err
		}),
		openrouter.ProviderName: openrouter.NewWithAPIKeyProvider(func() (string, error) {
			key, _, err := secrets.ProviderAPIKey(openrouter.ProviderName)
			return key, err
		}),
		xai.ProviderName: xai.NewWithAPIKeyProvider(func() (string, error) {
			key, _, err := secrets.ProviderAPIKey(xai.ProviderName)
			return key, err
		}),
	}

	app := &appState{
		cfg:           cfg,
		providers:     providers,
		clip:          clipboard.New(),
		paster:        paste.New(time.Duration(cfg.Paste.DelayMS)*time.Millisecond, cfg.Paste.Shortcut),
		hypr:          hyprland.New(),
		capturer:      screenshot.New(),
		cleaner:       screenshot.NewCleaner(),
		history:       historyStore,
		logger:        logger,
		attachmentDir: screenshot.DefaultDir(),
		states:        make(map[string]runtimeState),
	}
	if err := app.runMaintenance(ctx); err != nil {
		logger.Printf("initial maintenance failed: %v", err)
	}
	go app.maintenanceLoop(ctx)

	server := newIPCServer(app, cfg, serverMetadata{SocketPath: *socketPath, ConfigPath: *configPath})
	logger.Printf("starting harkd on %s", *socketPath)
	if err := server.Serve(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Fatalf("ipc server: %v", err)
	}
}
