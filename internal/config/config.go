package config

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration.
type Config struct {
	APIKey        string
	Model         string
	Debug         bool
	SystemPrompt  string
	RemainingArgs []string
}

// Default values.
const (
	DefaultModel = "deepseek-v4-flash"
)

// Load reads configuration from .env files, environment variables, and flags.
func Load() (*Config, error) {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	return loadWithFlagSet(fs, os.Args[1:])
}

func loadWithFlagSet(fs *flag.FlagSet, args []string) (*Config, error) {
	// Attempt to load .env file if present; ignore errors.
	_ = godotenv.Load()

	cfg := &Config{
		APIKey: os.Getenv("DEEPSEEK_API_KEY"),
		Model:  getEnv("MODEL", DefaultModel),
		Debug:  getEnvBool("DEBUG", false),
	}

	// Parse flags.
	var (
		apiKey = fs.String("key", "", "DeepSeek API key (overrides env)")
		model  = fs.String("model", "", "Model name (overrides env)")
		debug  = fs.Bool("debug", false, "Enable debug mode")
	)
	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	if *apiKey != "" {
		cfg.APIKey = *apiKey
	}
	if *model != "" {
		cfg.Model = *model
	}
	if *debug {
		cfg.Debug = true
	}
	cfg.RemainingArgs = fs.Args()

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("DEEPSEEK_API_KEY is required (set env var or use -key flag)")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
