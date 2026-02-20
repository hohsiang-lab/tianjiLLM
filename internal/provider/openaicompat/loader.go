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
		p := NewFromConfig(cfg)
		provider.Register(name, p)
	}

	return nil
}
