package shortcut

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	Default           = "SUPER + A"
	DefaultScreenshot = "SUPER + ALT + A"

	IntegrationOmarchy  Integration = "omarchy"
	IntegrationHyprland Integration = "hyprland"

	ActionOpen       Action = "open"
	ActionScreenshot Action = "screenshot"

	blockBegin = "-- BEGIN HARK MANAGED SHORTCUT"
	blockEnd   = "-- END HARK MANAGED SHORTCUT"

	screenshotBlockBegin = "-- BEGIN HARK MANAGED SCREENSHOT SHORTCUT"
	screenshotBlockEnd   = "-- END HARK MANAGED SCREENSHOT SHORTCUT"
)

// Hyprland modifier bitmask values, as reported by `hyprctl binds`.
const (
	modShift = 1
	modCtrl  = 4
	modAlt   = 8
	modSuper = 64
)

type Action string

type Integration string

type Status struct {
	Configured  bool        `json:"configured"`
	Shortcut    string      `json:"shortcut,omitempty"`
	ConfigPath  string      `json:"config_path"`
	Action      Action      `json:"action"`
	Integration Integration `json:"integration"`
}

type Options struct {
	BindingsPath string
	ShellPath    string
	Shortcut     string
	Force        bool
	Action       Action
	Integration  Integration
}

type actionSpec struct {
	action      Action
	blockBegin  string
	blockEnd    string
	comment     string
	description string
	ipcMethod   string
}

type parsedShortcut struct {
	display string
	key     string
	modmask int
}

func DefaultBindingsPathFor(integration Integration) string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			configHome = ".config"
		} else {
			configHome = filepath.Join(home, ".config")
		}
	}
	if normalizeIntegration(integration) == IntegrationHyprland {
		return filepath.Join(configHome, "hark", "hyprland.conf")
	}
	return filepath.Join(configHome, "hypr", "bindings.lua")
}

func DefaultShellPath() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return filepath.Join(".config", "quickshell", "hark", "shell.qml")
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "quickshell", "hark", "shell.qml")
}

func normalizeIntegration(integration Integration) Integration {
	if integration == "" {
		return IntegrationOmarchy
	}
	return integration
}

func validateIntegration(integration Integration) (Integration, error) {
	integration = normalizeIntegration(integration)
	switch integration {
	case IntegrationOmarchy, IntegrationHyprland:
		return integration, nil
	default:
		return "", fmt.Errorf("unsupported shortcut integration %q", integration)
	}
}

func Normalize(value string) (string, error) {
	parsed, err := parse(value)
	if err != nil {
		return "", err
	}
	return parsed.display, nil
}

func GetActionFor(path string, action Action, integration Integration) (Status, error) {
	integration, err := validateIntegration(integration)
	if err != nil {
		return Status{}, err
	}
	if path == "" {
		path = DefaultBindingsPathFor(integration)
	}
	spec, err := specForIntegration(action, integration)
	if err != nil {
		return Status{}, err
	}
	status := Status{ConfigPath: path, Action: spec.action, Integration: integration}
	source, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return status, nil
	}
	if err != nil {
		return Status{}, fmt.Errorf("read Hyprland bindings: %w", err)
	}

	block, found, err := managedActionBlock(source, spec)
	if err != nil {
		return Status{}, err
	}
	if !found {
		return status, nil
	}
	for _, line := range strings.Split(string(block), "\n") {
		prefix := spec.comment + " shortcut: "
		if strings.HasPrefix(line, prefix) {
			normalized, err := Normalize(strings.TrimPrefix(line, prefix))
			if err != nil {
				return Status{}, fmt.Errorf("read managed shortcut: %w", err)
			}
			status.Configured = true
			status.Shortcut = normalized
			return status, nil
		}
	}
	return Status{}, errors.New("managed Hark shortcut block is missing its shortcut metadata")
}

