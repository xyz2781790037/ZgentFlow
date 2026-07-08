package rerank

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/provider"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// Reranker defines the interface for document reranking
type Reranker interface {
	// Rerank reranks documents based on relevance to the query
	Rerank(ctx context.Context, query string, documents []string) ([]RankResult, error)

	// GetModelName returns the model name
	GetModelName() string

	// GetModelID returns the model ID
	GetModelID() string
}

type RankResult struct {
	Index          int          `json:"index"`
	Document       DocumentInfo `json:"document"`
	RelevanceScore float64      `json:"relevance_score"`
}

// Handles the RelevanceScore field by checking if RelevanceScore exists first, otherwise falls back to Score field
func (r *RankResult) UnmarshalJSON(data []byte) error {
	var temp struct {
		Index          int          `json:"index"`
		Document       DocumentInfo `json:"document"`
		RelevanceScore *float64     `json:"relevance_score"`
		Score          *float64     `json:"score"`
	}

	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal rank result: %w", err)
	}

	r.Index = temp.Index
	r.Document = temp.Document

	if temp.RelevanceScore != nil {
		r.RelevanceScore = *temp.RelevanceScore
	} else if temp.Score != nil {
		r.RelevanceScore = *temp.Score
	}

	return nil
}

type DocumentInfo struct {
	Text string `json:"text"`
}

// UnmarshalJSON handles both string and object formats for DocumentInfo
func (d *DocumentInfo) UnmarshalJSON(data []byte) error {
	// First try to unmarshal as a string
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		d.Text = text
		return nil
	}

	// If that fails, try to unmarshal as an object with text field
	var temp struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal DocumentInfo: %w", err)
	}

	d.Text = temp.Text
	return nil
}

type RerankerConfig struct {
	APIKey      string
	BaseURL     string
	ModelName   string
	Source      types.ModelSource
	ModelID     string
	Provider    string // Provider identifier: openai, aliyun, zhipu, siliconflow, jina, generic
	ExtraConfig map[string]string
	AppSecret   string
}

// ConfigFromModel 根据 types.Model 构造 RerankerConfig。
// 生产路径（从 DB 拉起）和测试连接路径（临时表单）共享这份映射。
// appSecret is the decrypted secondary credential used by providers such as LKEAP.
func ConfigFromModel(m *types.Model, appSecret string) *RerankerConfig {
	if m == nil {
		return nil
	}
	return &RerankerConfig{
		ModelID:     m.ID,
		APIKey:      m.Parameters.APIKey,
		BaseURL:     m.Parameters.BaseURL,
		ModelName:   m.Name,
		Source:      m.Source,
		Provider:    m.Parameters.Provider,
		ExtraConfig: m.Parameters.ExtraConfig,
		AppSecret:   appSecret,
	}
}

// NewReranker creates a reranker based on the configuration
func NewReranker(config *RerankerConfig) (Reranker, error) {
	r, err := newReranker(config)
	if err != nil {
		return r, err
	}
	if logger.LLMDebugEnabled() {
		r = &debugReranker{inner: r}
	}
	return wrapRerankerLangfuse(r, nil)
}

func newReranker(config *RerankerConfig) (Reranker, error) {
	// Use provider field if set, otherwise detect from URL using provider registry
	providerName := provider.ProviderName(config.Provider)
	if providerName == "" {
		providerName = provider.DetectProvider(config.BaseURL)
	}

	var (
		reranker Reranker
		err      error
	)
	switch providerName {
	case provider.ProviderAliyun:
		reranker, err = NewAliyunReranker(config)
	case provider.ProviderZhipu:
		reranker, err = NewZhipuReranker(config)
	case provider.ProviderJina:
		reranker, err = NewJinaReranker(config)
	case provider.ProviderNvidia:
		reranker, err = NewNvidiaReranker(config)
	case provider.ProviderLKEAP:
		reranker, err = NewLKEAPReranker(config)
	default:
		reranker, err = NewOpenAIReranker(config)
	}
	if err != nil {
		return nil, err
	}
	return reranker, nil
}
