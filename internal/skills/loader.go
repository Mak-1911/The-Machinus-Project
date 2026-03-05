package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NewLoader creates a new skills loader
func NewLoader(basePath string) *Loader {
	return &Loader{
		skills:    make(map[string]*Skill),
		paths: []string{
			filepath.Join(basePath, "skills", "core"),
			filepath.Join(basePath, "skills", "community"),
			filepath.Join(basePath, "skills", "experimental"),
		},
		indexPath: make(map[string][]*Skill),
	}
}

// NewLoaderWithPaths creates a loader with custom paths
func NewLoaderWithPaths(paths []string) *Loader {
	return &Loader{
		skills:    make(map[string]*Skill),
		paths:     paths,
		indexPath: make(map[string][]*Skill),
	}
}

// LoadAll discovers and loads all skills from configured paths
func (l *Loader) LoadAll() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for _, path := range l.paths {
		if err := l.loadFromPath(path); err != nil {
			// Log but continue loading other paths
			fmt.Printf("Warning: failed to load skills from %s: %v\n", path, err)
		}
	}

	// Build keyword index for fast matching
	l.buildKeywordIndex()

	fmt.Printf("Loaded %d skills from %d paths\n", len(l.skills), len(l.paths))
	return nil
}

// loadFromPath loads all skills from a directory path
func (l *Loader) loadFromPath(path string) error {
	// Determine category from path
	category := l.getCategoryFromPath(path)

	// Check if directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Skip non-existent paths
	}

	// Walk the directory
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-markdown files
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(filePath, "SKILL.md") {
			return nil
		}

		// Load the skill
		skill, err := l.parseSkill(filePath, category)
		if err != nil {
			fmt.Printf("Warning: failed to load skill from %s: %v\n", filePath, err)
			return nil // Continue loading other files
		}

		l.skills[skill.Name] = skill
		return nil
	})

	return err
}

// getCategoryFromPath determines the category from a directory path
func (l *Loader) getCategoryFromPath(path string) string {
	switch {
	case strings.Contains(path, "core"):
		return "core"
	case strings.Contains(path, "community"):
		return "community"
	case strings.Contains(path, "experimental"):
		return "experimental"
	default:
		return "unknown"
	}
}

// parseSkill parses a single SKILL.md file and extracts metadata only
// Full content is loaded on demand via LoadFullContent()
func (l *Loader) parseSkill(filePath, category string) (*Skill, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inFrontmatter := false
	frontmatter := []string{}

	// Only parse frontmatter for metadata (Agent Skills spec)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter delimiter
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				// Start of frontmatter
				inFrontmatter = true
				continue
			} else {
				// End of frontmatter
				break
			}
		}

		if inFrontmatter {
			frontmatter = append(frontmatter, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse frontmatter
	name, description := l.parseFrontmatter(frontmatter)

	if name == "" {
		// Extract name from filename if not in frontmatter
		base := filepath.Base(filepath.Dir(filePath))
		name = strings.ToLower(strings.ReplaceAll(base, "-", "_"))
	}

	// Create skill with metadata only - content loaded on demand
	skill := &Skill{
		Name:        name,
		Description: description,
		Content:     "", // Will be loaded on demand
		FilePath:    filePath,
		Directory:   filepath.Dir(filePath),
		Category:    category,
		Keywords:    l.extractKeywords(name, description),
	}

	return skill, nil
}

// parseFrontmatter extracts name and description from frontmatter lines
func (l *Loader) parseFrontmatter(lines []string) (name, description string) {
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse key: value pairs
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(strings.ToLower(parts[0]))
		value := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			name = value
		case "description":
			description = strings.Trim(value, "\"")
		}
	}

	return name, description
}

// extractKeywords extracts relevant keywords from skill name and description
func (l *Loader) extractKeywords(name, description string) []string {
	var keywords []string

	// Add name words
	nameWords := strings.Fields(strings.ToLower(name))
	keywords = append(keywords, nameWords...)

	// Add description words (filtered)
	descWords := strings.Fields(strings.ToLower(description))

	// Filter common words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true,
		"or": true, "but": true, "in": true, "on": true,
		"at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "from": true,
	}

	for _, word := range descWords {
		if !stopWords[word] && len(word) > 2 {
			keywords = append(keywords, word)
		}
	}

	return keywords
}

