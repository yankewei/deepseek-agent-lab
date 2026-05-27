package llm

import (
	"os"

	"github.com/sashabaranov/go-openai"
)

// NewClient creates an OpenAI-compatible client configured for DeepSeek.
func NewClient(apiKey string) *openai.Client {
	config := openai.DefaultConfig(apiKey)
	// Use DEEPSEEK_BASE_URL to avoid collision with OPENAI_BASE_URL which may
	// be set for other services in the user's environment.
	if baseURL := os.Getenv("DEEPSEEK_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
	} else {
		config.BaseURL = "https://api.deepseek.com/v1"
	}
	return openai.NewClientWithConfig(config)
}
