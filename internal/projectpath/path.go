package projectpath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	projectRoot string
	mu          sync.RWMutex
)

// ProjectRoot is the root directory of the project being managed.
// Deprecated: use GetRoot() instead.
var ProjectRoot string

// Init sets the project root directory. Safe for concurrent use; later calls override.
func Init(root string) {
	mu.Lock()
	projectRoot = root
	ProjectRoot = root
	mu.Unlock()
}

// GetRoot returns the project root directory.
func GetRoot() string {
	mu.RLock()
	defer mu.RUnlock()
	return projectRoot
}

// Resolve validates that a path stays within the project root.
func Resolve(relativePath string) (absolute string, relative string, err error) {
	relativePath = filepath.Clean(relativePath)

	// Block path escapes.
	if strings.Contains(relativePath, "..") {
		return "", "", fmt.Errorf("path escapes project root: %s", relativePath)
	}

	// Block absolute paths outside project.
	if filepath.IsAbs(relativePath) {
		if !strings.HasPrefix(relativePath, GetRoot()) {
			return "", "", fmt.Errorf("absolute path outside project: %s", relativePath)
		}
		absolute = relativePath
		rel, _ := filepath.Rel(GetRoot(), absolute)
		relative = rel
		return
	}

	absolute = filepath.Join(GetRoot(), relativePath)
	relative = relativePath

	// Block symlinks pointing outside project.
	if isSymlinkEscape(absolute) {
		return "", "", fmt.Errorf("symlink escapes project root: %s", relativePath)
	}

	return
}

// ResolveNewWritable validates a path for creating a new file.
func ResolveNewWritable(relativePath string) (absolute string, relative string, err error) {
	return Resolve(relativePath)
}

// IsBlockedPath checks if a write target is a sensitive/generated path.
func IsBlockedPath(relativePath string) bool {
	blocked := []string{
		".env", ".git", "node_modules", "dist", "build", ".next",
		"bun.lock", "package-lock.json", "yarn.lock", "pnpm-lock.yaml",
		"go.sum", "Cargo.lock", "composer.lock",
	}
	lower := strings.ToLower(filepath.ToSlash(relativePath))
	for _, b := range blocked {
		if strings.HasPrefix(lower, b) || strings.Contains(lower, "/"+b) {
			return true
		}
	}
	return false
}

func isSymlinkEscape(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(path)
		if err != nil {
			return true
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(path), target)
		}
		if !strings.HasPrefix(filepath.Clean(target), GetRoot()) {
			return true
		}
	}
	return false
}
