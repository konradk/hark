package dependencies

import (
	"os/exec"
	"strings"
)

type Tool struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose"`
	InstallHint string `json:"install_hint,omitempty"`
}

type Status struct {
	Name        string `json:"name"`
	Found       bool   `json:"found"`
	Path        string `json:"path,omitempty"`
	Purpose     string `json:"purpose"`
	InstallHint string `json:"install_hint,omitempty"`
}

var DefaultTools = []Tool{
	{Name: "hyprctl", Purpose: "active window lookup and focus restore", InstallHint: "installed with Hyprland"},
	{Name: "wl-copy", Purpose: "copy responses to the Wayland clipboard", InstallHint: "omarchy pkg add wl-clipboard"},
	{Name: "grim", Purpose: "capture screenshots", InstallHint: "omarchy pkg add grim"},
	{Name: "slurp", Purpose: "select screenshot regions", InstallHint: "omarchy pkg add slurp"},
	{Name: "wtype", Purpose: "paste responses back into the focused app", InstallHint: "omarchy pkg add wtype"},
}

func CheckDefault() []Status {
	return Check(DefaultTools)
}

func Check(tools []Tool) []Status {
	return checkWithLookPath(tools, exec.LookPath)
}

func MissingNames(statuses []Status) []string {
	var missing []string
	for _, status := range statuses {
		if !status.Found {
			missing = append(missing, status.Name)
		}
	}
	return missing
}

func MissingSummary(statuses []Status) string {
	return strings.Join(MissingNames(statuses), ", ")
}

func checkWithLookPath(tools []Tool, lookPath func(string) (string, error)) []Status {
	statuses := make([]Status, 0, len(tools))
	for _, tool := range tools {
		path, err := lookPath(tool.Name)
		statuses = append(statuses, Status{
			Name:        tool.Name,
			Found:       err == nil,
			Path:        path,
			Purpose:     tool.Purpose,
			InstallHint: tool.InstallHint,
		})
	}
	return statuses
}