// buildKeywordIndex builds an index of keywords to skills for fast matching
func (l *Loader) buildKeywordIndex() {
	l.indexPath = make(map[string][]*Skill)

	for _, skill := range l.skills {
		for _, keyword := range skill.Keywords {
			l.indexPath[keyword] = append(l.indexPath[keyword], skill)
		}
	}
}

// GetSkill retrieves a skill by name
func (l *Loader) GetSkill(name string) (*Skill, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skill, ok := l.skills[name]
	return skill, ok
}

// ListSkills returns all loaded skills
func (l *Loader) ListSkills() []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	skills := make([]*Skill, 0, len(l.skills))
	for _, skill := range l.skills {
		skills = append(skills, skill)
	}

	return skills
}

// SearchSkills searches for skills by keywords
func (l *Loader) SearchSkills(query string) []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	query = strings.ToLower(query)
	var results []*Skill

	// Check if query matches a skill name directly
	if skill, ok := l.skills[query]; ok {
		results = append(results, skill)
		return results
	}

	// Search by keywords
	for _, skill := range l.skills {
		name := strings.ToLower(skill.Name)
		description := strings.ToLower(skill.Description)

		if strings.Contains(name, query) || strings.Contains(description, query) {
			results = append(results, skill)
		}
	}

	return results
}

// GetSkillsForContext returns relevant skills based on the user message
func (l *Loader) GetSkillsForContext(userMessage string) []*Skill {
	l.mu.RLock()
	defer l.mu.RUnlock()

	userMessage = strings.ToLower(userMessage)
	var relevantSkills []*Skill

	// Check for keyword matches using index
	for keyword, skills := range l.indexPath {
		if strings.Contains(userMessage, keyword) {
			relevantSkills = append(relevantSkills, skills...)
		}
	}

	// Also do a broader search
	for _, skill := range l.skills {
		if skill.matchesContext(userMessage) {
			// Avoid duplicates
			alreadyAdded := false
			for _, s := range relevantSkills {
				if s.Name == skill.Name {
					alreadyAdded = true
					break
				}
			}
			if !alreadyAdded {
				relevantSkills = append(relevantSkills, skill)
			}
		}
	}

	return relevantSkills
}

// matchesContext checks if a skill is relevant to the given context
func (s *Skill) matchesContext(context string) bool {
	context = strings.ToLower(context)
	name := strings.ToLower(s.Name)
	description := strings.ToLower(s.Description)

	// Direct name match
	if strings.Contains(context, name) {
		return true
	}

	// Description keyword match
	descWords := strings.Fields(description)
	for _, word := range descWords {
		if len(word) > 4 && strings.Contains(context, word) {
			return true
		}
	}

	return false
}

// LoadFullContent loads the full skill content from disk
func (s *Skill) LoadFullContent() error {
	file, err := os.Open(s.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open skill file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	inFrontmatter := false
	skippingFrontmatter := true

	for scanner.Scan() {
		line := scanner.Text()

		// Check for frontmatter delimiter
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			} else {
				inFrontmatter = false
				skippingFrontmatter = false
				continue
			}
		}

		// Skip frontmatter, capture content
		if !skippingFrontmatter {
			lines = append(lines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read skill file: %w", err)
	}

	s.Content = strings.Join(lines, "\n")
	return nil
}

// Reload reloads all skills from disk
func (l *Loader) Reload() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Clear existing skills
	l.skills = make(map[string]*Skill)
	l.indexPath = make(map[string][]*Skill)

	// Reload from paths
	for _, path := range l.paths {
		if err := l.loadFromPath(path); err != nil {
			return fmt.Errorf("failed to reload skills from %s: %w", path, err)
		}
	}

	// Rebuild index
	l.buildKeywordIndex()

	fmt.Printf("Reloaded %d skills\n", len(l.skills))
	return nil
}
