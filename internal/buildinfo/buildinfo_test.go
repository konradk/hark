package buildinfo

import (
	"strings"
	"testing"
)

func TestStringIncludesConfiguredMetadata(t *testing.T) {
	previousVersion, previousCommit, previousBuildDate := Version, Commit, BuildDate
	t.Cleanup(func() {
		Version, Commit, BuildDate = previousVersion, previousCommit, previousBuildDate
	})
	Version, Commit, BuildDate = "v1.2.3", "abc123", "2026-07-27T20:00:00Z"

	value := String()
	for _, expected := range []string{"v1.2.3", "abc123", "2026-07-27T20:00:00Z"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("String() = %q, want %q", value, expected)
		}
	}
}
