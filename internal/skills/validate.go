package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ValidateSkill checks a skill directory against the agentskills.io spec.
// It returns an error with a detailed message if any rule is violated.
func ValidateSkill(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("cannot access skill path: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("skill path must be a directory: %s", path)
	}

	skillFile := filepath.Join(path, "SKILL.md")
	data, err := os.ReadFile(skillFile)
	if err != nil {
		return fmt.Errorf("SKILL.md not found or unreadable in %s: %w", path, err)
	}

	content := string(data)
	fmStr, body, err := splitFrontmatter(content)
	if err != nil {
		return fmt.Errorf("SKILL.md frontmatter error: %w", err)
	}

	var fm rawFrontmatter
	if err := yaml.Unmarshal([]byte(fmStr), &fm); err != nil {
		return fmt.Errorf("SKILL.md frontmatter YAML error: %w", err)
	}

	dirName := filepath.Base(path)

	// Required fields
	if fm.Name == "" {
		return fmt.Errorf("required field 'name' is missing in frontmatter")
	}
	if fm.Description == "" {
		return fmt.Errorf("required field 'description' is missing in frontmatter")
	}

	// Name validation
	if err := ValidateName(fm.Name, dirName); err != nil {
		return fmt.Errorf("invalid 'name' field: %w", err)
	}

	// Description constraints
	if len(fm.Description) > 1024 {
		return fmt.Errorf("'description' exceeds 1024 characters (got %d)", len(fm.Description))
	}

	// Compatibility constraint
	if fm.Compatibility != "" && len(fm.Compatibility) > 500 {
		return fmt.Errorf("'compatibility' exceeds 500 characters (got %d)", len(fm.Compatibility))
	}

	// Body checks
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("SKILL.md has no body content after frontmatter")
	}

	title := extractTitle(body)
	if title == "" {
		return fmt.Errorf("SKILL.md body has no title (no '# ' heading found)")
	}

	return nil
}
