package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"hark/internal/ipc"
)

func modelCommand(ctx context.Context, socketPath string, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harkctl model [add | remove]")
	}
	switch args[0] {
	case "add":
		return modelAdd(ctx, socketPath, args[1:])
	case "remove":
		return modelRemove(ctx, socketPath, args[1:])
	default:
		return fmt.Errorf("unknown model command %q", args[0])
	}
}

func modelAdd(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("model add", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	provider := flags.String("provider", "", "provider id the model belongs to")
	id := flags.String("id", "", "model id")
	label := flags.String("label", "", "display label (defaults to the model id)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *provider == "" || *id == "" {
		return errors.New("usage: harkctl model add --json --provider ID --id MODEL_ID [--label LABEL]")
	}

	params := map[string]string{"provider": *provider, "id": *id, "label": *label}
	var response map[string]any
	if err := ipc.Call(ctx, socketPath, "models_add", params, &response); err != nil {
		return fmt.Errorf("add model: %w", err)
	}
	return printResultJSON(*compactJSON, response, "model added")
}

func modelRemove(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("model remove", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	id := flags.String("id", "", "model id to remove")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *id == "" {
		return errors.New("usage: harkctl model remove --json --id MODEL_ID")
	}

	var response map[string]any
	if err := ipc.Call(ctx, socketPath, "models_remove", map[string]string{"id": *id}, &response); err != nil {
		return fmt.Errorf("remove model: %w", err)
	}
	return printResultJSON(*compactJSON, response, "model removed")
}
