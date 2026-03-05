package main

import (
	"fmt"
	"log"

	"github.com/machinus/cloud-agent/internal/skills"
)

func main() {
	fmt.Println("=== Agent Skills Test ===\n")

	// Create loader
	loader := skills.NewLoader(".")

	// Load skills
	if err := loader.LoadAll(); err != nil {
		log.Fatalf("Failed to load skills: %v", err)
	}

	// Show loaded skills
	skills := loader.ListSkills()
	fmt.Printf("Loaded %d skills:\n\n", len(skills))

	for _, skill := range skills {
		fmt.Printf("📦 %s\n", skill.Name)
		fmt.Printf("   Description: %s\n", skill.Description)
		fmt.Printf("   Category: %s\n", skill.Category)
		fmt.Printf("   File: %s\n", skill.FilePath)
		fmt.Printf("   Keywords: %v\n", skill.Keywords)
		fmt.Println()
	}

	// Show XML format
	fmt.Println("=== XML Format (for system prompts) ===\n")
	xml := loader.GetAvailableSkillsXML()
	fmt.Println(xml)

	// Test skill matching
	fmt.Println("\n=== Skill Matching ===\n")
	testQueries := []string{
		"I want to clone a website",
		"There's a bug in my code",
		"Help me debug this issue",
		"Download example.com",
	}

	for _, query := range testQueries {
		matched := loader.GetSkillsForContext(query)
		fmt.Printf("Query: \"%s\"\n", query)
		if len(matched) > 0 {
			for _, skill := range matched {
				fmt.Printf("  → Matched: %s\n", skill.Name)
			}
		} else {
			fmt.Printf("  → No matches\n")
		}
		fmt.Println()
	}

	// Test lazy loading
	fmt.Println("=== Lazy Loading ===\n")
	if len(skills) > 0 {
		skill := skills[0]
		fmt.Printf("Before LoadFullContent(): Content length = %d\n", len(skill.Content))

		if err := skill.LoadFullContent(); err != nil {
			log.Printf("Failed to load content: %v", err)
		} else {
			fmt.Printf("After LoadFullContent(): Content length = %d\n", len(skill.Content))
			fmt.Printf("First 100 chars: %s...\n", skill.Content[:100])
		}
	}

	fmt.Println("\n=== Agent Skills Test Complete ===")
}
