package prompt

import (
	"testing"

	"github.com/machinus/cloud-agent/internal/types"
)

func TestBuildFull(t *testing.T) {
	cfg := Config{
		Mode:          ModeFull,
		WorkDir:       "/test/dir",
		ModelName:     "test-model",
		Tools:         map[string]types.Tool{},
		SafetyEnabled: true,
	}

	result := BuildFull(cfg)

	if result == "" {
		t.Fatal("BuildFull returned empty string")
	}

	// Check for key sections
	if !contains(result, "Machinus") {
		t.Error("Missing identity section")
	}
	if !contains(result, "Runtime") {
		t.Error("Missing runtime section")
	}
	if !contains(result, "Tools") {
		t.Error("Missing tools section")
	}

	t.Logf("Generated prompt:\n%s", result)
}

func TestBuildMinimal(t *testing.T) {
	cfg := Config{
		Mode:      ModeMinimal,
		WorkDir:   "/test/dir",
		ModelName: "test-model",
		Tools:     map[string]types.Tool{},
	}

	result := NewBuilder(cfg).Build()

	if result == "" {
		t.Fatal("BuildMinimal returned empty string")
	}

	// Minimal mode should still have identity and runtime
	if !contains(result, "Machinus") {
		t.Error("Missing identity section")
	}
	if !contains(result, "Runtime") {
		t.Error("Missing runtime section")
	}

	t.Logf("Generated minimal prompt:\n%s", result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
