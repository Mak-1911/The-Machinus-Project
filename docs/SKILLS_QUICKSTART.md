# Agent Skills - Quick Start Summary

## What is Agent Skills?

**Agent Skills** = Modular, reusable capabilities packaged as folders with a `SKILL.md` file.

Think of it like:
- **npm packages** but for LLM capabilities
- **plugins** but following an open standard
- **skills** that models can discover and use

## Why Use It?

✅ **Organized** - Keep agent capabilities in separate folders
✅ **Discoverable** - Models can find relevant skills automatically
✅ **Fast** - Metadata-only loading (50-100 tokens per skill)
✅ **Version Control** - Track skills in git like code
✅ **Filesystem-Based** - Your agent can read skill files directly

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│ 1. Startup                                                  │
│    - Scan skills/ directories for SKILL.md files           │
│    - Parse only metadata (name, description)                │
│    - Add to system prompt in XML format                     │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. User Request                                             │
│    "Clone ampcode.com"                                      │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. Skill Matching                                           │
│    - Search for "clone" in skill names/descriptions         │
│    - Found: "clone-website" skill                          │
└─────────────────────────────────────────────────────────────┘
                            ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. Skill Activation                                         │
│    Model: "I'll use the clone-website skill"               │
│    Model: cat skills/core/clone-website/SKILL.md           │
│    Model: [reads full workflow and executes]               │
└─────────────────────────────────────────────────────────────┘
```

## Directory Structure

```
machinus/
├── skills/
│   ├── core/
│   │   ├── clone-website/
│   │   │   └── SKILL.md         ← This is a skill
│   │   └── debug-code/
│   │       └── SKILL.md         ← This is a skill
│   ├── community/               ← Community contributions
│   └── experimental/            ← Experimental skills
│
├── internal/
│   └── skills/                  ← Skills system implementation
│       ├── loader.go            ← Discovery & loading
│       ├── matcher.go           ← Skill matching
│       └── types.go             ← Data structures
│
└── prompts/
    └── agent_system.xml         ← Updated with skills XML
```

## Example Skill

**File**: `skills/core/clone-website/SKILL.md`

```yaml
---
name: clone-website
description: Create exact replicas of websites by downloading all HTML, CSS, and assets
---

# Website Cloning Skill

## Purpose
Download complete, working copies of websites.

## When to Use
- User asks to "clone", "copy", or "download" a website
- Need offline backup of a web page

## Workflow
1. Download HTML using http tool
2. Extract CSS/JS/image URLs using grep
3. Download all assets
4. Fix relative paths

## Example
User: "Clone https://example.com"
→ Download HTML
→ Extract assets
→ Download assets
→ Fix paths
→ Done!
```

## Implementation Steps

### Step 1: Create Skills Package
```bash
mkdir -p internal/skills
touch internal/skills/{loader.go,matcher.go,types.go}
```

### Step 2: Implement Discovery & Loading
```go
// internal/skills/loader.go
type Loader struct {
    skills map[string]*Skill
    paths  []string
}

func (l *Loader) LoadAll() error {
    // Scan skills/core, skills/community, skills/experimental
    // Parse SKILL.md files
    // Extract metadata (name, description)
}

func (l *Loader) GetAvailableSkillsXML() string {
    // Generate <available_skills> XML
}
```

### Step 3: Integrate with Planner
```go
// internal/planner/planner.go
type Planner struct {
    client       *LLMClient
    tools        map[string]types.Tool
    skillsLoader *skills.Loader  // NEW
}

func (p *Planner) buildSystemPrompt() string {
    prompt := "You are an intelligent agent..."

    // Add tools
    prompt += buildToolsSection()

    // Add skills (NEW)
    prompt += p.skillsLoader.GetAvailableSkillsXML()

    return prompt
}
```

### Step 4: Update Server/CLI
```go
// cmd/server/main.go
func main() {
    // Initialize skills loader
    skillsLoader := skills.NewLoader(".")
    skillsLoader.LoadAll()

    // Create planner with skills
    planner := planner.NewPlanner(baseURL, apiKey, model, tools, skillsLoader)
}
```

### Step 5: Create Sample Skills
```bash
mkdir -p skills/core/clone-website
# Edit skills/core/clone-website/SKILL.md
```

## Performance

✅ **Fast Startup**: Metadata-only loading (~10-20ms for 10 skills)
✅ **Low Memory**: Cache metadata, lazy load full content
✅ **Token Efficient**: 50-100 tokens per skill (metadata only)

```
Startup: 10 skills
- Metadata: 500-1000 tokens
- Full content: 0 tokens (lazy loaded)
- Time: ~10-20ms
```

## Your Situation: Filesystem-Based Agent

✅ **Perfect fit!** Your agent has:
- Shell tool → Can execute commands
- File tools → Can read/write files
- HTTP tool → Can download content

**Model activates skills by reading files**:
```
Model: "I need the clone-website skill"
Model: cat skills/core/clone-website/SKILL.md
Model: [gets full workflow and follows it]
```

**No special activation tool needed!**

## Key Concepts

### 1. Metadata-Only Loading
Parse **only the frontmatter** at startup:
```yaml
---
name: clone-website
description: Download websites
---
```

Keep the rest (full workflow) for lazy loading.

### 2. XML Format for System Prompts
```xml
<available_skills>
  <skill>
    <name>clone-website</name>
    <description>Download websites</description>
    <location>/path/to/SKILL.md</location>
  </skill>
</available_skills>
```

### 3. Lazy Loading
- Startup: Parse metadata only
- When needed: Load full SKILL.md content
- Result: Fast startup, full capability when needed

### 4. Skill Matching
```
User: "Clone this website"
→ Search skills for "clone"
→ Match: "clone-website"
→ Load full skill
→ Execute workflow
```

## Security

⚠️ **Script Execution Risks**:
- Skills may include executable scripts
- Sandbox execution environments
- Allowlist trusted skill directories
- Ask user before dangerous operations

⚠️ **File Access**:
- Restrict to configured directories
- Validate paths to prevent traversal
- Check file permissions

## Checklist

- [ ] Create `internal/skills/` package
- [ ] Implement skill discovery (walk directories)
- [ ] Implement metadata parsing (YAML frontmatter)
- [ ] Implement XML generation for system prompts
- [ ] Integrate with planner (add to buildSystemPrompt)
- [ ] Update server/cli to initialize skills loader
- [ ] Create sample skills (clone-website, debug-code)
- [ ] Test with real user requests
- [ ] Add security measures (path validation, allowlisting)
- [ ] Document skills for users

## Resources

- 📖 [Full Guide](docs/AGENT_SKILLS_GUIDE.md) - Detailed implementation
- 🔗 [Agent Skills Spec](https://github.com/vercel-labs/skills) - Official spec
- 📦 [Example Skills](https://github.com/vercel-labs/skills/tree/main/skills) - Reference implementations

## Summary

Agent Skills = Standardized way to package agent capabilities as modular, reusable folders.

**Your Benefits**:
- ✅ Better organization of agent capabilities
- ✅ Models can discover and use relevant skills
- ✅ Fast startup with lazy loading
- ✅ Version-controllable like code
- ✅ Perfect for your filesystem-based setup

**Key Point**: You're filesystem-based, so models can just `cat` the skill files directly - no special activation needed!
