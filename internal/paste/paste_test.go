package paste

import (
	"context"
	"strings"
	"testing"
)

func TestPasteReportsMissingCommand(t *testing.T) {
	p := Paster{Command: "hark-missing-wtype"}
	err := p.Paste(context.Background())
	if err == nil {
		t.Fatal("expected missing command error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShortcutArgs(t *testing.T) {
	tests := map[string][]string{
		"ctrl_shift_v": {"-M", "ctrl", "-M", "shift", "-P", "v", "-p", "v", "-m", "shift", "-m", "ctrl"},
		"ctrl_v":       {"-M", "ctrl", "-P", "v", "-p", "v", "-m", "ctrl"},
		"shift_insert": {"-M", "shift", "-P", "Insert", "-p", "Insert", "-m", "shift"},
	}

	for shortcut, want := range tests {
		got, err := shortcutArgs(shortcut)
		if err != nil {
			t.Fatalf("shortcutArgs(%q) returned error: %v", shortcut, err)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Fatalf("shortcutArgs(%q) = %#v, want %#v", shortcut, got, want)
		}
	}
}
