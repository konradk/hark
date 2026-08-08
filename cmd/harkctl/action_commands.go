package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"hark/internal/ipc"
	"hark/internal/screenshot"

	"golang.org/x/term"
)

func copyLatest(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("copy-latest", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	conversationID := flags.String("conversation-id", "", "conversation identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *conversationID == "" {
		return errors.New("usage: harkctl copy-latest --conversation-id id")
	}
	if err := ipc.Call(ctx, socketPath, "copy_latest", map[string]string{"conversation_id": *conversationID}, nil); err != nil {
		return fmt.Errorf("copy latest: %w", err)
	}
	return nil
}

func copyText(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("copy-text", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	fromStdin := flags.Bool("stdin", false, "read text from stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	text, err := readTextInput(*fromStdin, flags.Args())
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("usage: harkctl copy-text --stdin")
	}
	if err := ipc.Call(ctx, socketPath, "copy_text", map[string]string{"text": text}, nil); err != nil {
		return fmt.Errorf("copy text: %w", err)
	}
	return nil
}

func pasteText(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("paste-text", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	fromStdin := flags.Bool("stdin", false, "read text from stdin")
	stateID := flags.String("state-id", "", "client state identifier used for focus restore")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *stateID == "" {
		return errors.New("usage: harkctl paste-text --state-id id --stdin")
	}
	text, err := readTextInput(*fromStdin, flags.Args())
	if err != nil {
		return err
	}
	if text == "" {
		return errors.New("usage: harkctl paste-text --state-id id --stdin")
	}
	if err := ipc.Call(ctx, socketPath, "paste_text", map[string]any{"state_id": *stateID, "text": text}, nil); err != nil {
		return fmt.Errorf("paste text: %w", err)
	}
	return nil
}

func pasteLatest(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("paste-latest", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	conversationID := flags.String("conversation-id", "", "conversation identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *conversationID == "" {
		return errors.New("usage: harkctl paste-latest --conversation-id id")
	}
	if err := ipc.Call(ctx, socketPath, "paste_latest", map[string]string{"conversation_id": *conversationID}, nil); err != nil {
		return fmt.Errorf("paste latest: %w", err)
	}
	return nil
}

func rememberActiveWindow(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("remember-active-window", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	stateID := flags.String("state-id", "", "client state identifier")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *stateID == "" {
		return errors.New("usage: harkctl remember-active-window --state-id id")
	}
	var window any
	if err := ipc.Call(ctx, socketPath, "remember_active_window", map[string]string{"state_id": *stateID}, &window); err != nil {
		return fmt.Errorf("remember active window: %w", err)
	}
	return nil
}

func readTextInput(fromStdin bool, args []string) (string, error) {
	if len(args) != 0 {
		return "", errors.New("text must not be passed in process arguments; use --stdin")
	}
	if !fromStdin && term.IsTerminal(int(os.Stdin.Fd())) {
		return "", errors.New("text must be read from stdin; use --stdin")
	}
	data, err := io.ReadAll(io.LimitReader(os.Stdin, ipc.MaxTextActionBytes+2))
	if err != nil {
		return "", fmt.Errorf("read text from stdin: %w", err)
	}
	text := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if len(text) > ipc.MaxTextActionBytes {
		return "", fmt.Errorf("text from stdin exceeds %d bytes", ipc.MaxTextActionBytes)
	}
	return text, nil
}

func activeWindow(ctx context.Context, socketPath string) error {
	var window any
	if err := ipc.Call(ctx, socketPath, "active_window", nil, &window); err != nil {
		return fmt.Errorf("active window: %w", err)
	}
	encoded, err := json.MarshalIndent(window, "", "  ")
	if err != nil {
		return fmt.Errorf("format active window: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func screenshotRegion(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("screenshot-region", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write JSON attachment metadata")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl screenshot-region [--json]")
	}
	var attachment screenshot.Attachment
	if err := ipc.Call(ctx, socketPath, "screenshot_region", nil, &attachment); err != nil {
		return fmt.Errorf("screenshot region: %w", err)
	}
	return printAttachment(attachment, *jsonOutput)
}

func screenshotActiveWindow(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("screenshot-active-window", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl screenshot-active-window [--json]")
	}
	var attachment screenshot.Attachment
	if err := ipc.Call(ctx, socketPath, "screenshot_active_window", nil, &attachment); err != nil {
		return fmt.Errorf("screenshot active window: %w", err)
	}
	return printAttachment(attachment, *jsonOutput)
}

func printAttachment(attachment screenshot.Attachment, jsonOutput bool) error {
	if !jsonOutput {
		fmt.Println(attachment.Path)
		return nil
	}
	encoded, err := json.Marshal(attachment)
	if err != nil {
		return fmt.Errorf("format screenshot attachment: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}
