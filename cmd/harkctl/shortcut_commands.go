package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"hark/internal/shortcut"
)

func shortcutCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harkctl shortcut [get [--json] [--integration mode] [--action action] | set [--json] [--integration mode] [--action action] [--shell path] shortcut | remove [--json] [--integration mode] [--action action]]")
	}

	switch args[0] {
	case "get":
		flags := flag.NewFlagSet("shortcut get", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		jsonOutput := flags.Bool("json", false, "write compact JSON")
		bindingsPath := flags.String("bindings", "", "shortcut configuration path")
		action := flags.String("action", string(shortcut.ActionOpen), "shortcut action: open or screenshot")
		integration := flags.String("integration", string(shortcut.IntegrationHyprland), "shortcut integration: omarchy or hyprland")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("usage: harkctl shortcut get [--json] [--integration omarchy|hyprland] [--action open|screenshot]")
		}
		status, err := shortcut.GetActionFor(*bindingsPath, shortcut.Action(*action), shortcut.Integration(*integration))
		if err != nil {
			return err
		}
		return printShortcutStatus(status, *jsonOutput)
	case "set":
		flags := flag.NewFlagSet("shortcut set", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		jsonOutput := flags.Bool("json", false, "write compact JSON")
		bindingsPath := flags.String("bindings", "", "shortcut configuration path")
		shellPath := flags.String("shell", shortcut.DefaultShellPath(), "Hark shell.qml path")
		force := flags.Bool("force", false, "replace an existing binding")
		action := flags.String("action", string(shortcut.ActionOpen), "shortcut action: open or screenshot")
		integration := flags.String("integration", string(shortcut.IntegrationHyprland), "shortcut integration: omarchy or hyprland")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 1 {
			return errors.New("usage: harkctl shortcut set [--json] [--integration omarchy|hyprland] [--action open|screenshot] [--shell path] shortcut")
		}
		status, err := shortcut.Set(ctx, shortcut.Options{
			BindingsPath: *bindingsPath,
			ShellPath:    *shellPath,
			Shortcut:     flags.Arg(0),
			Force:        *force,
			Action:       shortcut.Action(*action),
			Integration:  shortcut.Integration(*integration),
		})
		if err != nil {
			return err
		}
		return printShortcutStatus(status, *jsonOutput)
	case "remove":
		flags := flag.NewFlagSet("shortcut remove", flag.ContinueOnError)
		flags.SetOutput(os.Stderr)
		jsonOutput := flags.Bool("json", false, "write compact JSON")
		bindingsPath := flags.String("bindings", "", "shortcut configuration path")
		action := flags.String("action", string(shortcut.ActionOpen), "shortcut action: open or screenshot")
		integration := flags.String("integration", string(shortcut.IntegrationHyprland), "shortcut integration: omarchy or hyprland")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if flags.NArg() != 0 {
			return errors.New("usage: harkctl shortcut remove [--json] [--integration omarchy|hyprland] [--action open|screenshot]")
		}
		status, err := shortcut.RemoveActionFor(ctx, *bindingsPath, shortcut.Action(*action), shortcut.Integration(*integration))
		if err != nil {
			return err
		}
		return printShortcutStatus(status, *jsonOutput)
	default:
		return fmt.Errorf("unknown shortcut command %q", args[0])
	}
}

func printShortcutStatus(status shortcut.Status, jsonOutput bool) error {
	if jsonOutput {
		encoded, err := json.Marshal(status)
		if err != nil {
			return fmt.Errorf("format shortcut status: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}
	if status.Configured {
		fmt.Println(status.Shortcut)
	} else {
		fmt.Println("not configured")
	}
	return nil
}
