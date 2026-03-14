// Package mcp provides MCP tool types for the UI.
package mcp

import (
	"encoding/json"
)

// Schema represents a JSON schema for tool parameters.
type Schema struct {
	Type        string             `json:"type"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Default     any                `json:"default,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []string           `json:"enum,omitempty"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"input_schema"`
}

// Arg represents an MCP tool argument.
type Arg struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolInfo contains information about an MCP tool.
type ToolInfo struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Args         []Arg  `json:"args"`
	JSONSchema   string `json:"json_schema,omitempty"`
}

// ParseSchema parses a JSON schema string.
func ParseSchema(schemaStr string) (*Schema, error) {
	var schema Schema
	if err := json.Unmarshal([]byte(schemaStr), &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

// String returns the JSON string representation of the schema.
func (s *Schema) String() string {
	if s == nil {
		return "{}"
	}
	data, _ := json.Marshal(s)
	return string(data)
}

// GetArgType returns the type of a property.
func (s *Schema) GetArgType(name string) string {
	if s == nil || s.Properties == nil {
		return ""
	}
	prop, ok := s.Properties[name]
	if !ok {
		return ""
	}
	return prop.Type
}

// IsRequired checks if a property is required.
func (s *Schema) IsRequired(name string) bool {
	if s == nil {
		return false
	}
	for _, req := range s.Required {
		if req == name {
			return true
		}
	}
	return false
}

// Resource represents an MCP resource.
type Resource struct {
	URI      string `json:"uri"`
	Name     string `json:"name"`
	MIMEType string `json:"mime_type,omitempty"`
	Text     string `json:"text,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// Resources returns all available MCP resources.
// This is a placeholder implementation.
func Resources() map[string][]Resource {
	return make(map[string][]Resource)
}

// ClientInfo represents MCP client information.
type ClientInfo struct {
	Name    string
	Version string
}

// Counts represents MCP counts.
type Counts struct {
	Servers   int
	Tools     int
	Resources int
	Prompts   int
}

// Event represents an MCP event.
type Event struct {
	Type string
	Name string
	Data any
}

// EventStateChanged represents a state changed event.
const EventStateChanged = "state_changed"

// EventPromptsListChanged represents prompts list changed.
const EventPromptsListChanged = "prompts_list_changed"

// EventToolsListChanged represents tools list changed.
const EventToolsListChanged = "tools_list_changed"

// EventResourcesListChanged represents resources list changed.
const EventResourcesListChanged = "resources_list_changed"

// ReadResource reads an MCP resource.
func ReadResource(ctx any, cfg any, name, uri string) ([]Resource, error) {
	return nil, nil // Placeholder
}

// GetStates returns the MCP states.
func GetStates() map[string]ClientInfo {
	return make(map[string]ClientInfo)
}

// RefreshPrompts refreshes MCP prompts.
func RefreshPrompts(ctx any, name string) error {
	return nil // Placeholder
}

// RefreshTools refreshes MCP tools.
func RefreshTools(ctx any, cfg any, name string) error {
	return nil // Placeholder
}

// RefreshResources refreshes MCP resources.
func RefreshResources(ctx any, name string) error {
	return nil // Placeholder
}
