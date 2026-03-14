// Package config provides configuration types for the UI.
package config

import (
	"charm.land/catwalk/pkg/catwalk"
	"github.com/machinus/cloud-agent/internal/config"
)

// UIConfig wraps the internal config for UI use.
type UIConfig struct {
	*config.Config
	ProviderColl ProviderCollection
	ModelConfigs map[SelectedModelType]ModelConfig
	RecentModel  map[SelectedModelType][]string
	MCPServers  map[string]interface{}
}

// MCP returns MCP servers.
func (c *UIConfig) MCP() map[string]interface{} {
	if c.MCPServers == nil {
		return make(map[string]interface{})
	}
	return c.MCPServers
}

// Options returns config options.
func (c *UIConfig) Options() interface{} {
	return nil
}

// InitializeAs returns the initialize as value.
func (c *UIConfig) InitializeAs() string {
	return ""
}

// TUI returns TUI options.
func (c *UIConfig) TUI() interface{} {
	return nil
}

// IsConfigured returns whether the app is configured.
func (c *UIConfig) IsConfigured() bool {
	// Check if we have a valid API key configured
	if c.Config != nil && c.Config.LLMAPIKey != "" {
		return true
	}
	// Also check if any provider has an API key
	if c.ProviderColl.seq != nil {
		for _, p := range c.ProviderColl.seq {
			if !p.Disable {
				return true
			}
		}
	}
	return false
}

// ProjectNeedsInitialization returns whether project needs initialization.
func ProjectNeedsInitialization() (bool, error) {
	return false, nil
}

// CompactMode returns compact mode setting.
func (c *UIConfig) CompactMode() bool {
	return false
}

// MarkProjectInitialized marks project as initialized.
func MarkProjectInitialized() error {
	return nil
}

// MarkProjectInitialized marks project as initialized (for UIConfig).
func (c *UIConfig) MarkProjectInitialized() error {
	return nil
}

// NewUIConfig creates a new UI config wrapper.
func NewUIConfig(cfg *config.Config) *UIConfig {
	uiConfig := &UIConfig{
		Config:       cfg,
		ProviderColl: ProviderCollection{seq: make(map[string]ProviderEntry)},
		ModelConfigs: make(map[SelectedModelType]ModelConfig),
		RecentModel:  make(map[SelectedModelType][]string),
		MCPServers:   make(map[string]interface{}),
	}

	// Add default providers for onboarding
	uiConfig.ProviderColl.seq["anthropic"] = ProviderEntry{
		Name: "Anthropic",
		Models: []ProviderModel{
			{
				ID:               "claude-sonnet-4-20250514",
				Name:             "Claude Sonnet 4",
				DefaultMaxTokens: 200000,
			},
			{
				ID:               "claude-3-5-sonnet-20241022",
				Name:             "Claude 3.5 Sonnet",
				DefaultMaxTokens: 200000,
			},
			{
				ID:               "claude-3-opus-20240229",
				Name:             "Claude 3 Opus",
				DefaultMaxTokens: 200000,
			},
		},
	}

	uiConfig.ProviderColl.seq["openai"] = ProviderEntry{
		Name: "OpenAI",
		Models: []ProviderModel{
			{
				ID:               "gpt-4o",
				Name:             "GPT-4o",
				DefaultMaxTokens: 128000,
			},
			{
				ID:               "o1",
				Name:             "o1",
				DefaultMaxTokens: 200000,
			},
		},
	}

	uiConfig.ProviderColl.seq["openrouter"] = ProviderEntry{
		Name: "OpenRouter",
		Models: []ProviderModel{
			{
				ID:               "anthropic/claude-sonnet-4",
				Name:             "Claude Sonnet 4",
				DefaultMaxTokens: 200000,
			},
			{
				ID:               "openai/gpt-4o",
				Name:             "GPT-4o",
				DefaultMaxTokens: 128000,
			},
		},
	}

	uiConfig.ProviderColl.seq["zai"] = ProviderEntry{
		Name: "Z.ai",
		Models: []ProviderModel{
			{
				ID:               "glm-4-plus",
				Name:             "GLM-4 Plus",
				DefaultMaxTokens: 128000,
			},
			{
				ID:               "glm-4-0520",
				Name:             "GLM-4 Turbo",
				DefaultMaxTokens: 128000,
			},
			{
				ID:               "glm-4.7",
				Name:             "GLM-4.7",
				DefaultMaxTokens: 128000,
			},
			{
				ID:               "glm-4-flash",
				Name:             "GLM-4 Flash",
				DefaultMaxTokens: 128000,
			},
		},
	}

	// Set up model configs
	uiConfig.ModelConfigs[ModelTypeLarge] = ModelConfig{
		Name:   "Claude Sonnet 4",
		Model:  "claude-sonnet-4-20250514",
		ReasoningLevels: []string{"high", "medium", "low"},
		CanReason: true,
	}

	uiConfig.ModelConfigs[ModelTypeSmall] = ModelConfig{
		Name:   "Claude 3.5 Sonnet",
		Model:  "claude-3-5-sonnet-20241022",
	}

	return uiConfig
}

