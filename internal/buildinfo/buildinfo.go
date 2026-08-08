package buildinfo

import "strings"

var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

func String() string {
	parts := []string{Version}
	if Commit != "" && Commit != "unknown" {
		parts = append(parts, "commit "+Commit)
	}
	if BuildDate != "" && BuildDate != "unknown" {
		parts = append(parts, "built "+BuildDate)
	}
	return strings.Join(parts, ", ")
}
