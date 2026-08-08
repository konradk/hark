package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"hark/internal/secrets"

	"golang.org/x/term"
)

const maxSecretBytes = 16 << 10

func secretCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: harkctl secret [status provider | set [--stdin] provider | delete provider]")
	}
	switch args[0] {
	case "status":
		return secretStatus(args[1:])
	case "set":
		return secretSet(args[1:])
	case "delete":
		return secretDelete(args[1:])
	default:
		return fmt.Errorf("unknown secret command %q", args[0])
	}
}

func secretStatus(args []string) error {
	flags := flag.NewFlagSet("secret status", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	compactJSON := flags.Bool("json", false, "write compact JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: harkctl secret status [--json] provider")
	}
	status, err := secrets.ProviderAPIKeyStatus(flags.Arg(0))
	if err != nil {
		return err
	}
	if *compactJSON {
		encoded, err := json.Marshal(status)
		if err != nil {
			return fmt.Errorf("format secret status: %w", err)
		}
		fmt.Println(string(encoded))
		return nil
	}
	state := "not configured"
	if status.Configured {
		state = "configured via " + string(status.Source)
	}
	fmt.Printf("%s API key: %s\n", status.Provider, state)
	return nil
}

func secretSet(args []string) error {
	flags := flag.NewFlagSet("secret set", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	fromStdin := flags.Bool("stdin", false, "read API key from stdin")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 1 {
		return errors.New("usage: harkctl secret set [--stdin] provider")
	}
	apiKey, err := readSecretValue(*fromStdin)
	if err != nil {
		return err
	}
	if err := secrets.SetProviderAPIKey(flags.Arg(0), apiKey); err != nil {
		return err
	}
	fmt.Printf("%s API key stored in Secret Service\n", strings.ToLower(flags.Arg(0)))
	return nil
}

func secretDelete(args []string) error {
	if len(args) != 1 {
		return errors.New("usage: harkctl secret delete provider")
	}
	if err := secrets.DeleteProviderAPIKey(args[0]); err != nil {
		return err
	}
	fmt.Printf("%s API key deleted from Secret Service\n", strings.ToLower(args[0]))
	return nil
}

func readSecretValue(fromStdin bool) (string, error) {
	if fromStdin || !term.IsTerminal(int(os.Stdin.Fd())) {
		data, err := io.ReadAll(io.LimitReader(os.Stdin, maxSecretBytes+1))
		if err != nil {
			return "", fmt.Errorf("read API key from stdin: %w", err)
		}
		return validateSecretValue(data)
	}
	fmt.Fprint(os.Stderr, "API key: ")
	data, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read API key: %w", err)
	}
	return validateSecretValue(data)
}

func validateSecretValue(data []byte) (string, error) {
	if len(data) > maxSecretBytes {
		return "", fmt.Errorf("API key must be at most %d bytes", maxSecretBytes)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", errors.New("API key must not be empty")
	}
	return value, nil
}
