package shortcut

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := map[string]string{
		"SUPER_A":                 "SUPER + A",
		"super + shift + space":   "SUPER + SHIFT + SPACE",
		"alt+ctrl+f12":            "CTRL + ALT + F12",
		"meta + control + return": "SUPER + CTRL + RETURN",
	}
	for input, want := range tests {
		got, err := Normalize(input)
		if err != nil {
			t.Fatalf("Normalize(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("Normalize(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeRejectsInvalidShortcuts(t *testing.T) {
	for _, input := range []string{"", "A", "SHIFT + A", "SUPER", "SUPER + A + B", "SUPER + MEDIA_PLAY"} {
		if _, err := Normalize(input); err == nil {
			t.Fatalf("Normalize(%q) unexpectedly succeeded", input)
		}
	}
}

func TestRewritePreservesUserBindings(t *testing.T) {
	original := []byte("-- user binding\no.bind(\"SUPER + B\", \"Browser\", \"browser\")\n")
	first, err := rewrite(original, renderBlock("SUPER + A", false))
	if err != nil {
		t.Fatalf("rewrite first: %v", err)
	}
	if !bytes.Contains(first, original[:len(original)-1]) {
		t.Fatalf("user content was not preserved:\n%s", first)
	}
	if !bytes.Contains(first, []byte(`o.bind("SUPER + A", "Hark"`)) {
		t.Fatalf("managed binding missing:\n%s", first)
	}
	if !bytes.Contains(first, []byte(`"omarchy-shell shell toggle hark"`)) {
		t.Fatalf("Omarchy command is missing:\n%s", first)
	}

	second, err := rewrite(first, renderBlock("SUPER + CTRL + A", false))
	if err != nil {
		t.Fatalf("rewrite second: %v", err)
	}
	if bytes.Count(second, []byte(blockBegin)) != 1 {
		t.Fatalf("managed block was duplicated:\n%s", second)
	}
	if bytes.Contains(second, []byte("-- shortcut: SUPER + A\n")) {
		t.Fatalf("old shortcut remains:\n%s", second)
	}
}

func TestGetAndRemoveManagedShortcut(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bindings.lua")
	source, err := rewrite([]byte("-- mine\n"), renderBlock("SUPER + A", false))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := GetActionFor(path, ActionOpen, IntegrationOmarchy)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !status.Configured || status.Shortcut != "SUPER + A" {
		t.Fatalf("unexpected status: %#v", status)
	}

	without, err := rewrite(source, nil)
	if err != nil {
		t.Fatalf("remove rewrite: %v", err)
	}
	if strings.Contains(string(without), "HARK MANAGED") {
		t.Fatalf("managed block remains:\n%s", without)
	}
	if string(without) != "-- mine\n" {
		t.Fatalf("user content changed: %q", without)
	}
}

func renderBlock(shortcut string, force bool) []byte {
	spec, err := specForIntegration(ActionOpen, IntegrationOmarchy)
	if err != nil {
		panic(err)
	}
	return renderActionBlock(shortcut, force, spec)
}

func rewrite(source, replacement []byte) ([]byte, error) {
	spec, err := specForIntegration(ActionOpen, IntegrationOmarchy)
	if err != nil {
		return nil, err
	}
	return rewriteAction(source, replacement, spec)
}

func TestScreenshotShortcutCoexistsWithOpenShortcut(t *testing.T) {
	openBlock := renderBlock("SUPER + A", false)
	source, err := rewrite([]byte("-- mine\n"), openBlock)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := specForIntegration(ActionScreenshot, IntegrationOmarchy)
	if err != nil {
		t.Fatal(err)
	}
	source, err = rewriteAction(source, renderActionBlock("SUPER + ALT + A", false, spec), spec)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(source, []byte("o.bind(")) != 2 {
		t.Fatalf("expected two managed bindings:\n%s", source)
	}
	if !bytes.Contains(source, []byte("captureActiveWindow")) {
		t.Fatalf("screenshot IPC action missing:\n%s", source)
	}

	path := filepath.Join(t.TempDir(), "bindings.lua")
	if err := os.WriteFile(path, source, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := GetActionFor(path, ActionScreenshot, IntegrationOmarchy)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || status.Shortcut != "SUPER + ALT + A" || status.Action != ActionScreenshot {
		t.Fatalf("unexpected screenshot shortcut status: %#v", status)
	}

	withoutScreenshot, err := rewriteAction(source, nil, spec)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(withoutScreenshot, []byte(`o.bind("SUPER + A", "Hark"`)) {
		t.Fatalf("open shortcut was removed:\n%s", withoutScreenshot)
	}
	if bytes.Contains(withoutScreenshot, []byte("captureActiveWindow")) {
		t.Fatalf("screenshot shortcut remains:\n%s", withoutScreenshot)
	}
}

func TestHyprlandShortcutUsesStandaloneShell(t *testing.T) {
	parsed, err := parse("SUPER + ALT + A")
	if err != nil {
		t.Fatal(err)
	}
	spec, err := specForIntegration(ActionScreenshot, IntegrationHyprland)
	if err != nil {
		t.Fatal(err)
	}
	block := renderHyprlandActionBlock(parsed, "/home/me/Hark shell/shell.qml", true, spec)
	for _, expected := range []string{
		"# BEGIN HARK MANAGED SCREENSHOT SHORTCUT",
		"# shortcut: SUPER + ALT + A",
		"unbind = SUPER ALT, A",
		"bindd = SUPER ALT, A, Hark screenshot, exec,",
		"qs -p '/home/me/Hark shell/shell.qml' ipc call hark captureActiveWindow",
	} {
		if !bytes.Contains(block, []byte(expected)) {
			t.Fatalf("Hyprland block does not contain %q:\n%s", expected, block)
		}
	}

	path := filepath.Join(t.TempDir(), "hyprland.conf")
	if err := os.WriteFile(path, block, 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := GetActionFor(path, ActionScreenshot, IntegrationHyprland)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Configured || status.Shortcut != "SUPER + ALT + A" || status.Integration != IntegrationHyprland {
		t.Fatalf("unexpected Hyprland shortcut status: %#v", status)
	}
}

func TestOmarchyScreenshotUsesShellSummonPayload(t *testing.T) {
	spec, err := specForIntegration(ActionScreenshot, IntegrationOmarchy)
	if err != nil {
		t.Fatal(err)
	}
	block := renderActionBlock("SUPER + ALT + A", false, spec)
	if !bytes.Contains(block, []byte(`omarchy-shell shell summon hark '{\"action\":\"captureActiveWindow\"}'`)) {
		t.Fatalf("Omarchy screenshot payload is missing:\n%s", block)
	}
}

func TestDefaultBindingsPathForIntegration(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if got := DefaultBindingsPathFor(IntegrationOmarchy); got != filepath.Join(configHome, "hypr", "bindings.lua") {
		t.Fatalf("Omarchy bindings path = %q", got)
	}
	if got := DefaultBindingsPathFor(IntegrationHyprland); got != filepath.Join(configHome, "hark", "hyprland.conf") {
		t.Fatalf("Hyprland bindings path = %q", got)
	}
}

func TestRejectsUnknownShortcutIntegration(t *testing.T) {
	if _, err := GetActionFor("", ActionOpen, Integration("unknown")); err == nil {
		t.Fatal("unknown integration unexpectedly succeeded")
	}
}

func TestRemoveWithoutManagedBlockDoesNotTouchFile(t *testing.T) {
	source := []byte("-- mine\n\n")
	updated, err := rewrite(source, nil)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	if !bytes.Equal(updated, source) {
		t.Fatalf("rewrite changed unmanaged file: %q", updated)
	}
}

func TestConflictFromBinds(t *testing.T) {
	target, err := parse("SUPER + SPACE")
	if err != nil {
		t.Fatal(err)
	}
	output := []byte(`bindd
	modmask: 64
	submap:
	key: SPACE
	keycode: 0
	description: Omarchy menu
	dispatcher: __lua
	arg: 1

bindd
	modmask: 65
	submap:
	key: SPACE
	keycode: 0
	description: Toggle top bar
	dispatcher: __lua
	arg: 2
`)
	if got := conflictFromBinds(output, target); got != "Omarchy menu" {
		t.Fatalf("conflictFromBinds = %q", got)
	}
}
