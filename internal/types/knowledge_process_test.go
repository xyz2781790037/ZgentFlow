package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func boolPtr(v bool) *bool {
	return &v
}

func TestKnowledgeProcessOverridesRoundtrip(t *testing.T) {
	k := &Knowledge{}
	overrides := &KnowledgeProcessOverrides{
		EnableMultimodel: boolPtr(true),
		ChunkingConfig:   &ChunkingConfig{ChunkSize: 1024},
	}
	require.NoError(t, k.SetProcessOverrides(overrides))
	got, err := k.ProcessOverrides()
	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, *got.EnableMultimodel)
	require.Equal(t, 1024, got.ChunkingConfig.ChunkSize)
}
