package config

import (
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	// Server
	Host string
	Port int

	// Database
	DatabaseURL string

	// LLM API
	LLMBaseURL    string
	LLMAPIKey     string
	LLMModel      string
	LLMTimeout    time.Duration

	// Security
	AuthToken         string
	MaxExecutionTime  time.Duration
	EnableSandbox     bool

	// Memory
	EnableMemory      bool
	EmbeddingModel    string
	MaxMemories       int

	// Subagents
	EnableSubagents   bool
}

// Load loads configuration from environment variables with sensible defaults
// Automatically loads .env file if it exists
func Load() *Config {
	// Try to load .env file from multiple locations (ignore error if file doesn't exist)
	_ = godotenv.Load() // Current directory
	_ = godotenv.Load("../.env") // Parent directory
	_ = godotenv.Load("../../.env") // Two levels up

	return &Config{
		Host:              getEnv("HOST", "0.0.0.0"),
		Port:              getEnvInt("PORT", 8080),
		DatabaseURL:       getEnv("DATABASE_URL", "./data/machinus.db"),
		LLMBaseURL:        getEnv("LLM_BASE_URL", "https://api.z.ai/api/coding/paas/v4"),
		LLMAPIKey:         getEnv("LLM_API_KEY", ""),
		LLMModel:          getEnv("LLM_MODEL", "glm-4.7"),
		LLMTimeout:        getEnvDuration("LLM_TIMEOUT", 30*time.Second),
		AuthToken:         getEnv("AUTH_TOKEN", "dev-token"),
		MaxExecutionTime:  getEnvDuration("MAX_EXECUTION_TIME", 30*time.Second),
		EnableSandbox:     getEnvBool("ENABLE_SANDBOX", true),
		EnableMemory:      getEnvBool("ENABLE_MEMORY", true),
		EmbeddingModel:    getEnv("EMBEDDING_MODEL", "text-embedding-3-small"),
		MaxMemories:       getEnvInt("MAX_MEMORIES", 3),
		EnableSubagents:   getEnvBool("ENABLE_SUBAGENTS", false),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

// MarkProjectInitialized marks the project as initialized.
func MarkProjectInitialized() error {
	return nil
}