func Set(ctx context.Context, opts Options) (Status, error) {
	integration, err := validateIntegration(opts.Integration)
	if err != nil {
		return Status{}, err
	}
	opts.Integration = integration
	if opts.BindingsPath == "" {
		opts.BindingsPath = DefaultBindingsPathFor(integration)
	}
	if opts.ShellPath == "" {
		opts.ShellPath = DefaultShellPath()
	}
	parsed, err := parse(opts.Shortcut)
	if err != nil {
		return Status{}, err
	}
	spec, err := specForIntegration(opts.Action, integration)
	if err != nil {
		return Status{}, err
	}

	if !opts.Force {
		description, err := liveConflict(ctx, parsed)
		if err != nil {
			return Status{}, err
		}
		if description != "" && !strings.EqualFold(description, spec.description) {
			return Status{}, fmt.Errorf("%s is already bound to %s; choose another shortcut", parsed.display, description)
		}
	}

	original, existed, err := readOptional(opts.BindingsPath)
	if err != nil {
		return Status{}, err
	}
	var replacement []byte
	if integration == IntegrationHyprland {
		replacement = renderHyprlandActionBlock(parsed, opts.ShellPath, opts.Force, spec)
	} else {
		replacement = renderActionBlock(parsed.display, opts.Force, spec)
	}
	updated, err := rewriteAction(original, replacement, spec)
	if err != nil {
		return Status{}, err
	}
	if bytes.Equal(original, updated) {
		return GetActionFor(opts.BindingsPath, spec.action, integration)
	}

	if err := writeBindings(opts.BindingsPath, original, existed, updated); err != nil {
		return Status{}, err
	}
	if err := reloadAndValidate(ctx); err != nil {
		if restoreErr := restore(opts.BindingsPath, original, existed); restoreErr != nil {
			return Status{}, fmt.Errorf("%v; rollback failed: %w", err, restoreErr)
		}
		_ = reload(ctx)
		return Status{}, err
	}
	return GetActionFor(opts.BindingsPath, spec.action, integration)
}

func RemoveActionFor(ctx context.Context, path string, action Action, integration Integration) (Status, error) {
	integration, err := validateIntegration(integration)
	if err != nil {
		return Status{}, err
	}
	if path == "" {
		path = DefaultBindingsPathFor(integration)
	}
	spec, err := specForIntegration(action, integration)
	if err != nil {
		return Status{}, err
	}
	original, existed, err := readOptional(path)
	if err != nil {
		return Status{}, err
	}
	if !existed {
		return Status{ConfigPath: path, Action: spec.action, Integration: integration}, nil
	}
	updated, err := rewriteAction(original, nil, spec)
	if err != nil {
		return Status{}, err
	}
	if bytes.Equal(original, updated) {
		return Status{ConfigPath: path, Action: spec.action, Integration: integration}, nil
	}

	if err := writeBindings(path, original, true, updated); err != nil {
		return Status{}, err
	}
	if err := reloadAndValidate(ctx); err != nil {
		if restoreErr := restore(path, original, true); restoreErr != nil {
			return Status{}, fmt.Errorf("%v; rollback failed: %w", err, restoreErr)
		}
		_ = reload(ctx)
		return Status{}, err
	}
	return Status{ConfigPath: path, Action: spec.action, Integration: integration}, nil
}

func specForIntegration(action Action, integration Integration) (actionSpec, error) {
	comment := "--"
	beginPrefix := "--"
	if normalizeIntegration(integration) == IntegrationHyprland {
		comment = "#"
		beginPrefix = "#"
	}
	switch action {
	case "", ActionOpen:
		return actionSpec{
			action:      ActionOpen,
			blockBegin:  strings.Replace(blockBegin, "--", beginPrefix, 1),
			blockEnd:    strings.Replace(blockEnd, "--", beginPrefix, 1),
			comment:     comment,
			description: "Hark",
			ipcMethod:   "toggle",
		}, nil
	case ActionScreenshot:
		return actionSpec{
			action:      ActionScreenshot,
			blockBegin:  strings.Replace(screenshotBlockBegin, "--", beginPrefix, 1),
			blockEnd:    strings.Replace(screenshotBlockEnd, "--", beginPrefix, 1),
			comment:     comment,
			description: "Hark screenshot",
			ipcMethod:   "captureActiveWindow",
		}, nil
	default:
		return actionSpec{}, fmt.Errorf("unsupported shortcut action %q", action)
	}
}

func parse(value string) (parsedShortcut, error) {
	value = strings.TrimSpace(strings.ToUpper(value))
	if value == "" {
		return parsedShortcut{}, errors.New("shortcut must not be empty")
	}

	// Accept legacy CTRL_SHIFT_A syntax.
	if !strings.Contains(value, "+") {
		value = strings.ReplaceAll(value, "_", "+")
	}
	rawParts := strings.Split(value, "+")
	var (
		super bool
		ctrl  bool
		shift bool
		alt   bool
		key   string
	)
	for _, raw := range rawParts {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		switch part {
		case "SUPER", "META", "WIN", "MOD4":
			super = true
		case "CTRL", "CONTROL":
			ctrl = true
		case "SHIFT":
			shift = true
		case "ALT":
			alt = true
		default:
			if key != "" {
				return parsedShortcut{}, errors.New("shortcut must contain exactly one non-modifier key")
			}
			key = canonicalKey(part)
		}
	}
	if key == "" {
		return parsedShortcut{}, errors.New("shortcut must include a non-modifier key")
	}
	if !super && !ctrl && !alt {
		return parsedShortcut{}, errors.New("shortcut must include Super, Ctrl, or Alt")
	}
	if !validKey(key) {
		return parsedShortcut{}, fmt.Errorf("unsupported shortcut key %q", key)
	}

	parts := make([]string, 0, 5)
	modmask := 0
	if super {
		parts = append(parts, "SUPER")
		modmask |= modSuper
	}
	if ctrl {
		parts = append(parts, "CTRL")
		modmask |= modCtrl
	}
	if shift {
		parts = append(parts, "SHIFT")
		modmask |= modShift
	}
	if alt {
		parts = append(parts, "ALT")
		modmask |= modAlt
	}
	parts = append(parts, key)
	return parsedShortcut{
		display: strings.Join(parts, " + "),
		key:     key,
		modmask: modmask,
	}, nil
}

