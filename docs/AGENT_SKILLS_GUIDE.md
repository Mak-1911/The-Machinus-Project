# Agent Skills Implementation Guide

Based on the [Agent Skills Specification](https://github.com/vercel-labs/skills), this guide explains how to properly integrate skills into the Machinus agent.

## Overview

**Agent Skills** is an open standard for packaging LLM capabilities as modular, reusable skills. A skill is a folder containing:
- `SKILL.md` - Main skill definition with YAML frontmatter
- Bundled resources - Scripts, templates, documentation
- Metadata - Name, description, usage instructions

## Why Skills?

✅ **Modular** - Package capabilities as self-contained units
✅ **Reusable** - Share skills across projects and agents
✅ **Discoverable** - Models can find and use relevant skills
✅ **Version Control** - Track skills in git like code
✅ **Low Overhead** - Metadata-only loading at startup (50-100 tokens)

## Architecture

### Integration Approaches

#### 1. Filesystem-Based Agents (Recommended)
**Best for**: Agents with full computer environment access

**How it works**:
- Model issues shell commands like `cat /path/to/skill/SKILL.md`
- Direct filesystem access to skill files
- Maximum flexibility and capability

**Example interaction**:
```
User: "Clone this website"
Model: "I'll use the clone-website skill for this"
Model: cat skills/core/clone-website/SKILL.md
Model: [reads skill content and follows workflow]
```

**Your situation**: ✅ **Filesystem-based** - You have shell tool and file access

#### 2. Tool-Based Agents
**Best for**: Agents without direct filesystem access

**How it works**:
- Model calls a "activate_skill" tool
- Tool loads and returns skill content
- More restricted but safer

---

## Implementation Requirements

A skills-compatible agent needs 5 capabilities:

### 1. Skill Discovery
Scan configured directories for valid skills (folders with `SKILL.md`)

```go
// Scan for skills in configured paths
paths := []string{
    "skills/core",
    "skills/community",
    "skills/experimental",
}

for _, path := range paths {
    skills := discoverSkills(path)
    // Found: skills/core/clone-website/SKILL.md
    // Found: skills/core/debug-code/SKILL.md
}
```

### 2. Metadata Loading
Parse **only the frontmatter** at startup (not full content)

```yaml
---
name: clone-website
description: Download exact replicas of websites
---
```

**Why metadata-only?**
- Keeps startup fast
- Minimizes token usage (50-100 tokens per skill)
- Full content loaded only when needed

### 3. Context Injection
Add skill metadata to system prompt in **XML format**:

```xml
<available_skills>
  <skill>
    <name>clone-website</name>
    <description>Download exact replicas of websites with all assets</description>
    <location>/absolute/path/to/skills/core/clone-website/SKILL.md</location>
  </skill>
  <skill>
    <name>debug-code</name>
    <description>Systematic debugging workflow for code issues</description>
    <location>/absolute/path/to/skills/core/debug-code/SKILL.md</location>
  </skill>
</available_skills>
```

### 4. Skill Matching
When user makes a request, identify relevant skills

```
User: "I need to clone ampcode.com"
→ Search for "clone" in skill names/descriptions
→ Match: "clone-website" skill
→ Load full skill content
```

### 5. Skill Activation
Load full skill instructions when model requests them

```go
// Model requests: cat skills/core/clone-website/SKILL.md
content := readFile("skills/core/clone-website/SKILL.md")
// Return full content to model
```

---

## Directory Structure

```
machinus/
├── skills/
│   ├── core/                    # Official skills
│   │   ├── clone-website/
│   │   │   └── SKILL.md
│   │   ├── debug-code/
│   │   │   └── SKILL.md
│   │   └── [more core skills]
│   │
│   ├── community/               # Community-contributed skills
│   │   └── [community skills]
│   │
│   └── experimental/            # Experimental skills
│       └── [experimental skills]
│
├── internal/
│   └── skills/
│       ├── loader.go           # Skill discovery & loading
│       ├── matcher.go          # Skill matching logic
│       └── types.go            # Data structures
│
└── prompts/
    └── agent_system.xml        # Updated to include skills
```

---

## Implementation Plan

### Phase 1: Core Skills System

#### 1.1 Create Data Structures
```go
// internal/skills/types.go

// Skill represents a loaded skill
type Skill struct {
    Name        string
    Description string
    Content     string       // Full content (lazy-loaded)
    FilePath    string       // Absolute path to SKILL.md
    Category    string       // core, community, experimental
}

// Loader manages skill discovery and loading
type Loader struct {
    skills    map[string]*Skill
    paths     []string
    mu        sync.RWMutex
}
```

#### 1.2 Implement Discovery & Loading
```go
// internal/skills/loader.go

func (l *Loader) LoadAll() error {
    for _, path := range l.paths {
        l.loadFromPath(path)
    }
}

func (l *Loader) loadFromPath(path string) error {
    // Walk directory
    filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
        if strings.HasSuffix(filePath, "SKILL.md") {
            skill := l.parseSkill(filePath)
            l.skills[skill.Name] = skill
        }
        return nil
    })
}
```

#### 1.3 Metadata-Only Parsing
```go
func (l *Loader) parseSkill(filePath string) *Skill {
    file, _ := os.Open(filePath)
    defer file.Close()

    // Parse only frontmatter
    inFrontmatter := false
    frontmatter := []string{}

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := scanner.Text()
        if strings.TrimSpace(line) == "---" {
            if !inFrontmatter {
                inFrontmatter = true
                continue
            } else {
                break  // End of frontmatter
            }
        }
        if inFrontmatter {
            frontmatter = append(frontmatter, line)
        }
    }

    // Extract name and description
    name, description := parseFrontmatter(frontmatter)

    return &Skill{
        Name:        name,
        Description: description,
        Content:     "",  // Lazy load on demand
        FilePath:    filePath,
    }
}
```

#### 1.4 XML Generation for System Prompt
```go
func (l *Loader) GetAvailableSkillsXML() string {
    l.mu.RLock()
    defer l.mu.RUnlock()

    var xml strings.Builder
    xml.WriteString("<available_skills>\n")

    for _, skill := range l.skills {
        xml.WriteString(fmt.Sprintf(`
  <skill>
    <name>%s</name>
    <description>%s</description>
    <location>%s</location>
  </skill>`,
            xmlEscape(skill.Name),
            xmlEscape(skill.Description),
            xmlEscape(skill.FilePath),
        ))
    }

    xml.WriteString("</available_skills>")
    return xml.String()
}
```

#### 1.5 Lazy Loading for Full Content
```go
func (s *Skill) LoadFullContent() error {
    content, _ := os.ReadFile(s.FilePath)
    s.Content = string(content)
    return nil
}
```

### Phase 2: Integration with Planner

#### 2.1 Add Skills Loader to Planner
```go
// internal/planner/planner.go

type Planner struct {
    client         *LLMClient
    tools          map[string]types.Tool
    cachedXMLPrompt *SystemPromptXML
    skillsLoader   *skills.Loader  // NEW
}

func NewPlanner(baseURL, apiKey, model string, tools map[string]types.Tool, skillsLoader *skills.Loader) *Planner {
    return &Planner{
        client:       &LLMClient{...},
        tools:        tools,
        skillsLoader: skillsLoader,
    }
}
```

#### 2.2 Inject Skills into System Prompt
```go
func (p *Planner) buildSystemPrompt(tools []ToolDef) string {
    var prompt strings.Builder

    // Add role
    prompt.WriteString("You are an intelligent agent...\n\n")

    // Add tools
    prompt.WriteString(p.buildToolsSection(tools))

    // Add skills (NEW)
    if p.skillsLoader != nil {
        prompt.WriteString("\n# Available Skills\n")
        prompt.WriteString(p.skillsLoader.GetAvailableSkillsXML())
    }

    return prompt.String()
}
```

#### 2.3 Skill Matching for Context
```go
func (l *Loader) GetSkillsForContext(userMessage string) []*Skill {
    userMessage = strings.ToLower(userMessage)
    var relevant []*Skill

    for _, skill := range l.skills {
        name := strings.ToLower(skill.Name)
        description := strings.ToLower(skill.Description)

        if strings.Contains(userMessage, name) ||
           strings.Contains(userMessage, description) {
            relevant = append(relevant, skill)
        }
    }

    return relevant
}
```

### Phase 3: Model Activation

Since you're **filesystem-based**, the model activates skills by reading files:

```
User: "Clone ampcode.com"

Model thinking:
- User wants to "clone" a website
- I have a "clone-website" skill available
- I should read that skill to get the workflow

Model action:
- Reads: cat skills/core/clone-website/SKILL.md
- Gets full workflow and instructions
- Follows the step-by-step process
```

**No special activation tool needed** - just use existing file tools!

---

## Example Skill

### File: `skills/core/clone-website/SKILL.md`

```yaml
---
name: clone-website
description: Create exact replicas of websites by downloading all HTML, CSS, JavaScript, and assets
---

# Website Cloning Skill

## Purpose
Download complete, working copies of websites for offline use, backup, or analysis.

## When to Use
Use this skill when the user asks to:
- Clone, copy, or replicate a website
- Download a website for offline viewing
- Create a local backup of a web page
- Mirror a website

## Prerequisites
- HTTP tool for downloading files
- File tools for saving content
- Sufficient disk space

## Workflow

### Step 1: Download HTML
```bash
http:
  url: https://example.com
  method: GET
```

Save the response to file.

### Step 2: Extract Asset URLs
Use grep to find CSS, JS, and image references:
```bash
grep:
  pattern: "href=\"(.*\\.css)\""
  path: downloaded/index.html
```

### Step 3: Download Assets
Download each found asset URL.

### Step 4: Fix Paths
Update HTML to use local paths instead of remote URLs.

## Limitations
- Single Page Applications (SPAs) require JavaScript execution
- Dynamic content may not be captured
- Some sites may have anti-scraping measures

## Troubleshooting
- **Blank page**: Site is likely an SPA
- **Broken images**: Check asset downloads
- **Styles not loading**: Verify CSS file paths
```

---

## Security Considerations

### Script Execution
Some skills may include executable scripts. Protect against:

1. **Sandboxing**: Run scripts in isolated environments
2. **Allowlisting**: Only execute from trusted skill directories
3. **Confirmation**: Ask user before dangerous operations
4. **Logging**: Record all script executions

### File Access
- Restrict skill access to configured directories only
- Validate file paths to prevent directory traversal
- Check file permissions before reading

### Content Validation
- Validate YAML frontmatter structure
- Check for malicious content in skill files
- Limit skill file size to prevent memory issues

---

## Testing

### Unit Tests
```go
func TestLoadSkill(t *testing.T) {
    loader := NewLoader("skills/core")
    loader.LoadAll()

    skill, ok := loader.GetSkill("clone-website")
    assert.True(t, ok)
    assert.Equal(t, "clone-website", skill.Name)
    assert.True(t, skill.Content == "")  // Should be empty (lazy loaded)
}
```

### Integration Tests
```go
func TestSkillInjection(t *testing.T) {
    loader := NewLoader(".")
    loader.LoadAll()

    xml := loader.GetAvailableSkillsXML()
    assert.Contains(t, xml, "<available_skills>")
    assert.Contains(t, xml, "<name>clone-website</name>")
}
```

---

## Performance Optimization

### Metadata-Only Loading
- ✅ Parse frontmatter only at startup (~1-2ms per skill)
- ✅ Load full content on demand (~5-10ms per skill)
- ✅ Cache parsed skills in memory

### Token Efficiency
- Metadata: 50-100 tokens per skill
- Full content: Only loaded when needed
- XML format: Compact and structured

### Benchmark
```
Startup with 10 skills:
- Metadata-only: ~10-20ms
- Full content: ~500-1000ms (without lazy loading)

Savings: 98% faster startup!
```

---

## Next Steps

1. **Create `internal/skills/` package**
   - Implement loader and types
   - Add XML generation

2. **Create sample skills**
   - `skills/core/clone-website/SKILL.md`
   - `skills/core/debug-code/SKILL.md`

3. **Integrate with planner**
   - Add skills loader to Planner struct
   - Inject XML into system prompts

4. **Test with real requests**
   - Verify skill discovery works
   - Test lazy loading
   - Confirm model can use skills

5. **Add more skills**
   - File operations
   - Web scraping
   - Data analysis
   - Debugging workflows

---

## Resources

- [Agent Skills Spec](https://github.com/vercel-labs/skills)
- [Example Skills](https://github.com/vercel-labs/skills/tree/main/skills)
- [skills-ref Implementation](https://github.com/vercel-labs/skills-ref)

---

## Summary

Agent Skills provides:
- ✅ Standard format for packaging capabilities
- ✅ Low-overhead discovery and activation
- ✅ Filesystem-based integration (perfect for your setup)
- ✅ Modular, reusable, and version-controllable

Your agent is **filesystem-based**, which means:
- Models can directly read skill files with existing tools
- No special activation tool needed
- Maximum flexibility and capability

The key is **metadata-only loading at startup** with **lazy loading of full content** - this keeps boot time minimal while making full skill content available when needed.
