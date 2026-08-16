package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"hark/internal/ipc"
)

type providerModelEntry struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type providerEntry struct {
	ID      string               `json:"id"`
	Label   string               `json:"label"`
	BaseURL string               `json:"base_url"`
	Models  []providerModelEntry `json:"models"`
}

func providerCommand(ctx context.Context, socketPath string, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harkctl provider [list | save | remove]")
	}
	switch args[0] {
	case "list":
		return providerList(ctx, socketPath, args[1:])
	case "save":
		return providerSave(ctx, socketPath, args[1:])
	case "remove":
		return providerRemove(ctx, socketPath, args[1:])
	default:
		return fmt.Errorf("unknown provider command %q", args[0])
	}
}

func providerList(ctx context.Context, socketPath string, args []string) error {
	return printIPCList(ctx, socketPath, "provider list", "providers_list", args, &[]providerEntry{})
}

func providerSave(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("provider save", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	id := flags.String("id", "", "provider id (a-z, 0-9, '.', '_' or '-')")
	label := flags.String("label", "", "display label")
	baseURL := flags.String("base-url", "", "absolute http(s) base URL")
	models := multiFlag{}
	flags.Var(&models, "model", "model id; may be repeated")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *id == "" || *baseURL == "" {
		return errors.New("usage: harkctl provider save --json --id ID --label LABEL --base-url URL [--model MODEL_ID ...]")
	}

	params := map[string]any{"id": *id, "label": *label, "base_url": *baseURL, "models": models}
	var response map[string]any
	if err := ipc.Call(ctx, socketPath, "providers_save", params, &response); err != nil {
		return fmt.Errorf("save provider: %w", err)
	}
	return printResultJSON(*compactJSON, response, "provider saved")
}

func providerRemove(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("provider remove", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	id := flags.String("id", "", "provider id to remove")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *id == "" {
		return errors.New("usage: harkctl provider remove --json --id ID")
	}

	var response map[string]any
	if err := ipc.Call(ctx, socketPath, "providers_remove", map[string]string{"id": *id}, &response); err != nil {
		return fmt.Errorf("remove provider: %w", err)
	}
	return printResultJSON(*compactJSON, response, "provider removed")
}

func printResultJSON(compact bool, response any, humanText string) error {
	if compact {
		encoded, err := json.Marshal(response)
		if err != nil {
			return fmt.Errorf("format result: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}
	fmt.Println(humanText)
	return nil
}