// Config is an alias for the internal config type for direct use.
type Config = config.Config

// SelectedModel represents a selected model.
type SelectedModel struct {
	Provider        string `json:"provider"`
	Name            string `json:"name"`
	ID              string `json:"id"`
	Model           string `json:"model"`
	ReasoningEffort string `json:"reasoning_effort,omitempty"`
	MaxTokens       int    `json:"max_tokens,omitempty"`
}

// SelectedModelType represents the type of model selection.
type SelectedModelType int

const (
	ModelTypeLarge SelectedModelType = iota
	ModelTypeSmall
)

// Resolver represents a config resolver.
type Resolver struct {
	// Placeholder for resolver functionality
}

// ProviderConfig represents provider configuration.
type ProviderConfig struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Name    string          `json:"name"`
	BaseURL string          `json:"base_url"`
	APIKey  string          `json:"api_key"`
	Disable bool            `json:"disable"`
	Models  []ProviderModel `json:"models"`
}

// TestConnection tests the provider connection.
func (p *ProviderConfig) TestConnection() error {
	return nil // Placeholder
}

// GlobalConfigData represents global configuration data.
type GlobalConfigData struct {
	Providers map[string]ProviderConfig `json:"providers"`
}

// NewGlobalConfigData creates a new GlobalConfigData.
func NewGlobalConfigData() *GlobalConfigData {
	return &GlobalConfigData{
		Providers: make(map[string]ProviderConfig),
	}
}

// WrapConfig wraps an internal config in UIConfig.
func WrapConfig(cfg *config.Config) *UIConfig {
	return &UIConfig{Config: cfg}
}

// APIKey returns the API key.
func (c *UIConfig) APIKey() string {
	if c.Config == nil {
		return ""
	}
	return c.Config.LLMAPIKey
}

// Model returns the model name.
func (c *UIConfig) Model() string {
	if c.Config == nil {
		return ""
	}
	return c.Config.LLMModel
}

// WorkingDir returns the working directory.
func (c *UIConfig) WorkingDir() string {
	if c.Config == nil {
		return ""
	}
	return "."
}

// SessionID returns the session ID.
func (c *UIConfig) SessionID() string {
	return ""
}

// SetProviderAPIKey sets the API key for a provider.
func (c *UIConfig) SetProviderAPIKey(provider, key string) error {
	// Store the API key - in a real implementation this would save to config file
	// For now, we'll just mark it as configured
	if c.Config == nil {
		return nil
	}
	// Set the API key for the selected provider
	c.Config.LLMAPIKey = key
	c.Config.LLMBaseURL = getBaseURLForProvider(provider)
	c.Config.LLMModel = getDefaultModelForProvider(provider)
	return nil
}

// getBaseURLForProvider returns the base URL for a given provider.
func getBaseURLForProvider(provider string) string {
	switch provider {
	case "anthropic":
		return "https://api.anthropic.com"
	case "openai":
		return "https://api.openai.com/v1"
	case "openrouter":
		return "https://openrouter.ai/api/v1"
	default:
		return ""
	}
}

