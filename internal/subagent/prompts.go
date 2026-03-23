package subagent

import "strings"

// buildDefaultPrompt: Creates a minimal system prompt for SubAgents
func buildDefaultPrompt() string {
	var prompt strings.Builder

	prompt.WriteString("You are a specialized agent focused on completing a specific task.\n\n")
	prompt.WriteString("# Rules\n")
	prompt.WriteString("1. Complete your assigned task efficiently\n")
	prompt.WriteString("2. Use tools only when necessary\n")
	prompt.WriteString("3. Stop when the task is done - no extra work\n")
	prompt.WriteString("4. Provide a clear summary of what you did\n\n")
	prompt.WriteString("When finished, respond with a summary of your work.")

	return prompt.String()
}