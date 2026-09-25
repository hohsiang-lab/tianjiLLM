package openaicompat

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/praxisllmlab/tianjiLLM/internal/provider"
)

// LoadProviders reads a providers.json file and registers each provider
// in the global provider registry.
func LoadProviders(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read providers.json: %w", err)
	}

	var file ProvidersFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("parse providers.json: %w", err)
	}

	for name, cfg := range file.Providers {
		cfg.Name = name

		// Preserve first-class provider behavior when an older providers.json
		// still contains the same name. Apply its configured base URL through
		// the provider's clone hook instead of replacing it with the generic
		// OpenAI-compatible implementation.
		if existing, err := provider.Get(name); err == nil {
			if configurable, ok := existing.(provider.BaseURLConfigurable); ok {
				if cfg.BaseURL != "" {
					existing = configurable.WithBaseURL(cfg.BaseURL)
				}
				provider.Register(name, newConfiguredProvider(existing, cfg))
				continue
			}
		}

		p := NewFromConfig(cfg)
		provider.Register(name, p)
	}

	return nil
}
