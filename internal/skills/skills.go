package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Skill is a local instruction package loaded from skill-name/SKILL.md.
type Skill struct {
	Name          string
	Title         string
	Description   string
	WhenToUse     string
	License       string
	Compatibility string
	AllowedTools  string
	Metadata      map[string]string
	Content       string // lazy-loaded body (frontmatter stripped)
	Path          string
}

// Body returns the markdown body of the skill, loading from disk on first call.
// If Content is already populated (e.g. from a direct test construction), it is returned as-is.
func (s *Skill) Body() (string, error) {
	if s.Content != "" {
		return s.Content, nil
	}
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return "", err
	}
	_, body, err := splitFrontmatter(string(data))
	if err != nil {
		// Treat entire file as body if frontmatter parsing fails.
		s.Content = string(data)
		return s.Content, nil
	}
	s.Content = strings.TrimSpace(body)
	return s.Content, nil
}

// Root points at a directory containing skill-name/SKILL.md entries.
type Root struct {
	Path string
}

// DefaultRoots returns the built-in skill search roots.
func DefaultRoots(projectRoot, homeDir string, extraDirs []string) []Root {
	var roots []Root
	if homeDir != "" {
		roots = append(roots, Root{Path: filepath.Join(homeDir, ".agents", "skills")})
	}
	for _, dir := range extraDirs {
		if dir == "" {
			continue
		}
		roots = append(roots, Root{Path: expandHome(dir, homeDir)})
	}
	if projectRoot != "" {
		roots = append(roots, Root{Path: filepath.Join(projectRoot, ".disco", "skills")})
	}
	return roots
}

// Load scans roots in order and loads only the frontmatter metadata from each
// SKILL.md. The full markdown body is loaded lazily via Skill.Body() when the
// skill is activated. Later roots override earlier roots with the same skill
// directory name.
func Load(roots []Root) ([]Skill, error) {
	byName := make(map[string]Skill)
	for _, root := range roots {
		entries, err := os.ReadDir(root.Path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirName := entry.Name()
			path := filepath.Join(root.Path, dirName, "SKILL.md")
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			skill, err := parse(dirName, path, string(data))
			if err != nil {
				continue
			}
			byName[skill.Name] = skill
		}
	}

	out := make([]Skill, 0, len(byName))
	for _, skill := range byName {
		out = append(out, skill)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out, nil
}

// Match returns skills whose metadata matches the user text.
func Match(all []Skill, text string) []Skill {
	queryTokens := tokenSet(text)
	query := strings.ToLower(text)
	if len(queryTokens) == 0 && strings.TrimSpace(query) == "" {
		return nil
	}
	var matches []Skill
	for _, skill := range all {
		if matchesSkill(skill, query, queryTokens) {
			matches = append(matches, skill)
		}
	}
	return matches
}

// Inject appends the active skill instructions to a base system prompt.
func Inject(base string, active []Skill) string {
	if len(active) == 0 {
		return base
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(base, "\n"))
	b.WriteString("\n\n## Active Skills\n")
	for _, skill := range active {
		title := skill.Title
		if title == "" {
			title = skill.Name
		}
		body, err := skill.Body()
		if err != nil {
			continue
		}
		b.WriteString(fmt.Sprintf("\n### %s\n", title))
		b.WriteString(fmt.Sprintf("Skill directory: %s\n\n", filepath.Dir(skill.Path)))
		b.WriteString(strings.TrimSpace(body))
		b.WriteString("\n")
	}
	return b.String()
}

type rawFrontmatter struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	AllowedTools  string            `yaml:"allowed-tools"`
	Metadata      map[string]string `yaml:"metadata"`
	WhenToUse     string            `yaml:"when_to_use"`
}

func parse(dirName, path, content string) (Skill, error) {
	fmStr, body, err := splitFrontmatter(content)

	var fm rawFrontmatter
	if err == nil {
		if uerr := yaml.Unmarshal([]byte(fmStr), &fm); uerr != nil {
			// If YAML unmarshal fails, fall through with zero-valued fm.
			fm = rawFrontmatter{}
		}
	}

	name := fm.Name
	if name == "" {
		name = dirName
	}
	if verr := ValidateName(name, dirName); verr != nil {
		return Skill{}, verr
	}

	title := extractTitle(body)
	description := fm.Description
	if description == "" {
		description = extractFirstParagraph(body)
	}

	return Skill{
		Name:          name,
		Title:         title,
		Description:   description,
		WhenToUse:     fm.WhenToUse,
		License:       fm.License,
		Compatibility: fm.Compatibility,
		AllowedTools:  fm.AllowedTools,
		Metadata:      fm.Metadata,
		Path:          path,
	}, nil
}

// ValidateName checks that a skill name conforms to the agentskills.io spec.
func ValidateName(name, dirName string) error {
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if name != dirName {
		return fmt.Errorf("name %q must match directory name %q", name, dirName)
	}
	if len(name) < 1 || len(name) > 64 {
		return fmt.Errorf("name %q must be 1-64 characters", name)
	}
	for i, r := range name {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !valid {
			return fmt.Errorf("name %q must only contain lowercase letters, digits, and hyphens", name)
		}
		if r == '-' {
			if i == 0 || i == len(name)-1 {
				return fmt.Errorf("name %q must not start or end with a hyphen", name)
			}
			if i > 0 && name[i-1] == '-' {
				return fmt.Errorf("name %q must not contain consecutive hyphens", name)
			}
		}
	}
	return nil
}

func splitFrontmatter(content string) (string, string, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return "", content, fmt.Errorf("no frontmatter")
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			fm := strings.Join(lines[1:i], "\n")
			body := strings.Join(lines[i+1:], "\n")
			return fm, body, nil
		}
	}
	return "", content, fmt.Errorf("unclosed frontmatter")
}

func extractTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

func extractFirstParagraph(body string) string {
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		return trimmed
	}
	return ""
}

func matchesSkill(skill Skill, query string, queryTokens map[string]bool) bool {
	searchText := skill.Name + " " + skill.Title + " " + skill.Description + " " + skill.WhenToUse
	for token := range tokenSet(searchText) {
		if queryTokens[token] {
			return true
		}
	}
	for _, phrase := range splitPhrases(skill.WhenToUse) {
		if phrase == "" {
			continue
		}
		if strings.Contains(query, phrase) || strings.Contains(phrase, query) {
			return true
		}
	}
	return false
}

func tokenSet(text string) map[string]bool {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	})
	out := make(map[string]bool)
	for _, field := range fields {
		if len([]rune(field)) < 2 {
			continue
		}
		out[field] = true
	}
	return out
}

func expandHome(path, homeDir string) string {
	if homeDir == "" {
		return path
	}
	if path == "~" {
		return homeDir
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func splitPhrases(text string) []string {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return r == ',' || r == ';' || r == '\n'
	})
	for i, field := range fields {
		fields[i] = strings.TrimSpace(field)
	}
	return fields
}
