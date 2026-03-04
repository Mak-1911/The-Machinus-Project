package tools

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
	"os"

	"github.com/machinus/cloud-agent/internal/types"
)

// ShellTool executes shell commands in a sandboxed environment
type ShellTool struct {
	workDir      string
	maxExecution time.Duration
	enabled      bool
}

// dangerousCommands contains commands that should be blocked
var dangerousCommands = []string{
	"rm -rf /",
	"rm -rf /*",
	"mkfs",
	"dd if=/dev/zero",
	"shutdown",
	"reboot",
	"poweroff",
	"halt",
	":(){:|:&};:", // fork bomb
	"chmod -R 777 /",
}

// NewShellTool creates a new shell tool
func NewShellTool(workDir string, maxExecution time.Duration, enabled bool) *ShellTool {
	// If workingDir is ".", get the current working directory
	if workDir == "." {
		if dir, err := os.Getwd(); err == nil {
			workDir = dir
		}
	}
	return &ShellTool{
		workDir:      workDir,
		maxExecution: maxExecution,
		enabled:      enabled,
	}
}



// Name returns the tool name
func (t *ShellTool) Name() string {
	return "shell"
}

// Description returns the tool description
func (t *ShellTool) Description() string {
	return "Execute shell commands. On Windows: uses PowerShell with automatic Unix→PowerShell translation (ls→Get-ChildItem, cat→Get-Content, etc.). On Unix/Linux: uses bash. Use for system operations, building code, running tests, package management, and git operations."
}

// Examples returns example usages
func (t *ShellTool) Examples() []types.ToolExample {
	return []types.ToolExample{
		{
			Input: map[string]any{"cmd": "go build ./..."},
			Description: "Build all Go packages in the project",
		},
		{
			Input: map[string]any{"cmd": "go test ./..."},
			Description: "Run all tests in the project",
		},
		{
			Input: map[string]any{"cmd": "git status"},
			Description: "Check git status",
		},
		{
			Input: map[string]any{"cmd": "npm install"},
			Description: "Install npm dependencies",
		},
		{
			Input: map[string]any{"cmd": "Get-ChildItem"},
			Description: "List files (Windows PowerShell)",
		},
	}
}

// WhenToUse returns when this tool should be used
func (t *ShellTool) WhenToUse() string {
	return "Use when you need to execute system commands, run build tools, execute tests, manage packages, check git status, or perform any shell operation. DO NOT use for file operations (use read_file/write_file/edit_file instead) or searching files (use glob/grep instead). Note: Unix commands like 'ls', 'cat' are automatically translated to PowerShell on Windows."
}

// ChainsWith returns tools that typically follow this tool
func (t *ShellTool) ChainsWith() []string {
	return []string{"read_file", "grep", "glob"}
}

// ValidateArgs validates the tool arguments
func (t *ShellTool) ValidateArgs(args map[string]any) error {

	cmd, ok := args["cmd"].(string)
	if !ok {
		return fmt.Errorf("missing or invalid 'cmd' argument")
	}

	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("command cannot be empty")
	}

	// Check for dangerous commands
	cmdLower := strings.ToLower(cmd)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(cmdLower, dangerous) {
			return fmt.Errorf("dangerous command blocked: %s", dangerous)
		}
	}

	return nil
}

// translateToPowerShell translates Unix shell commands to PowerShell-compatible commands
func translateToPowerShell(cmd string) string {
	// Replace && with ; (PowerShell's command separator)
	cmd = strings.ReplaceAll(cmd, "&&", ";")

	// Handle specific patterns (must check these before general replacements)
	patterns := map[string]string{
		// ls commands
		"ls -la":         "Get-ChildItem -Force",
		"ls -l":          "Get-ChildItem",
		"ls -a":          "Get-ChildItem -Force",
		"ls ":            "Get-ChildItem ",

		// Directory operations
		"mkdir -p ":      "New-Item -ItemType Directory -Force -Path ",
		"mkdir ":         "New-Item -ItemType Directory -Path ",
		"rm -rf ":        "Remove-Item -Recurse -Force ",
		"rm -r ":         "Remove-Item -Recurse ",
		"rmdir ":         "Remove-Item ",

		// File operations
		"cat ":           "Get-Content ",
		"touch ":         "New-Item -ItemType File ",
		"head ":          "Select-Object -First ",
		"tail ":          "Select-Object -Last ",

		// Search operations
		"grep ":          "Select-String -Pattern ",
		"find . -name ":  "Get-ChildItem -Filter ",
		"which ":         "Get-Command ",

		// Other
		"pwd":            "Get-Location",
		"echo ":          "Write-Output ",
	}

	// Check patterns in order (longer patterns first)
	for unix, ps := range patterns {
		if strings.HasPrefix(cmd, unix) || strings.Contains(cmd, unix) {
			cmd = strings.ReplaceAll(cmd, unix, ps)
		}
	}

	return cmd
}

