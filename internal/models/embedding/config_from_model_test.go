package embedding

import (
	"testing"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func TestConfigFromModel(t *testing.T) {
	m := &types.Model{
		ID:     "emb-1",
		Name:   "text-embedding-3-small",
		Source: types.ModelSourceRemote,
		Parameters: types.ModelParameters{
			BaseURL:  "https://api.example.com/v1",
			APIKey:   "sk-xxx",
			Provider: "openai",
			EmbeddingParameters: types.EmbeddingParameters{
				Dimension:                 1536,
				TruncatePromptTokens:      512,
				SupportsDimensionOverride: true,
			},
			ExtraConfig: map[string]string{"region": "us-east"},
		},
	}

	cfg := ConfigFromModel(m)
	if cfg.ModelID != "emb-1" || cfg.ModelName != "text-embedding-3-small" {
		t.Errorf("identity mismatch: %+v", cfg)
	}
	if cfg.Dimensions != 1536 || cfg.TruncatePromptTokens != 512 {
		t.Errorf("embedding params mismatch: %+v", cfg)
	}
	if !cfg.SupportsDimensionOverride {
		t.Errorf("SupportsDimensionOverride not propagated: %+v", cfg)
	}
	if cfg.ExtraConfig["region"] != "us-east" {
		t.Errorf("ExtraConfig not propagated: %+v", cfg.ExtraConfig)
	}
}
