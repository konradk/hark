package clipboard

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

type Clipboard struct {
	Command string
}

func New() Clipboard {
	return Clipboard{Command: "wl-copy"}
}

func (c Clipboard) Copy(ctx context.Context, text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("clipboard text must not be empty")
	}

	command := c.Command
	if command == "" {
		command = "wl-copy"
	}

	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command)
	}

	cmd := exec.CommandContext(ctx, command)
	cmd.Stdin = strings.NewReader(text)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message == "" {
		message = err.Error()
	}
	return fmt.Errorf("%s failed: %s", command, message)
}
