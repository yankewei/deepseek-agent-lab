package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Skill is a local instruction package loaded from skill-name/SKILL.md.
type Skill struct {
	Name        string
	Title       string
	Description string
	WhenToUse   string
	Content     string
	Path        string
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

// Load scans roots in order. Later roots override earlier roots with the same
// skill directory name, so project-local skills can override user skills.
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
			name := entry.Name()
			path := filepath.Join(root.Path, name, "SKILL.md")
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			skill := parse(name, path, string(content))
			byName[name] = skill
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
		b.WriteString(fmt.Sprintf("\n### %s\n", title))
		b.WriteString(fmt.Sprintf("Skill directory: %s\n\n", filepath.Dir(skill.Path)))
		b.WriteString(strings.TrimSpace(skill.Content))
		b.WriteString("\n")
	}
	return b.String()
}

func parse(name, path, content string) Skill {
	title := ""
	description := ""
	lines := strings.Split(content, "\n")
	frontmatter := parseFrontmatter(lines)
	if v := frontmatter["description"]; v != "" {
		description = v
	}
	whenToUse := frontmatter["when_to_use"]

	titleIndex := -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			titleIndex = i
			break
		}
	}
	start := 0
	if titleIndex >= 0 {
		start = titleIndex + 1
	}
	if description == "" {
		for _, line := range lines[start:] {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				continue
			}
			description = trimmed
			break
		}
	}
	return Skill{
		Name:        name,
		Title:       title,
		Description: description,
		WhenToUse:   whenToUse,
		Content:     content,
		Path:        path,
	}
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

func parseFrontmatter(lines []string) map[string]string {
	out := make(map[string]string)
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return out
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		out[key] = value
	}
	return out
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
