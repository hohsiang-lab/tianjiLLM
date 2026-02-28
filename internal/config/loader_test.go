package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMinimalConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list:
  - model_name: gpt-4
    tianji_params:
      model: gpt-4
general_settings:
  master_key: sk-test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.GeneralSettings.MasterKey != "sk-test" {
		t.Fatalf("master_key: got %q, want sk-test", cfg.GeneralSettings.MasterKey)
	}
	if cfg.GeneralSettings.Port != 4000 {
		t.Fatalf("port: got %d, want 4000 (default)", cfg.GeneralSettings.Port)
	}
	if len(cfg.ModelList) != 1 {
		t.Fatalf("model_list: got %d, want 1", len(cfg.ModelList))
	}
	if cfg.ModelList[0].ModelName != "gpt-4" {
		t.Fatalf("model_name: got %q", cfg.ModelList[0].ModelName)
	}
}

func TestLoadWithEnvVars(t *testing.T) {
	os.Setenv("TEST_MASTER_KEY", "from-env")
	defer os.Unsetenv("TEST_MASTER_KEY")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: os.environ/TEST_MASTER_KEY
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GeneralSettings.MasterKey != "from-env" {
		t.Fatalf("got %q, want from-env", cfg.GeneralSettings.MasterKey)
	}
}

func TestLoadWithEnvironmentVariablesSection(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
environment_variables:
  MY_CUSTOM_VAR: hello_world
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("MY_CUSTOM_VAR") != "hello_world" {
		t.Fatalf("env var not set: got %q", os.Getenv("MY_CUSTOM_VAR"))
	}
	os.Unsetenv("MY_CUSTOM_VAR")
}

func TestLoadInvalidPath(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(cfgPath, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(cfgPath)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestLoadWithOverflowFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
  unknown_field: value
unknown_top_level: true
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Overflow) == 0 {
		t.Fatal("expected overflow fields")
	}
}

func TestLoadWithCacheParams(t *testing.T) {
	os.Setenv("TEST_CACHE_PW", "cachepw")
	defer os.Unsetenv("TEST_CACHE_PW")

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "proxy_config.yaml")
	content := `
model_list: []
general_settings:
  master_key: test
tianji_settings:
  cache: true
  cache_params:
    type: redis
    password: os.environ/TEST_CACHE_PW
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.TianjiSettings.CacheParams == nil {
		t.Fatal("cache_params should not be nil")
	}
	if cfg.TianjiSettings.CacheParams.Password != "cachepw" {
		t.Fatalf("cache password: got %q", cfg.TianjiSettings.CacheParams.Password)
	}
}

func TestValidate(t *testing.T) {
	cfg := &ProxyConfig{
		Overflow: map[string]any{"unknown": true},
		ModelList: []ModelConfig{
			{
				ModelName: "test",
				TianjiParams: TianjiParams{
					Overflow: map[string]any{"custom_param": "val"},
				},
			},
		},
		Guardrails: []GuardrailConfig{
			{GuardrailName: "g1", Overflow: map[string]any{"x": 1}},
		},
		PassThroughEndpoints: []PassThroughEndpoint{
			{Path: "/p", Overflow: map[string]any{"y": 2}},
		},
		RouterSettings: &RouterSettings{
			Overflow: map[string]any{"z": 3},
		},
		TianjiSettings: TianjiSettings{
			CacheParams: &CacheParams{
				Overflow: map[string]any{"w": 4},
			},
		},
	}
	// Should not panic
	Validate(cfg)
}
