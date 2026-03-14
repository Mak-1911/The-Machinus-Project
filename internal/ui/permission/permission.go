// Package permission provides permission types for the UI.
package permission

import "errors"

// Permission represents a permission request.
type Permission struct {
	ID          string            `json:"id"`
	Type        PermissionType    `json:"type"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Details     string            `json:"details,omitempty"`
	AlwaysAllow bool              `json:"always_allow,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ToolName    string            `json:"tool_name,omitempty"`
	Path        string            `json:"path,omitempty"`
	Params      interface{}       `json:"params,omitempty"`
}

// PermissionType is the type of permission.
type PermissionType string

const (
	PermissionTypeExec      PermissionType = "exec"
	PermissionTypeWrite     PermissionType = "write"
	PermissionTypeNetwork   PermissionType = "network"
	PermissionTypeSession   PermissionType = "session"
	PermissionTypeTool      PermissionType = "tool"
	PermissionTypeAlways    PermissionType = "always"
	PermissionTypeSessionSoa PermissionType = "session_soa"
	PermissionTypeBackground PermissionType = "background"
)

// PermissionStatus is the status of a permission.
type PermissionStatus string

const (
	PermissionStatusPending  PermissionStatus = "pending"
	PermissionStatusApproved PermissionStatus = "approved"
	PermissionStatusDenied   PermissionStatus = "denied"
	PermissionStatusCanceled PermissionStatus = "canceled"
)

// PermissionRequest represents a permission request with callback.
type PermissionRequest struct {
	Permission  Permission
	ResponseCh   chan PermissionResponse
	ToolName     string
	Path         string
	Params       string
}

// PermissionResponse is the response to a permission request.
type PermissionResponse struct {
	Approved   bool   `json:"approved"`
	AlwaysAllow bool  `json:"always_allow,omitempty"`
}

// NewPermissionRequest creates a new permission request.
func NewPermissionRequest(pType PermissionType, title, description string) *PermissionRequest {
	return &PermissionRequest{
		Permission: Permission{
			Type:        pType,
			Title:       title,
			Description: description,
		},
		ResponseCh: make(chan PermissionResponse, 1),
	}
}

// ToolPermissionData is metadata for tool permissions.
type ToolPermissionData struct {
	ToolName string `json:"tool_name"`
	ToolArgs string `json:"tool_args"`
}

// PermissionNotification represents a permission notification.
type PermissionNotification struct {
	Permission Permission
	ToolCallID  string
	Granted     bool
}

// ErrorPermissionDenied is returned when permission is denied.
var ErrorPermissionDenied = errors.New("permission denied")
