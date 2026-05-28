package config

import (
	"flag"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	t.Setenv("MODEL", "deepseek-chat")
	t.Setenv("DEBUG", "true")

	cfg, err := loadWithFlagSet(flag.NewFlagSet("test", flag.ContinueOnError), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "sk-test-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-test-key")
	}
	if cfg.Model != "deepseek-chat" {
		t.Errorf("Model = %q, want %q", cfg.Model, "deepseek-chat")
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadMissingKey(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")

	_, err := loadWithFlagSet(flag.NewFlagSet("test", flag.ContinueOnError), []string{})
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	t.Setenv("MODEL", "")
	t.Setenv("DEBUG", "")

	cfg, err := loadWithFlagSet(flag.NewFlagSet("test", flag.ContinueOnError), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != DefaultModel {
		t.Errorf("Model = %q, want default %q", cfg.Model, DefaultModel)
	}
	if cfg.Debug {
		t.Error("Debug = true, want default false")
	}
	if !cfg.SkillsEnabled {
		t.Error("SkillsEnabled = false, want default true")
	}
}

func TestLoadFlagOverrides(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-env-key")
	t.Setenv("MODEL", "env-model")

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	cfg, err := loadWithFlagSet(fs, []string{"-key=sk-flag-key", "-model=flag-model", "-debug"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.APIKey != "sk-flag-key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "sk-flag-key")
	}
	if cfg.Model != "flag-model" {
		t.Errorf("Model = %q, want %q", cfg.Model, "flag-model")
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestLoadSkillsConfigFromEnv(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "sk-test-key")
	t.Setenv("DISCO_SKILLS", "0")
	t.Setenv("DISCO_SKILLS_DIR", "/tmp/custom-skills")

	cfg, err := loadWithFlagSet(flag.NewFlagSet("test", flag.ContinueOnError), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.SkillsEnabled {
		t.Fatal("SkillsEnabled = true, want false")
	}
	if len(cfg.SkillDirs) != 1 || cfg.SkillDirs[0] != "/tmp/custom-skills" {
		t.Fatalf("SkillDirs = %+v, want custom dir", cfg.SkillDirs)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_KEY_A", "value")
	if got := getEnv("TEST_KEY_A", "fallback"); got != "value" {
		t.Errorf("getEnv = %q, want %q", got, "value")
	}
	if got := getEnv("TEST_KEY_B", "fallback"); got != "fallback" {
		t.Errorf("getEnv = %q, want %q", got, "fallback")
	}
}

func TestGetEnvBool(t *testing.T) {
	tests := []struct {
		value    string
		fallback bool
		want     bool
	}{
		{"true", false, true},
		{"1", false, true},
		{"false", true, false},
		{"0", true, false},
		{"", true, true},
		{"invalid", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv("TEST_BOOL", tt.value)
			if got := getEnvBool("TEST_BOOL", tt.fallback); got != tt.want {
				t.Errorf("getEnvBool(%q, %v) = %v, want %v", tt.value, tt.fallback, got, tt.want)
			}
		})
	}
}
