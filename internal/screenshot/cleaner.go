package screenshot

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const UnreferencedGracePeriod = 24 * time.Hour

type Cleaner struct {
	Dir string
}

func NewCleaner() Cleaner {
	return Cleaner{Dir: DefaultDir()}
}

func (c Cleaner) RemoveManaged(paths []string) (int, error) {
	var removed int
	var failures []error
	for _, path := range uniquePaths(paths) {
		if !c.isManagedPath(path) {
			continue
		}
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("inspect screenshot %q: %w", path, err))
			continue
		}
		if info.IsDir() {
			failures = append(failures, fmt.Errorf("refusing to remove screenshot directory %q", path))
			continue
		}
		if err := os.Remove(path); err != nil {
			failures = append(failures, fmt.Errorf("remove screenshot %q: %w", path, err))
			continue
		}
		removed++
	}
	return removed, errors.Join(failures...)
}

func (c Cleaner) RemoveAll() (int, error) {
	directory := c.directory()
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read screenshot directory: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if c.isManagedPath(path) {
			paths = append(paths, path)
		}
	}
	return c.RemoveManaged(paths)
}

func (c Cleaner) RemoveUnreferenced(referenced []string, olderThan time.Time) (int, error) {
	directory := c.directory()
	entries, err := os.ReadDir(directory)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read screenshot directory: %w", err)
	}

	references := make(map[string]struct{}, len(referenced))
	for _, path := range referenced {
		if absolute, err := filepath.Abs(path); err == nil {
			references[filepath.Clean(absolute)] = struct{}{}
		}
	}

	var candidates []string
	var failures []error
	for _, entry := range entries {
		path := filepath.Join(directory, entry.Name())
		if !c.isManagedPath(path) {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			failures = append(failures, fmt.Errorf("resolve screenshot %q: %w", path, err))
			continue
		}
		if _, ok := references[filepath.Clean(absolute)]; ok {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			failures = append(failures, fmt.Errorf("inspect screenshot %q: %w", path, err))
			continue
		}
		if info.ModTime().After(olderThan) {
			continue
		}
		candidates = append(candidates, path)
	}
	removed, removeErr := c.RemoveManaged(candidates)
	if removeErr != nil {
		failures = append(failures, removeErr)
	}
	return removed, errors.Join(failures...)
}

func (c Cleaner) isManagedPath(path string) bool {
	return ManagedPath(c.directory(), path)
}

// ManagedPath reports whether path names a screenshot Hark itself created
// directly inside directory. It is the only path shape accepted as an
// attachment, so a request cannot make the daemon read an unrelated file.
func ManagedPath(directory, path string) bool {
	if directory == "" {
		directory = DefaultDir()
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	root, err := filepath.Abs(directory)
	if err != nil || filepath.Clean(filepath.Dir(absolute)) != filepath.Clean(root) {
		return false
	}
	name := filepath.Base(absolute)
	return strings.HasSuffix(name, ".png") &&
		(strings.HasPrefix(name, "region-") || strings.HasPrefix(name, "window-"))
}

func (c Cleaner) directory() string {
	if c.Dir != "" {
		return c.Dir
	}
	return DefaultDir()
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		path = filepath.Clean(path)
		if path == "." || path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		result = append(result, path)
	}
	return result
}
