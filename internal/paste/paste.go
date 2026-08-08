package paste

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type Paster struct {
	Command  string
	Delay    time.Duration
	Shortcut string
}

func New(delay time.Duration, shortcut string) Paster {
	if shortcut == "" {
		shortcut = "ctrl_shift_v"
	}
	return Paster{
		Command:  "wtype",
		Delay:    delay,
		Shortcut: shortcut,
	}
}

func (p Paster) Paste(ctx context.Context) error {
	if p.Delay > 0 {
		timer := time.NewTimer(p.Delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}

	command := p.Command
	if command == "" {
		command = "wtype"
	}

	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command)
	}

	args, err := shortcutArgs(p.Shortcut)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s paste failed: %s", command, message)
	}

	return nil
}

func shortcutArgs(shortcut string) ([]string, error) {
	switch shortcut {
	case "", "ctrl_shift_v":
		return []string{"-M", "ctrl", "-M", "shift", "-P", "v", "-p", "v", "-m", "shift", "-m", "ctrl"}, nil
	case "ctrl_v":
		return []string{"-M", "ctrl", "-P", "v", "-p", "v", "-m", "ctrl"}, nil
	case "shift_insert":
		return []string{"-M", "shift", "-P", "Insert", "-p", "Insert", "-m", "shift"}, nil
	default:
		return nil, fmt.Errorf("unsupported paste shortcut %q", shortcut)
	}
}
