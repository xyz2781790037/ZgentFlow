package vlm

import (
	"context"
	"fmt"
	"strings"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/provider"
	"github.com/xyz2781790037/ZealRAG/internal/models/utils/ollama"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// VLM defines the interface for Vision Language Model operations.
type VLM interface {
	// Predict sends one or more images with a text prompt to the VLM and returns the generated text.
	Predict(ctx context.Context, imgBytes [][]byte, prompt string) (string, error)

	GetModelName() string
	GetModelID() string
}

// Config holds the configuration needed to create a VLM instance.
type Config struct {
	Source        types.ModelSource
	BaseURL       string
	ModelName     string
	APIKey        string
	ModelID       string
	InterfaceType string // "ollama" or "openai" (default)
	Provider      string
	Extra         map[string]any
}

// ConfigFromModel 根据 types.Model 构造 vlm.Config。
// 生产路径（从 DB 拉起）和测试连接路径（临时表单）共享这份映射。
// InterfaceType 会根据 source / 模型参数自动回退到合理默认值。
func ConfigFromModel(m *types.Model) *Config {
	if m == nil {
		return nil
	}
	ifType := m.Parameters.InterfaceType
	if ifType == "" {
		if m.Source == types.ModelSourceLocal {
			ifType = "ollama"
		} else {
			ifType = "openai"
		}
	}
	return &Config{
		ModelID:       m.ID,
		APIKey:        m.Parameters.APIKey,
		BaseURL:       m.Parameters.BaseURL,
		ModelName:     m.Name,
		Source:        m.Source,
		InterfaceType: ifType,
		Provider:      m.Parameters.Provider,
		Extra:         stringMapToAnyMap(m.Parameters.ExtraConfig),
	}
}

func stringMapToAnyMap(in map[string]string) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// NewVLM creates a VLM instance based on the provided configuration.
func NewVLM(config *Config, ollamaService *ollama.OllamaService) (VLM, error) {
	v, err := newVLM(config, ollamaService)
	if err != nil {
		return v, err
	}
	if logger.LLMDebugEnabled() {
		v = &debugVLM{inner: v}
	}
	return wrapVLMLangfuse(v, nil)
}

func newVLM(config *Config, ollamaService *ollama.OllamaService) (VLM, error) {
	ifType := strings.ToLower(config.InterfaceType)

	if ifType == "ollama" || config.Source == types.ModelSourceLocal {
		return NewOllamaVLM(config, ollamaService)
	}

	providerName := provider.ProviderName(config.Provider)
	if providerName == "" {
		providerName = provider.DetectProvider(config.BaseURL)
	}
	return NewRemoteAPIVLM(config)
}

// NewVLMFromLegacyConfig creates a VLM from a legacy VLMConfig (inline BaseURL/APIKey/ModelName).
func NewVLMFromLegacyConfig(vlmCfg types.VLMConfig, ollamaService *ollama.OllamaService) (VLM, error) {
	if !vlmCfg.IsEnabled() {
		return nil, fmt.Errorf("VLM config is not enabled")
	}

	ifType := vlmCfg.InterfaceType
	if ifType == "" {
		ifType = "openai"
	}

	source := types.ModelSourceRemote
	if strings.EqualFold(ifType, "ollama") {
		source = types.ModelSourceLocal
	}

	return NewVLM(&Config{
		Source:        source,
		BaseURL:       vlmCfg.BaseURL,
		ModelName:     vlmCfg.ModelName,
		APIKey:        vlmCfg.APIKey,
		InterfaceType: ifType,
	}, ollamaService)
}
