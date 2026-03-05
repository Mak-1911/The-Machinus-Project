package skills

import (
	"sync"
)

// Skill represents a loaded skill from a SKILL.md file
type Skill struct {
	Name        string   // Skill name from frontmatter
	Description string   // Skill description from frontmatter
	Content     string   // Full content (lazy-loaded)
	FilePath    string   // Absolute path to SKILL.md
	Directory   string   // Absolute path to skill directory
	Category    string   // core, community, experimental
	Keywords    []string // Extracted keywords for matching
}

// Loader manages skill discovery and loading
type Loader struct {
	skills    map[string]*Skill  // name -> skill mapping
	paths     []string           // paths to scan for skills
	mu        sync.RWMutex       // thread-safe access
	indexPath map[string][]*Skill // keyword -> skills mapping
}

// Config holds loader configuration
type Config struct {
	Paths []string // Directories to scan for skills
}

// Frontmatter holds parsed YAML frontmatter from SKILL.md
type Frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// SkillMatch represents a matched skill with relevance score
type SkillMatch struct {
	Skill       *Skill
	Relevance   float64 // 0.0 to 1.0
	MatchReason string  // Why this skill matched
}
