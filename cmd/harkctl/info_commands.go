package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"hark/internal/config"
	"hark/internal/dependencies"
	"hark/internal/ipc"
)

func status(ctx context.Context, socketPath string, args []string) error {
	flags := flag.NewFlagSet("status", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write compact JSON")
	requiredProtocol := flags.Int("require-protocol", 0, "require an exact daemon protocol version")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl status [--json] [--require-protocol version]")
	}

	var response ipc.Status
	if err := ipc.Call(ctx, socketPath, "status", nil, &response); err != nil {
		return fmt.Errorf("daemon unavailable: %w", err)
	}
	if *requiredProtocol > 0 && response.ProtocolVersion != *requiredProtocol {
		return commandError{
			err:      fmt.Errorf("daemon protocol version %d is incompatible; expected %d", response.ProtocolVersion, *requiredProtocol),
			exitCode: ipc.IncompatibleProtocolExitCode,
		}
	}
	var (
		encoded []byte
		err     error
	)
	if *jsonOutput {
		encoded, err = json.Marshal(response)
	} else {
		encoded, err = json.MarshalIndent(response, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("format status: %w", err)
	}
	fmt.Println(string(encoded))
	return nil
}

func configCommand(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: harkctl config path")
	}
	if args[0] != "path" {
		return fmt.Errorf("unknown config command %q", args[0])
	}
	fmt.Println(config.DefaultPath())
	return nil
}

func doctorCommand(args []string) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	jsonOutput := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("usage: harkctl doctor [--json]")
	}

	statuses := dependencies.CheckDefault()
	if *jsonOutput {
		encoded, err := json.Marshal(statuses)
		if err != nil {
			return fmt.Errorf("format doctor output: %w", err)
		}
		fmt.Println(string(encoded))
	} else {
		for _, status := range statuses {
			state := "missing"
			location := status.InstallHint
			if status.Found {
				state = "ok"
				location = status.Path
			}
			fmt.Printf("%-8s %-8s %s", status.Name, state, status.Purpose)
			if location != "" {
				fmt.Printf(" (%s)", location)
			}
			fmt.Println()
		}
	}
	if missing := dependencies.MissingSummary(statuses); missing != "" {
		return fmt.Errorf("missing dependencies: %s", missing)
	}
	return nil
}