// Execute executes the shell command
func (t *ShellTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	if !t.enabled {
		return types.ToolResult{Success: false, Output: "Shell tool is disabled"}, nil
	}

	cmd, ok := args["cmd"].(string)
	if !ok || cmd == "" {
		return types.ToolResult{Success: false, Output: "missing or invalid 'cmd' argument"}, nil
	}

	// Creating working directory if it doesnt exist
	if err := os.MkdirAll(t.workDir, 0755); err != nil {
		return types.ToolResult{Success: false, Output: fmt.Sprintf("failed to create working directory: %v", err)}, nil
	}

	//Block dangerous commands
	dangerousCommands := []string{"rm -rf /", "rm -rf /*", "mkfs", "dd if=/dev/zero", "shutdown", "reboot", "poweroff", "halt", ":(){:|:&};:"}
	for _, dangerous := range dangerousCommands {
		if strings.Contains(cmd, dangerous) {
			return types.ToolResult{Success: false, Output: fmt.Sprintf("dangerous command blocked: %s", dangerous)}, nil
		}
	}

	// Create command with timeout
	execCtx, cancel := context.WithTimeout(ctx, t.maxExecution)
	defer cancel()

	var execCmd *exec.Cmd
	shellUsed := ""

	// Use appropriate shell for the OS
	if runtime.GOOS == "windows" {
		// Translate Unix commands to PowerShell
		cmd = translateToPowerShell(cmd)
		shellUsed = "powershell"
		execCmd = exec.CommandContext(execCtx, "powershell", "-Command", cmd)
	} else {
		// Unix/Linux/Mac: use sh -c
		shellUsed = "sh"
		if strings.Contains(cmd, "|") || strings.Contains(cmd, ">") || strings.Contains(cmd, "<") || strings.Contains(cmd, "&&") {
			execCmd = exec.CommandContext(execCtx, "sh", "-c", cmd)
		} else {
			parts := strings.Fields(cmd)
			if len(parts) > 0 {
				execCmd = exec.CommandContext(execCtx, parts[0], parts[1:]...)
			} else {
				return types.ToolResult{}, fmt.Errorf("empty command")
			}
		}
	}


	execCmd.Dir = t.workDir

	output, err := execCmd.CombinedOutput()
	success := err == nil
	errorMsg := ""
	if err != nil {
		errorMsg = err.Error()
		// If command failed, include the output in the error message
		if len(output) > 0 {
			errorMsg = fmt.Sprintf("%s: %s", errorMsg, string(output))
		}
	}

	// Build result with error recovery metadata
	result := types.ToolResult{
		Success: success,
		Output:  string(output),
		Error:   errorMsg,
		Data: map[string]any{
			"work_dir": t.workDir,
			"command":  cmd,
			"shell":    shellUsed,
		},
	}

	// Add error recovery metadata for failed commands
	if !success {
		errStr := errorMsg

		// Analyze error type
		if ctx.Err() == context.DeadlineExceeded || strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") {
			// Timeout errors
			result.FailureType = types.FailureTypeSoft
			result.Retryable = true
			result.Alternatives = []string{}
			// Add timeout info to data
			if dataMap, ok := result.Data.(map[string]any); ok {
				dataMap["timeout_hit"] = true
			}
		} else if strings.Contains(errStr, "command not found") || strings.Contains(errStr, "not recognized") {
			// Command not found - typically not retryable with same command
			result.FailureType = types.FailureTypeHard
			result.Retryable = false
			result.Alternatives = []string{"http", "browser"} // May be available as web service
		} else if strings.Contains(errStr, "permission denied") {
			// Permission errors - not retryable
			result.FailureType = types.FailureTypeHard
			result.Retryable = false
			result.Alternatives = []string{}
		} else if strings.Contains(errStr, "no such file or directory") {
			// File not found
			result.FailureType = types.FailureTypeHard
			result.Retryable = false
			result.Alternatives = []string{"glob", "search"}
		} else {
			// Other errors - treat as soft failure, might be transient
			result.FailureType = types.FailureTypeSoft
			result.Retryable = true
			result.Alternatives = []string{}
		}
	}

	return result, nil
}
// MockTool is a simple echo tool for testing
type MockTool struct {
	name string
}

// NewMockTool creates a mock tool
func NewMockTool(name string) *MockTool {
	return &MockTool{name: name}
}

// Name returns the tool name
func (t *MockTool) Name() string {
	return t.name
}

// Description returns the tool description
func (t *MockTool) Description() string {
	return fmt.Sprintf("Mock tool '%s' for testing - echoes back arguments", t.name)
}

// ValidateArgs validates the tool arguments
func (t *MockTool) ValidateArgs(args map[string]any) error {
	return nil
}

// Execute executes the mock tool (echoes back args)
func (t *MockTool) Execute(ctx context.Context, args map[string]any) (types.ToolResult, error) {
	return types.ToolResult{
		Success: true,
		Output:  fmt.Sprintf("MockTool '%s' executed with args: %v", t.name, args),
		Data:    args,
	}, nil
}

// Examples returns example usages
func (t *MockTool) Examples() []types.ToolExample {
	return nil
}

// WhenToUse returns when this tool should be used
func (t *MockTool) WhenToUse() string {
	return ""
}

// ChainsWith returns tools that typically follow this tool
func (t *MockTool) ChainsWith() []string {
	return nil
}
