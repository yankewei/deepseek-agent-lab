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
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = filepath.Clean(root)
	}
	mu.Lock()
	projectRoot = filepath.Clean(abs)
	ProjectRoot = projectRoot
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
	root := GetRoot()
	if root == "" {
		return "", "", fmt.Errorf("project root is not initialized")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	rootAbs = filepath.Clean(rootAbs)

	var targetAbs string
	if filepath.IsAbs(relativePath) {
		targetAbs = filepath.Clean(relativePath)
	} else {
		targetAbs = filepath.Join(rootAbs, filepath.Clean(relativePath))
	}
	targetAbs, err = filepath.Abs(targetAbs)
	if err != nil {
		return "", "", err
	}
	targetAbs = filepath.Clean(targetAbs)

	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", "", err
	}
	if pathEscapes(rel) {
		return "", "", fmt.Errorf("path escapes project root: %s", relativePath)
	}

	if isSymlinkEscape(rootAbs, targetAbs) {
		return "", "", fmt.Errorf("symlink escapes project root: %s", relativePath)
	}

	return targetAbs, rel, nil
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

func pathEscapes(rel string) bool {
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func isSymlinkEscape(rootAbs, targetAbs string) bool {
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return true
	}
	realRoot = filepath.Clean(realRoot)

	if _, err := os.Lstat(targetAbs); err == nil {
		realTarget, err := filepath.EvalSymlinks(targetAbs)
		if err != nil {
			return true
		}
		return !isWithin(realRoot, filepath.Clean(realTarget))
	}

	parent := filepath.Dir(targetAbs)
	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return false
	}
	realTarget := filepath.Join(realParent, filepath.Base(targetAbs))
	return !isWithin(realRoot, filepath.Clean(realTarget))
}

func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return !pathEscapes(rel)
}