func canonicalKey(key string) string {
	switch key {
	case "ENTER":
		return "RETURN"
	case "ESC":
		return "ESCAPE"
	case "PAGE_UP":
		return "PAGEUP"
	case "PAGE_DOWN":
		return "PAGEDOWN"
	default:
		return key
	}
}

func validKey(key string) bool {
	if len(key) == 1 {
		return key[0] >= 'A' && key[0] <= 'Z' || key[0] >= '0' && key[0] <= '9'
	}
	if strings.HasPrefix(key, "F") {
		number, err := strconv.Atoi(strings.TrimPrefix(key, "F"))
		return err == nil && number >= 1 && number <= 24
	}
	switch key {
	case "SPACE", "RETURN", "TAB", "BACKSPACE", "DELETE", "INSERT",
		"HOME", "END", "PAGEUP", "PAGEDOWN", "LEFT", "RIGHT", "UP", "DOWN",
		"ESCAPE", "MINUS", "EQUAL", "COMMA", "PERIOD", "SLASH", "SEMICOLON",
		"APOSTROPHE", "BRACKETLEFT", "BRACKETRIGHT", "BACKSLASH", "GRAVE":
		return true
	default:
		return false
	}
}

func renderActionBlock(shortcut string, force bool, spec actionSpec) []byte {
	command := omarchyCommand(spec)
	var builder strings.Builder
	builder.WriteString(spec.blockBegin)
	builder.WriteString("\n")
	builder.WriteString(spec.comment)
	builder.WriteString(" Managed by Hark. Change this shortcut from Hark Settings.\n")
	builder.WriteString(spec.comment)
	builder.WriteString(" shortcut: ")
	builder.WriteString(shortcut)
	builder.WriteByte('\n')
	if force {
		builder.WriteString("hl.unbind(")
		builder.WriteString(strconv.Quote(shortcut))
		builder.WriteString(")\n")
	}
	builder.WriteString("o.bind(")
	builder.WriteString(strconv.Quote(shortcut))
	builder.WriteString(", ")
	builder.WriteString(strconv.Quote(spec.description))
	builder.WriteString(", ")
	builder.WriteString(strconv.Quote(command))
	builder.WriteString(")\n")
	builder.WriteString(spec.blockEnd)
	builder.WriteByte('\n')
	return []byte(builder.String())
}

func renderHyprlandActionBlock(shortcut parsedShortcut, shellPath string, force bool, spec actionSpec) []byte {
	displayParts := strings.Split(shortcut.display, " + ")
	modifiers := strings.Join(displayParts[:len(displayParts)-1], " ")
	command := "qs -p " + shellQuote(filepath.Clean(shellPath)) + " ipc call hark " + spec.ipcMethod

	var builder strings.Builder
	builder.WriteString(spec.blockBegin)
	builder.WriteString("\n# Managed by Hark. Change this shortcut from Hark Settings.\n")
	builder.WriteString("# shortcut: ")
	builder.WriteString(shortcut.display)
	builder.WriteByte('\n')
	if force {
		builder.WriteString("unbind = ")
		builder.WriteString(modifiers)
		builder.WriteString(", ")
		builder.WriteString(shortcut.key)
		builder.WriteByte('\n')
	}
	builder.WriteString("bindd = ")
	builder.WriteString(modifiers)
	builder.WriteString(", ")
	builder.WriteString(shortcut.key)
	builder.WriteString(", ")
	builder.WriteString(spec.description)
	builder.WriteString(", exec, ")
	builder.WriteString(command)
	builder.WriteByte('\n')
	builder.WriteString(spec.blockEnd)
	builder.WriteByte('\n')
	return []byte(builder.String())
}

