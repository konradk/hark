package clipboard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCopyRejectsEmptyText(t *testing.T) {
	clip := Clipboard{Command: "wl-copy"}
	if err := clip.Copy(context.Background(), " \n\t "); err == nil {
		t.Fatal("expected empty text error")
	}
}

func TestCopyReturnsCommandFailure(t *testing.T) {
	command := filepath.Join(t.TempDir(), "wl-copy")
	if err := os.WriteFile(command, []byte("#!/bin/sh\necho clipboard unavailable >&2\nexit 23\n"), 0o700); err != nil {
		t.Fatalf("write fake wl-copy: %v", err)
	}

	err := (Clipboard{Command: command}).Copy(context.Background(), "new clipboard text")
	if err == nil || !strings.Contains(err.Error(), "clipboard unavailable") {
		t.Fatalf("Copy error = %v, want command failure", err)
	}
}
