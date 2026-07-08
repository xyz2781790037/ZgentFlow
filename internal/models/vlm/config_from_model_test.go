package vlm

import (
	"testing"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func TestConfigFromModel_RemoteDefaultsToOpenAI(t *testing.T) {
	m := &types.Model{
		ID:     "v1",
		Name:   "gpt-4o",
		Source: types.ModelSourceRemote,
		Parameters: types.ModelParameters{
			BaseURL:     "https://api.example.com/v1",
			APIKey:      "sk",
			Provider:    "openai",
			ExtraConfig: map[string]string{"x": "y"},
		},
	}
	cfg := ConfigFromModel(m)
	if cfg.InterfaceType != "openai" {
		t.Errorf("expected openai default for remote, got %q", cfg.InterfaceType)
	}
	if cfg.Extra["x"] != "y" {
		t.Errorf("ExtraConfig not propagated as Extra: %+v", cfg.Extra)
	}
}

func TestConfigFromModel_LocalDefaultsToOllama(t *testing.T) {
	m := &types.Model{
		Name:   "qwen2-vl",
		Source: types.ModelSourceLocal,
	}
	cfg := ConfigFromModel(m)
	if cfg.InterfaceType != "ollama" {
		t.Errorf("expected ollama default for local, got %q", cfg.InterfaceType)
	}
}

func TestConfigFromModel_RespectsExplicitInterface(t *testing.T) {
	m := &types.Model{
		Name:   "qwen2-vl",
		Source: types.ModelSourceRemote,
		Parameters: types.ModelParameters{
			InterfaceType: "ollama",
		},
	}
	if got := ConfigFromModel(m).InterfaceType; got != "ollama" {
		t.Errorf("expected explicit interface to win, got %q", got)
	}
}