func omarchyCommand(spec actionSpec) string {
	if spec.action == ActionScreenshot {
		return "omarchy-shell shell summon hark " + shellQuote(`{"action":"captureActiveWindow"}`)
	}
	return "omarchy-shell shell toggle hark"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func rewriteAction(source, replacement []byte, spec actionSpec) ([]byte, error) {
	_, found, err := managedActionBlock(source, spec)
	if err != nil {
		return nil, err
	}
	if !found && len(replacement) == 0 {
		return append([]byte(nil), source...), nil
	}
	if found {
		start := bytes.Index(source, []byte(spec.blockBegin))
		endRelative := bytes.Index(source[start:], []byte(spec.blockEnd))
		end := start + endRelative + len(spec.blockEnd)
		for end < len(source) && (source[end] == '\n' || source[end] == '\r') {
			end++
		}
		source = append(append([]byte(nil), source[:start]...), source[end:]...)
	}

	source = bytes.TrimRight(source, " \t\r\n")
	if len(replacement) == 0 {
		if len(source) == 0 {
			return nil, nil
		}
		return append(source, '\n'), nil
	}
	if len(source) > 0 {
		source = append(source, '\n', '\n')
	}
	return append(source, replacement...), nil
}

func managedActionBlock(source []byte, spec actionSpec) ([]byte, bool, error) {
	start := bytes.Index(source, []byte(spec.blockBegin))
	end := bytes.Index(source, []byte(spec.blockEnd))
	switch {
	case start < 0 && end < 0:
		return nil, false, nil
	case start < 0 || end < 0 || end < start:
		return nil, false, errors.New("incomplete managed Hark shortcut block in Hyprland bindings")
	default:
		end += len(spec.blockEnd)
		return source[start:end], true, nil
	}
}

func readOptional(path string) ([]byte, bool, error) {
	source, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read Hyprland bindings: %w", err)
	}
	return source, true, nil
}

func writeBindings(path string, original []byte, existed bool, updated []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create Hyprland config directory: %w", err)
	}
	if existed {
		backupPath := path + ".hark.bak"
		if _, err := os.Stat(backupPath); errors.Is(err, os.ErrNotExist) {
			if err := os.WriteFile(backupPath, original, 0o600); err != nil {
				return fmt.Errorf("back up Hyprland bindings: %w", err)
			}
		}
	}
	return writeAtomic(path, updated)
}

func restore(path string, original []byte, existed bool) error {
	if !existed {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return writeAtomic(path, original)
}

func writeAtomic(path string, content []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".hark-bindings-*.lua")
	if err != nil {
		return fmt.Errorf("create temporary Hyprland bindings: %w", err)
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)

	if err := temp.Chmod(mode); err != nil {
		return closeTemporaryFile(temp, fmt.Errorf("set temporary Hyprland bindings permissions: %w", err))
	}
	if _, err := temp.Write(content); err != nil {
		return closeTemporaryFile(temp, fmt.Errorf("write temporary Hyprland bindings: %w", err))
	}
	if err := temp.Sync(); err != nil {
		return closeTemporaryFile(temp, fmt.Errorf("sync temporary Hyprland bindings: %w", err))
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temporary Hyprland bindings: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace Hyprland bindings: %w", err)
	}
	return nil
}

func closeTemporaryFile(file *os.File, operationErr error) error {
	if closeErr := file.Close(); closeErr != nil {
		return errors.Join(operationErr, fmt.Errorf("close temporary Hyprland bindings: %w", closeErr))
	}
	return operationErr
}

func liveConflict(ctx context.Context, target parsedShortcut) (string, error) {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return "", nil
	}
	output, err := exec.CommandContext(ctx, "hyprctl", "binds").Output()
	if err != nil {
		return "", fmt.Errorf("inspect Hyprland shortcuts: %w", err)
	}
	return conflictFromBinds(output, target), nil
}

func conflictFromBinds(output []byte, target parsedShortcut) string {
	for _, block := range strings.Split(string(output), "\n\n") {
		var (
			mask        = -1
			key         string
			description string
		)
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(line, "modmask:"):
				mask, _ = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "modmask:")))
			case strings.HasPrefix(line, "key:"):
				key = strings.ToUpper(strings.TrimSpace(strings.TrimPrefix(line, "key:")))
			case strings.HasPrefix(line, "description:"):
				description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			}
		}
		if mask == target.modmask && key == target.key {
			if description == "" {
				return "another Hyprland action"
			}
			return description
		}
	}
	return ""
}

func reloadAndValidate(ctx context.Context) error {
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") == "" {
		return nil
	}
	if err := reload(ctx); err != nil {
		return err
	}
	output, err := exec.CommandContext(ctx, "hyprctl", "configerrors").CombinedOutput()
	if err != nil {
		return fmt.Errorf("validate Hyprland configuration: %s", commandError(err, output))
	}
	if message := strings.TrimSpace(string(output)); message != "" {
		return fmt.Errorf("hyprland rejected the shortcut: %s", message)
	}
	return nil
}

func reload(ctx context.Context) error {
	output, err := exec.CommandContext(ctx, "hyprctl", "reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("reload Hyprland: %s", commandError(err, output))
	}
	return nil
}

func commandError(err error, output []byte) string {
	if message := strings.TrimSpace(string(output)); message != "" {
		return message
	}
	return err.Error()
}
