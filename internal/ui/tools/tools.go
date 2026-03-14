// Package tools provides tool parameter types for the UI.
package tools

// ============================================================================
// Bash Tool
// ============================================================================

// BashParams represents bash tool parameters.
type BashParams struct {
	Command         string `json:"command"`
	RunInBackground bool   `json:"run_in_background"`
}

// BashResponseMetadata represents bash response metadata.
type BashResponseMetadata struct {
	Background  bool   `json:"background"`
	ShellID     string `json:"shell_id"`
	Output      string `json:"output"`
	Description string `json:"description"`
}

// BashNoOutput is the content returned when bash has no output.
const BashNoOutput = "(no output)"

// ============================================================================
// Job Output Tool
// ============================================================================

// JobOutputParams represents job_output tool parameters.
type JobOutputParams struct {
	ShellID string `json:"shell_id"`
}

// JobOutputResponseMetadata represents job_output response metadata.
type JobOutputResponseMetadata struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ============================================================================
// Job Kill Tool
// ============================================================================

// JobKillParams represents job_kill tool parameters.
type JobKillParams struct {
	ShellID string `json:"shell_id"`
}

// JobKillResponseMetadata represents job_kill response metadata.
type JobKillResponseMetadata struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// ============================================================================
// Fetch Tool
// ============================================================================

// FetchParams represents fetch tool parameters.
type FetchParams struct {
	URL     string `json:"url"`
	Format  string `json:"format,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
}

// ============================================================================
// WebFetch Tool
// ============================================================================

// WebFetchParams represents web_fetch tool parameters.
type WebFetchParams struct {
	URL string `json:"url"`
}

// ============================================================================
// WebSearch Tool
// ============================================================================

// WebSearchParams represents web_search tool parameters.
type WebSearchParams struct {
	Query string `json:"query"`
}

// ============================================================================
// Diagnostics Tool
// ============================================================================

// DiagnosticsParams represents diagnostics tool parameters.
type DiagnosticsParams struct {
	FilePath string `json:"file_path,omitempty"`
}

// ============================================================================
// Read Tool
// ============================================================================

// ReadParams represents read tool parameters.
type ReadParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ============================================================================
// Write Tool
// ============================================================================

// WriteParams represents write tool parameters.
type WriteParams struct {
	FilePath  string `json:"file_path"`
	Content   string `json:"content"`
	CreateDir bool   `json:"create_dir,omitempty"`
}

// ============================================================================
// Edit Tool
// ============================================================================

// EditParams represents edit tool parameters.
type EditParams struct {
	FilePath    string `json:"file_path"`
	OldString   string `json:"old_string"`
	NewString   string `json:"new_string"`
	ReplaceAll  bool   `json:"replace_all,omitempty"`
}

// EditResponseMetadata represents edit response metadata.
type EditResponseMetadata struct {
	Diff       string `json:"diff"`
	OldContent string `json:"old_content"`
	NewContent string `json:"new_content"`
}

// MultiEditParams represents multi_edit tool parameters.
type MultiEditParams struct {
	FilePath string `json:"file_path"`
	Edits    []Edit `json:"edits"`
}

// Edit represents a single edit operation.
type Edit struct {
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

// MultiEditResponseMetadata represents multi_edit response metadata.
type MultiEditResponseMetadata struct {
	EditsMade   int    `json:"edits_made"`
	OldContent  string `json:"old_content"`
	NewContent  string `json:"new_content"`
	EditsFailed int    `json:"edits_failed"`
	EditsApplied int   `json:"edits_applied"`
}

// ============================================================================
// View Tool
// ============================================================================

// ViewParams represents view tool parameters.
type ViewParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

// ViewResponseMetadata represents view response metadata.
type ViewResponseMetadata struct {
	FileType            string `json:"file_type"`
	Content             string `json:"content"`
	ResourceType        string `json:"resource_type"`
	ResourceName        string `json:"resource_name"`
	ResourceDescription string `json:"resource_description"`
	FilePath            string `json:"file_path"`
}

// ViewResourceSkill represents a view resource skill.
const ViewResourceSkill = "skill"

// ============================================================================
// Glob Tool
// ============================================================================

// GlobParams represents glob tool parameters.
type GlobParams struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path,omitempty"`
}

// ============================================================================
// References Tool
// ============================================================================

// ReferencesParams represents references tool parameters.
type ReferencesParams struct {
	Symbol string `json:"symbol"`
	Path   string `json:"path,omitempty"`
}

// ============================================================================
// LS Tool
// ============================================================================

// LSParams represents ls tool parameters.
type LSParams struct {
	Path string `json:"path,omitempty"`
}

// ============================================================================
// Grep Tool
// ============================================================================

// GrepParams represents grep tool parameters.
type GrepParams struct {
	Pattern       string `json:"pattern"`
	Path          string `json:"path,omitempty"`
	Glob          string `json:"glob,omitempty"`
	OutputMode    string `json:"output_mode,omitempty"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
	Include       string `json:"include,omitempty"`
	LiteralText   bool   `json:"literal_text,omitempty"`
}

// ============================================================================
// LSP Restart Tool
// ============================================================================

// LSPRestartParams represents lsp_restart tool parameters.
type LSPRestartParams struct {
	Name string `json:"name,omitempty"`
}

// ============================================================================
// MCP Tool
// ============================================================================

// MCPToolParams represents generic MCP tool parameters.
type MCPToolParams struct {
	Name string          `json:"name"`
	Args map[string] any `json:"arguments"`
}

// ============================================================================
// Download Tool
// ============================================================================

// DownloadParams represents download tool parameters.
type DownloadParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path,omitempty"`
	Timeout  int    `json:"timeout,omitempty"`
}

