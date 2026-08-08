package screenshot

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDirUsesXDGCacheHome(t *testing.T) {
	temp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", temp)

	got := DefaultDir()
	want := filepath.Join(temp, "hark", "screenshots")
	if got != want {
		t.Fatalf("defaultDir() = %q, want %q", got, want)
	}
}

func TestCaptureRegionReportsMissingSlurp(t *testing.T) {
	c := Capturer{
		SlurpCommand: "hark-missing-slurp",
		GrimCommand:  "grim",
		Dir:          t.TempDir(),
	}

	_, err := c.CaptureRegion(testContext(t))
	if err == nil {
		t.Fatal("expected missing slurp error")
	}
	if !strings.Contains(err.Error(), "not installed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCaptureWindowRejectsInvalidSize(t *testing.T) {
	c := Capturer{GrimCommand: "grim", Dir: t.TempDir()}
	if _, err := c.CaptureWindow(testContext(t), 0, 0, 0, 100); err == nil {
		t.Fatal("expected invalid window size error")
	}
}

func testContext(t *testing.T) context.Context {
	t.Helper()
	return context.Background()
}