// getDefaultModelForProvider returns the default model for a given provider.
func getDefaultModelForProvider(provider string) string {
	switch provider {
	case "anthropic":
		return "claude-sonnet-4-20250514"
	case "openai":
		return "gpt-4o"
	case "openrouter":
		return "anthropic/claude-sonnet-4"
	default:
		return ""
	}
}

// Resolver returns the config resolver.
func (c *UIConfig) Resolver() *Resolver {
	return &Resolver{}
}

// AgentConfig represents an agent configuration.
type AgentConfig struct {
	Model string `json:"model"`
}

// ModelConfig represents a model configuration.
type ModelConfig struct {
	Name            string   `json:"name"`
	Model           string   `json:"model"`
	Provider        string   `json:"provider"`
	CanReason       bool     `json:"can_reason"`
	ReasoningLevels []string `json:"reasoning_levels,omitempty"`
	ReasoningEffort string   `json:"reasoning_effort,omitempty"`
	ContextWindow   int      `json:"context_window,omitempty"`
	SupportsImages  bool     `json:"supports_images"`
	Think           bool     `json:"think,omitempty"`
}

// Agents is a map of agent configurations.
type Agents map[string]AgentConfig

// Agent constants
const (
	AgentCoder = "coder"
)

// ModelsMap is a map of model configurations.
type ModelsMap map[string]ModelConfig

// Agents returns the agents configuration.
func (c *UIConfig) Agents() Agents {
	return make(Agents)
}

// Models returns the models configuration.
func (c *UIConfig) Models() map[SelectedModelType]ModelConfig {
	if c.ModelConfigs == nil {
		return make(map[SelectedModelType]ModelConfig)
	}
	return c.ModelConfigs
}

// ProvidersConfig represents the providers configuration.
type ProvidersConfig struct {
	providers map[string]ProviderEntry
}

// ProviderEntry represents a single provider entry.
type ProviderEntry struct {
	Name    string            `json:"name"`
	Disable bool              `json:"disable"`
	Models  []ProviderModel   `json:"models"`
}

// ProviderModel represents a model within a provider.
type ProviderModel struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	DefaultReasoningEffort string          `json:"default_reasoning_effort,omitempty"`
	DefaultMaxTokens       int             `json:"default_max_tokens,omitempty"`
}

// Seq2 returns a sequenced list of provider entries.
func (p *ProvidersConfig) Seq2() map[string]ProviderEntry {
	return p.providers
}

// ProvidersConfig returns the providers configuration.
func (c *UIConfig) ProvidersConfig() *ProvidersConfig {
	return &ProvidersConfig{
		providers: c.ProviderColl.seq,
	}
}

// ToProvider converts a provider entry to a catwalk provider.
func (p *ProviderEntry) ToProvider() catwalk.Provider {
	return catwalk.Provider{
		ID:   catwalk.InferenceProvider(p.Name),
		Name: p.Name,
	}
}

// GetProviderForModel returns the provider for a given model.
func (c *UIConfig) GetProviderForModel(model string) *ProviderConfig {
	return nil // Placeholder
}

// GetModelByType returns the model configuration for a given model type.
func (c *UIConfig) GetModelByType(modelType string) *ModelConfig {
	return nil // Placeholder
}

// ProviderCollection returns the providers collection.
func (c *UIConfig) ProviderCollection() *ProviderCollection {
	return &c.ProviderColl
}

// Providers returns the providers collection (for dialog compatibility).
func (c *UIConfig) Providers() *ProviderCollection {
	return &c.ProviderColl
}

// ProviderCollection represents a collection of providers.
type ProviderCollection struct {
	seq map[string]ProviderEntry
}

// Get returns a provider by ID.
func (p *ProviderCollection) Get(id string) (ProviderConfig, bool) {
	if p.seq == nil {
		return ProviderConfig{}, false
	}
	entry, ok := p.seq[id]
	if !ok {
		return ProviderConfig{}, false
	}
	return ProviderConfig{
		ID:      id,
		Name:    entry.Name,
		Disable: entry.Disable,
		Models:  entry.Models,
	}, true
}

// Seq2 returns a map of provider entries.
func (p *ProviderCollection) Seq2() map[string]ProviderEntry {
	if p.seq == nil {
		return make(map[string]ProviderEntry)
	}
	return p.seq
}

