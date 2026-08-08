package screenshot

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Capturer struct {
	GrimCommand  string
	SlurpCommand string
	Dir          string
}

type Attachment struct {
	Type     string `json:"type"`
	Path     string `json:"path"`
	MIMEType string `json:"mime_type"`
}

func New() Capturer {
	return Capturer{
		GrimCommand:  "grim",
		SlurpCommand: "slurp",
		Dir:          DefaultDir(),
	}
}

func DefaultDir() string {
	if cacheHome := os.Getenv("XDG_CACHE_HOME"); cacheHome != "" {
		return filepath.Join(cacheHome, "hark", "screenshots")
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "hark", "screenshots")
	}

	return filepath.Join(home, ".cache", "hark", "screenshots")
}

func (c Capturer) CaptureRegion(ctx context.Context) (Attachment, error) {
	slurp := c.command(c.SlurpCommand, "slurp")

	if _, err := exec.LookPath(slurp); err != nil {
		return Attachment{}, fmt.Errorf("%s is not installed or not in PATH", slurp)
	}

	geometryBytes, err := exec.CommandContext(ctx, slurp).Output()
	if err != nil {
		return Attachment{}, fmt.Errorf("%s selection failed: %w", slurp, err)
	}

	geometry := strings.TrimSpace(string(geometryBytes))
	if geometry == "" {
		return Attachment{}, fmt.Errorf("screenshot selection was empty")
	}
	return c.captureGeometry(ctx, geometry, "region")
}

func (c Capturer) CaptureWindow(ctx context.Context, x, y, width, height int) (Attachment, error) {
	if width <= 0 || height <= 0 {
		return Attachment{}, fmt.Errorf("active window has invalid size %dx%d", width, height)
	}
	geometry := fmt.Sprintf("%d,%d %dx%d", x, y, width, height)
	return c.captureGeometry(ctx, geometry, "window")
}

func (c Capturer) captureGeometry(ctx context.Context, geometry, prefix string) (Attachment, error) {
	grim := c.command(c.GrimCommand, "grim")
	if _, err := exec.LookPath(grim); err != nil {
		return Attachment{}, fmt.Errorf("%s is not installed or not in PATH", grim)
	}
	dir := c.Dir
	if dir == "" {
		dir = DefaultDir()
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Attachment{}, fmt.Errorf("create screenshot directory: %w", err)
	}
	if filepath.Clean(dir) == filepath.Clean(DefaultDir()) {
		if err := os.Chmod(dir, 0o700); err != nil {
			return Attachment{}, fmt.Errorf("set screenshot directory permissions: %w", err)
		}
	}

	path := filepath.Join(dir, prefix+"-"+time.Now().Format("20060102-150405.000000000")+".png")
	cmd := exec.CommandContext(ctx, grim, "-g", geometry, "-t", "png", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return Attachment{}, fmt.Errorf("%s capture failed: %s", grim, message)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = os.Remove(path)
		return Attachment{}, fmt.Errorf("set screenshot permissions: %w", err)
	}

	return Attachment{
		Type:     "image",
		Path:     path,
		MIMEType: "image/png",
	}, nil
}

func (c Capturer) command(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
