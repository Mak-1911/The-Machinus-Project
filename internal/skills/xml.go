package skills

import (
	"fmt"
	"strings"
)

// GetAvailableSkillsXML returns all skills in Agent Skills XML format
// This is injected into the system prompt so the model knows what skills are available
func (l *Loader) GetAvailableSkillsXML() string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var xml strings.Builder
	xml.WriteString("<available_skills>\n")

	for _, skill := range l.skills {
		xml.WriteString(skill.GetMetadataAsXML())
		xml.WriteString("\n")
	}

	xml.WriteString("</available_skills>")
	return xml.String()
}

// GetMetadataAsXML returns the skill metadata in Agent Skills XML format
func (s *Skill) GetMetadataAsXML() string {
	return fmt.Sprintf(`  <skill>
    <name>%s</name>
    <description>%s</description>
    <location>%s</location>
  </skill>`,
		xmlEscape(s.Name),
		xmlEscape(s.Description),
		xmlEscape(s.FilePath),
	)
}

// xmlEscape escapes special XML characters
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// GetRelevantSkillsXML returns skills relevant to the user message in XML format
// This includes full content for relevant skills (not just metadata)
func (l *Loader) GetRelevantSkillsXML(userMessage string, maxSkills int) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	relevantSkills := l.GetSkillsForContext(userMessage)

	// Limit number of skills
	if maxSkills > 0 && len(relevantSkills) > maxSkills {
		relevantSkills = relevantSkills[:maxSkills]
	}

	if len(relevantSkills) == 0 {
		return ""
	}

	var xml strings.Builder
	xml.WriteString("<relevant_skills>\n")

	for _, skill := range relevantSkills {
		xml.WriteString(fmt.Sprintf("  <skill name=\"%s\">\n", xmlEscape(skill.Name)))
		xml.WriteString(fmt.Sprintf("    <description>%s</description>\n", xmlEscape(skill.Description)))

		// Include full content if loaded
		if skill.Content != "" {
			xml.WriteString(fmt.Sprintf("    <content><![CDATA[%s]]></content>\n", skill.Content))
		}

		xml.WriteString("  </skill>\n")
	}

	xml.WriteString("</relevant_skills>")
	return xml.String()
}

// FormatForPrompt formats a skill for inclusion in a system prompt
// This is used when the model needs full skill content
func (s *Skill) FormatForPrompt() string {
	return fmt.Sprintf(`
---
# Skill: %s
%s

%s
---
`, s.Name, s.Description, s.Content)
}