// SelectedModelTypeLarge represents the large model type.
const SelectedModelTypeLarge = SelectedModelType(ModelTypeLarge)

// SelectedModelTypeSmall represents the small model type.
const SelectedModelTypeSmall = SelectedModelType(ModelTypeSmall)

// GetModel returns a model by provider and model name.
func (c *UIConfig) GetModel(provider, model string) *catwalk.Model {
	// Find the provider entry
	if c.ProviderColl.seq == nil {
		return nil
	}
	entry, ok := c.ProviderColl.seq[provider]
	if !ok {
		return nil
	}

	// Find the model in the provider's models list
	for _, m := range entry.Models {
		if m.ID == model || m.Name == model {
			// Convert to catwalk.Model
			return &catwalk.Model{
				ID:               m.ID,
				Name:             m.Name,
				ContextWindow:    int64(m.DefaultMaxTokens),
				DefaultMaxTokens: int64(m.DefaultMaxTokens),
			}
		}
	}

	// If not found in the list, create a basic model entry
	return &catwalk.Model{
		ID:   model,
		Name: model,
	}
}

// GetProviders returns a list of catwalk providers from the config.
func GetProviders(cfg *UIConfig) ([]catwalk.Provider, error) {
	if cfg == nil || cfg.ProviderColl.seq == nil {
		return []catwalk.Provider{}, nil
	}

	var providers []catwalk.Provider
	for id, entry := range cfg.ProviderColl.seq {
		providers = append(providers, catwalk.Provider{
			ID:   catwalk.InferenceProvider(id),
			Name: entry.Name,
		})
	}
	return providers, nil
}

// SetConfigField sets a config field.
func (c *UIConfig) SetConfigField(field string, value interface{}) error {
	return nil // Placeholder
}

// RecentModels returns recent models.
func (c *UIConfig) RecentModels() map[SelectedModelType][]string {
	if c.RecentModel == nil {
		return make(map[SelectedModelType][]string)
	}
	return c.RecentModel
}

// UpdatePreferredModel updates the preferred model for a given model type.
func (c *UIConfig) UpdatePreferredModel(modelType SelectedModelType, model interface{}) error {
	// Store the selected model preference
	if c.ModelConfigs == nil {
		c.ModelConfigs = make(map[SelectedModelType]ModelConfig)
	}

	// Handle different model input formats
	switch m := model.(type) {
	case ModelConfig:
		c.ModelConfigs[modelType] = m
	case SelectedModel:
		// Handle SelectedModel from dialog
		cfg := ModelConfig{
			Name:     m.Name,
			Model:    m.Model,
			Provider: m.Provider,
		}
		c.ModelConfigs[modelType] = cfg
	case map[string]interface{}:
		// Handle map format from dialog
		cfg := ModelConfig{}
		if name, ok := m["name"].(string); ok {
			cfg.Name = name
		}
		if model, ok := m["model"].(string); ok {
			cfg.Model = model
		}
		if provider, ok := m["provider"].(string); ok {
			cfg.Provider = provider
		}
		if canReason, ok := m["can_reason"].(bool); ok {
			cfg.CanReason = canReason
		}
		if reasoningLevels, ok := m["reasoning_levels"].([]string); ok {
			cfg.ReasoningLevels = reasoningLevels
		}
		c.ModelConfigs[modelType] = cfg
	}

	// Update the recent models list
	if c.RecentModel == nil {
		c.RecentModel = make(map[SelectedModelType][]string)
	}
	if cfg, ok := c.ModelConfigs[modelType]; ok {
		c.RecentModel[modelType] = []string{cfg.Name}
	}

	return nil
}

// ImportCopilot imports GitHub Copilot configuration.
func (c *UIConfig) ImportCopilot() error {
	// Placeholder
	return nil
}

// SetCompactMode sets the compact mode setting.
func (c *UIConfig) SetCompactMode(compact bool) error {
	// Placeholder
	return nil
}

// SetupAgents sets up the agent configurations.
func (c *UIConfig) SetupAgents() error {
	// Placeholder
	return nil
}
