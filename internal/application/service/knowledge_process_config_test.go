package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	werrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func processConfigBoolPtr(v bool) *bool {
	return &v
}

func TestResolveProcessConfig_OverridesChunkSize(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50},
	}
	overrides := &types.KnowledgeProcessOverrides{
		ChunkingConfig: &types.ChunkingConfig{ChunkSize: 2048},
	}
	eff := ResolveProcessConfig(kb, overrides)
	require.Equal(t, 2048, eff.ChunkingConfig.ChunkSize)
	require.Equal(t, 50, eff.ChunkingConfig.ChunkOverlap)
}

func TestResolveProcessConfig_OverrideTogglesParentChild(t *testing.T) {
	t.Parallel()

	// KB has parent-child on; override snapshot turns it off.
	kbOn := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, EnableParentChild: true},
	}
	effOff := ResolveProcessConfig(kbOn, &types.KnowledgeProcessOverrides{
		ChunkingConfig: &types.ChunkingConfig{ChunkSize: 512, EnableParentChild: false},
	})
	require.False(t, effOff.ChunkingConfig.EnableParentChild)

	// KB has parent-child off; override snapshot turns it on.
	kbOff := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, EnableParentChild: false},
	}
	effOn := ResolveProcessConfig(kbOff, &types.KnowledgeProcessOverrides{
		ChunkingConfig: &types.ChunkingConfig{ChunkSize: 512, EnableParentChild: true},
	})
	require.True(t, effOn.ChunkingConfig.EnableParentChild)
}

func TestResolveProcessConfig_NilOverridesUsesKBDefaults(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50},
		VLMConfig:      types.VLMConfig{Enabled: true, ModelID: "vlm-1"},
		QuestionGenerationConfig: &types.QuestionGenerationConfig{
			Enabled:       true,
			QuestionCount: 3,
		},
	}

	eff := ResolveProcessConfig(kb, nil)

	require.Equal(t, 512, eff.ChunkingConfig.ChunkSize)
	require.Equal(t, 50, eff.ChunkingConfig.ChunkOverlap)
	require.True(t, eff.EnableMultimodel)
	require.Equal(t, "vlm-1", eff.VLMConfig.ModelID)
	require.True(t, eff.QuestionGenerationConfig.Enabled)
	require.Equal(t, 3, eff.QuestionGenerationConfig.QuestionCount)
}

func TestBuildSplitterConfigFromChunking_UsesEffectiveChunkingConfig(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{ChunkSize: 512, ChunkOverlap: 50, Strategy: "token"},
	}
	overrides := &types.KnowledgeProcessOverrides{
		ChunkingConfig: &types.ChunkingConfig{ChunkSize: 1500, ChunkOverlap: 120, Strategy: "character"},
	}
	eff := ResolveProcessConfig(kb, overrides)
	cfg := buildSplitterConfigFromChunking(eff.ChunkingConfig)

	require.Equal(t, 1500, cfg.ChunkSize)
	require.Equal(t, 120, cfg.ChunkOverlap)
	require.Equal(t, "character", cfg.Strategy)
}

func TestEffectiveChunkingConfig_ResolveParserEngineFromOverrides(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{
			ParserEngineRules: []types.ParserEngineRule{
				{FileTypes: []string{"pdf"}, Engine: "builtin"},
			},
		},
	}
	overrides := &types.KnowledgeProcessOverrides{
		ParserEngineRules: []types.ParserEngineRule{
			{FileTypes: []string{"pdf"}, Engine: "mineru"},
		},
	}
	eff := ResolveProcessConfig(kb, overrides)
	require.Equal(t, "mineru", eff.ChunkingConfig.ResolveParserEngine("pdf"))
}

func TestResolveProcessConfig_ParserEngineRulesReplaced(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		ChunkingConfig: types.ChunkingConfig{
			ParserEngineRules: []types.ParserEngineRule{
				{FileTypes: []string{"pdf"}, Engine: "builtin"},
			},
		},
	}
	overrides := &types.KnowledgeProcessOverrides{
		ParserEngineRules: []types.ParserEngineRule{
			{FileTypes: []string{"docx"}, Engine: "custom"},
		},
	}
	eff := ResolveProcessConfig(kb, overrides)
	require.Len(t, eff.ChunkingConfig.ParserEngineRules, 1)
	require.Equal(t, []string{"docx"}, eff.ChunkingConfig.ParserEngineRules[0].FileTypes)
	require.Equal(t, "custom", eff.ChunkingConfig.ParserEngineRules[0].Engine)
}

func TestResolveProcessConfig_EnableMultimodelOverride(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		VLMConfig: types.VLMConfig{Enabled: true, ModelID: "vlm-1"},
	}
	overrides := &types.KnowledgeProcessOverrides{
		EnableMultimodel: processConfigBoolPtr(false),
	}
	eff := ResolveProcessConfig(kb, overrides)
	require.False(t, eff.EnableMultimodel)
}

func TestValidateProcessOverrides_NilOverrides(t *testing.T) {
	t.Parallel()

	err := ValidateProcessOverrides(context.Background(), &types.KnowledgeBase{}, nil, []string{"png"})
	require.NoError(t, err)
}

func TestValidateProcessOverrides_ImageRequiresVLM(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		VLMConfig: types.VLMConfig{Enabled: false},
	}
	err := ValidateProcessOverrides(context.Background(), kb, &types.KnowledgeProcessOverrides{}, []string{"png"})
	require.Error(t, err)
	var badReq *werrors.AppError
	require.ErrorAs(t, err, &badReq)
}

func TestValidateProcessOverrides_ImageWithEffectiveVLM(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		VLMConfig: types.VLMConfig{Enabled: false},
	}
	overrides := &types.KnowledgeProcessOverrides{
		VLMConfig: &types.VLMConfig{Enabled: true, ModelID: "vlm-1"},
	}
	err := ValidateProcessOverrides(context.Background(), kb, overrides, []string{"jpg"})
	require.NoError(t, err)
}

func TestValidateProcessOverrides_NonMediaFileTypes(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{}
	err := ValidateProcessOverrides(context.Background(), kb, &types.KnowledgeProcessOverrides{}, []string{"pdf", "txt"})
	require.NoError(t, err)
}

func TestValidateProcessOverrides_LocalImageStorage(t *testing.T) {
	t.Parallel()

	kb := &types.KnowledgeBase{
		VLMConfig: types.VLMConfig{Enabled: true, ModelID: "vlm-1"},
	}

	err := ValidateProcessOverrides(context.Background(), kb, &types.KnowledgeProcessOverrides{}, []string{"png"})
	require.NoError(t, err)
}