// ============================================================================
// Upload Tool
// ============================================================================

// UploadParams represents upload tool parameters.
type UploadParams struct {
	FilePath string `json:"file_path"`
	FileName string `json:"file_name,omitempty"`
}

// ============================================================================
// Todos Tool
// ============================================================================

// TodosParams represents todos tool parameters.
type TodosParams struct {
	Todos []TodoItem `json:"todos,omitempty"`
}

// TodosResponseMetadata represents todos response metadata.
type TodosResponseMetadata struct {
	Todos          []TodoItem `json:"todos"`
	IsNew          bool       `json:"is_new"`
	JustStarted    string     `json:"just_started"`
	Total          int        `json:"total"`
	Completed      int        `json:"completed"`
	JustCompleted  []string   `json:"just_completed"`
}

// TodoItem represents a todo item.
type TodoItem struct {
	ID         string `json:"id"`
	Message    string `json:"message"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	ActiveForm string `json:"active_form"`
}

// ============================================================================
// Agentic Fetch Tool
// ============================================================================

// AgenticFetchParams represents agentic_fetch tool parameters.
type AgenticFetchParams struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt,omitempty"`
}

// ============================================================================
// Tool Names
// ============================================================================

// BashToolName is the name of the bash tool.
const BashToolName = "bash"

// JobOutputToolName is the name of the job_output tool.
const JobOutputToolName = "job_output"

// JobKillToolName is the name of the job_kill tool.
const JobKillToolName = "job_kill"

// ViewToolName is the name of the view tool.
const ViewToolName = "view"

// WriteToolName is the name of the write tool.
const WriteToolName = "write"

// GlobToolName is the name of the glob tool.
const GlobToolName = "glob"

// GrepToolName is the name of the grep tool.
const GrepToolName = "grep"

// LSToolName is the name of the ls tool.
const LSToolName = "ls"

// DownloadToolName is the name of the download tool.
const DownloadToolName = "download"

// FetchToolName is the name of the fetch tool.
const FetchToolName = "fetch"

// SourcegraphToolName is the name of the sourcegraph tool.
const SourcegraphToolName = "sourcegraph"

// DiagnosticsToolName is the name of the diagnostics tool.
const DiagnosticsToolName = "diagnostics"

// AgentToolName is the name of the agent tool.
const AgentToolName = "agent"

// AgenticFetchToolName is the name of the agentic_fetch tool.
const AgenticFetchToolName = "agentic_fetch"

// WebFetchToolName is the name of the web_fetch tool.
const WebFetchToolName = "web_fetch"

// WebSearchToolName is the name of the web_search tool.
const WebSearchToolName = "web_search"

// TodosToolName is the name of the todos tool.
const TodosToolName = "todos"

// ReferencesToolName is the name of the references tool.
const ReferencesToolName = "references"

// LSPRestartToolName is the name of the lsp_restart tool.
const LSPRestartToolName = "lsp_restart"

// ============================================================================
// Sourcegraph Tool
// ============================================================================

// EditPermissionsParams represents edit permissions params.
type EditPermissionsParams struct {
	FilePath    string `json:"file_path"`
	OldContent  string `json:"old_content"`
	NewContent  string `json:"new_content"`
}

// WritePermissionsParams represents write permissions params.
type WritePermissionsParams struct {
	FilePath    string `json:"file_path"`
	OldContent  string `json:"old_content"`
	NewContent  string `json:"new_content"`
}

// MultiEditPermissionsParams represents multi_edit permissions params.
type MultiEditPermissionsParams struct {
	FilePath    string `json:"file_path"`
	OldContent  string `json:"old_content"`
	NewContent  string `json:"new_content"`
}

// ViewPermissionsParams represents view permissions params.
type ViewPermissionsParams struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"`
	Limit    int    `json:"limit"`
}

// LSPermissionsParams represents ls permissions params.
type LSPermissionsParams struct {
	Path    string   `json:"path"`
	Ignore  []string `json:"ignore"`
}

// BashPermissionsParams represents bash permissions params.
type BashPermissionsParams struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// DownloadPermissionsParams represents download permissions params.
type DownloadPermissionsParams struct {
	URL      string `json:"url"`
	FilePath string `json:"file_path"`
	Timeout  int    `json:"timeout"`
}

// FetchPermissionsParams represents fetch permissions params.
type FetchPermissionsParams struct {
	URL string `json:"url"`
}

// AgenticFetchPermissionsParams represents agentic_fetch permissions params.
type AgenticFetchPermissionsParams struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

// SourcegraphParams represents sourcegraph tool parameters.
type SourcegraphParams struct {
	Query         string `json:"query"`
	QueryType     string `json:"query_type,omitempty"`
	Repository    string `json:"repository,omitempty"`
	Count         int    `json:"count,omitempty"`
	ContextWindow int    `json:"context_window,omitempty"`
}

// ============================================================================
// Tool Names
// ============================================================================

// EditToolName is the name of the edit tool.
const EditToolName = "edit"

// MultiEditToolName is the name of the multi_edit tool.
const MultiEditToolName = "multi_edit"

// ResetCache resets the tool cache.
func ResetCache() {
	// Placeholder
}
