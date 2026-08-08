package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"hark/internal/ipc"
	"hark/internal/settings"
)

func historyCommand(ctx context.Context, socketPath string, args []string) error {
	if len(args) == 0 {
		return historyList(ctx, socketPath, nil)
	}
	switch args[0] {
	case "list":
		return historyList(ctx, socketPath, args[1:])
	case "get":
		return historyGet(ctx, socketPath, args[1:])
	case "delete":
		return historyDelete(ctx, socketPath, args[1:])
	case "clear":
		return historyClear(ctx, socketPath, args[1:])
	default:
		return fmt.Errorf("unknown history command %q", args[0])
	}
}

func historyList(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("history list", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	limit := flags.Int("limit", 20, "maximum entries to show")
	compactJSON := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl history list [--limit n] [--json]")
	}
	var entries []any
	if err := ipc.Call(ctx, socketPath, "history_list", map[string]int{"limit": *limit}, &entries); err != nil {
		return fmt.Errorf("history list: %w", err)
	}
	return printJSON(entries, *compactJSON, "history list")
}

func historyGet(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("history get", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	id, err := parseIDArg(flags.Args(), "usage: harkctl history get [--json] id")
	if err != nil {
		return err
	}
	var entry any
	if err := ipc.Call(ctx, socketPath, "history_get", map[string]int64{"id": id}, &entry); err != nil {
		return fmt.Errorf("history get: %w", err)
	}
	return printJSON(entry, *compactJSON, "history entry")
}

func historyDelete(ctx context.Context, socketPath string, args []string) error {
	id, err := parseIDArg(args, "usage: harkctl history delete id")
	if err != nil {
		return err
	}
	var result struct {
		Warning string `json:"warning"`
	}
	if err := ipc.Call(ctx, socketPath, "history_delete", map[string]int64{"id": id}, &result); err != nil {
		return fmt.Errorf("history delete: %w", err)
	}
	if result.Warning != "" {
		fmt.Fprintf(os.Stderr, "harkctl: history deleted, but screenshot cleanup failed: %s\n", result.Warning)
	}
	return nil
}

func historyClear(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("history clear", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	confirmed := flags.Bool("yes", false, "confirm deletion of all history")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || !*confirmed {
		return errors.New("usage: harkctl history clear --yes")
	}
	var result struct {
		DeletedEntries     int64  `json:"deleted_entries"`
		ScreenshotsRemoved int    `json:"screenshots_removed"`
		Warning            string `json:"warning"`
	}
	if err := ipc.Call(ctx, socketPath, "history_clear", nil, &result); err != nil {
		return fmt.Errorf("history clear: %w", err)
	}
	fmt.Printf("Cleared %d history entries and removed %d screenshots.\n", result.DeletedEntries, result.ScreenshotsRemoved)
	if result.Warning != "" {
		fmt.Fprintf(os.Stderr, "harkctl: history cleared, but screenshot cleanup failed: %s\n", result.Warning)
	}
	return nil
}

func parseIDArg(args []string, usage string) (int64, error) {
	if len(args) != 1 {
		return 0, errors.New(usage)
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse id: %w", err)
	}
	if id <= 0 {
		return 0, errors.New("id must be positive")
	}
	return id, nil
}

func settingCommand(ctx context.Context, socketPath string, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harkctl setting [get key | set key value]")
	}
	switch args[0] {
	case "get":
		if len(args) != 2 {
			return errors.New("usage: harkctl setting get key")
		}
		if _, err := settings.ParseKey(args[1]); err != nil {
			return err
		}
		var setting any
		if err := ipc.Call(ctx, socketPath, "settings_get", map[string]string{"key": args[1]}, &setting); err != nil {
			return fmt.Errorf("setting get: %w", err)
		}
		return printJSON(setting, true, "setting")
	case "set":
		if len(args) != 3 {
			return errors.New("usage: harkctl setting set key value")
		}
		key, err := settings.ParseKey(args[1])
		if err != nil {
			return err
		}
		value, err := settings.ParseCLIValue(key, args[2])
		if err != nil {
			return err
		}
		var result struct {
			Warning string `json:"warning"`
		}
		if err := ipc.Call(ctx, socketPath, "settings_set", map[string]any{"key": key, "value": value}, &result); err != nil {
			return fmt.Errorf("setting set: %w", err)
		}
		if result.Warning != "" {
			fmt.Fprintf(os.Stderr, "harkctl: setting saved, but maintenance failed: %s\n", result.Warning)
		}
		return nil
	default:
		return fmt.Errorf("unknown setting command %q", args[0])
	}
}

func printJSON(value any, compact bool, description string) error {
	var (
		encoded []byte
		err     error
	)
	if compact {
		encoded, err = json.Marshal(value)
	} else {
		encoded, err = json.MarshalIndent(value, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("format %s: %w", description, err)
	}
	fmt.Println(string(encoded))
	return nil
}
