package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"hark/internal/buildinfo"
	"hark/internal/ipc"
)

func main() {
	socketPath := flag.String("socket", ipc.DefaultSocketPath(), "path to the harkd Unix socket")
	timeout := flag.Duration("timeout", 2*time.Minute, "command timeout")
	printVersion := flag.Bool("version", false, "print version information")
	flag.Parse()

	if *printVersion {
		fmt.Println(buildinfo.String())
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	var err error
	switch args[0] {
	case "status":
		err = status(ctx, *socketPath, args[1:])
	case "version":
		fmt.Println(buildinfo.String())
	case "ask":
		err = ask(ctx, *socketPath, args[1:])
	case "models":
		err = models(ctx, *socketPath, args[1:])
	case "reasoning-modes":
		err = reasoningModes(ctx, *socketPath, args[1:])
	case "theme":
		err = theme(ctx, *socketPath, args[1:])
	case "copy-latest":
		err = copyLatest(ctx, *socketPath, args[1:])
	case "copy-text":
		err = copyText(ctx, *socketPath, args[1:])
	case "paste-text":
		err = pasteText(ctx, *socketPath, args[1:])
	case "paste-latest":
		err = pasteLatest(ctx, *socketPath, args[1:])
	case "remember-active-window":
		err = rememberActiveWindow(ctx, *socketPath, args[1:])
	case "active-window":
		err = activeWindow(ctx, *socketPath)
	case "screenshot-region":
		err = screenshotRegion(ctx, *socketPath, args[1:])
	case "screenshot-active-window":
		err = screenshotActiveWindow(ctx, *socketPath, args[1:])
	case "history":
		err = historyCommand(ctx, *socketPath, args[1:])
	case "setting":
		err = settingCommand(ctx, *socketPath, args[1:])
	case "provider":
		err = providerCommand(ctx, *socketPath, args[1:])
	case "model":
		err = modelCommand(ctx, *socketPath, args[1:])
	case "shortcut":
		err = shortcutCommand(ctx, args[1:])
	case "secret":
		err = secretCommand(args[1:])
	case "config":
		err = configCommand(args[1:])
	case "doctor":
		err = doctorCommand(args[1:])
	case "help", "-h", "--help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", args[0])
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "harkctl:", err)
		exitCode := 1
		var coded interface{ ExitCode() int }
		if errors.As(err, &coded) {
			exitCode = coded.ExitCode()
		}
		os.Exit(exitCode)
	}
}

type commandError struct {
	err      error
	exitCode int
}

func (e commandError) Error() string {
	return e.err.Error()
}

func (e commandError) Unwrap() error {
	return e.err
}

func (e commandError) ExitCode() int {
	return e.exitCode
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage:
  harkctl [flags] status [--json] [--require-protocol VERSION]
  harkctl [flags] version
  harkctl [flags] ask [--model ID] [--reasoning-effort MODE] [--image PATH] [--conversation-id ID] --stdin
  harkctl [flags] models [--json]
  harkctl [flags] reasoning-modes [--json] [--model ID]
  harkctl [flags] theme [--json]
  harkctl [flags] copy-latest --conversation-id ID
  harkctl [flags] copy-text --stdin
  harkctl [flags] paste-text --state-id ID --stdin
  harkctl [flags] paste-latest --conversation-id ID
  harkctl [flags] remember-active-window --state-id ID
  harkctl [flags] active-window
  harkctl [flags] screenshot-region [--json]
  harkctl [flags] screenshot-active-window [--json]
  harkctl [flags] history list [--limit N]
  harkctl [flags] history get ID
  harkctl [flags] history delete ID
  harkctl [flags] history clear --yes
  harkctl [flags] setting get KEY
  harkctl [flags] setting set KEY VALUE
  harkctl [flags] provider list [--json]
  harkctl [flags] provider add [--json] --id ID --label LABEL --base-url URL
  harkctl [flags] provider remove [--json] --id ID
  harkctl [flags] provider fetch-models [--json] [--provider ID | --base-url URL]
  harkctl [flags] model add [--json] --provider ID --id MODEL_ID [--label LABEL]
  harkctl [flags] model remove [--json] --id MODEL_ID
  harkctl [flags] shortcut get [--action open|screenshot]
  harkctl [flags] shortcut set [--action open|screenshot] SHORTCUT
  harkctl [flags] shortcut remove [--action open|screenshot]
  harkctl [flags] secret status PROVIDER
  harkctl [flags] secret set [--stdin] PROVIDER
  harkctl [flags] secret delete PROVIDER
  harkctl [flags] config path
  harkctl [flags] doctor [--json]

Flags:
  -socket PATH       path to the harkd Unix socket
  -timeout DURATION  command timeout
  -version           print version information`)
}
