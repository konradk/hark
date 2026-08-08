package hyprland

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	Command string
}

type Window struct {
	Address   string    `json:"address"`
	Class     string    `json:"class"`
	Title     string    `json:"title"`
	At        [2]int    `json:"at"`
	Size      [2]int    `json:"size"`
	Workspace Workspace `json:"workspace"`
	PID       int       `json:"pid"`
	Mapped    bool      `json:"mapped"`
	Hidden    bool      `json:"hidden"`
}

type Workspace struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func New() Client {
	return Client{Command: "hyprctl"}
}

func (c Client) ActiveWindow(ctx context.Context) (Window, error) {
	command := c.command()
	if _, err := exec.LookPath(command); err != nil {
		return Window{}, fmt.Errorf("%s is not installed or not in PATH", command)
	}

	cmd := exec.CommandContext(ctx, command, "activewindow", "-j")
	output, err := cmd.Output()
	if err != nil {
		return Window{}, fmt.Errorf("%s activewindow failed: %w", command, err)
	}

	return ParseWindow(output)
}

func ParseWindow(data []byte) (Window, error) {
	var win Window
	if err := json.Unmarshal(data, &win); err != nil {
		return Window{}, fmt.Errorf("decode Hyprland active window: %w", err)
	}
	if win.Address == "" || win.Address == "0x0" {
		return Window{}, fmt.Errorf("hyprland active window has no usable address")
	}
	return win, nil
}

func (c Client) FocusWindow(ctx context.Context, win Window) error {
	if win.Address == "" || win.Address == "0x0" {
		return fmt.Errorf("cannot focus window without address")
	}

	command := c.command()
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%s is not installed or not in PATH", command)
	}

	target := "address:" + win.Address
	cmd := exec.CommandContext(ctx, command, "dispatch", "focuswindow", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("%s focuswindow %s failed: %s", command, target, message)
	}
	return nil
}

func (c Client) command() string {
	if c.Command == "" {
		return "hyprctl"
	}
	return c.Command
}
