package retriever

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func fixedRegistry(t *testing.T) *RetrieveEngineRegistry {
	t.Helper()
	registry := NewRetrieveEngineRegistry().(*RetrieveEngineRegistry)
	require.NoError(t, registry.Register(newMock(types.PostgresRetrieverEngineType)))
	return registry
}

func TestCreateRetrieveEngineForKBIgnoresLegacyBinding(t *testing.T) {
	legacyStoreID := "legacy-store"
	ownership := &countingOwnership{}
	engine, err := CreateRetrieveEngineForKB(
		context.Background(), fixedRegistry(t), ownership, 99, &legacyStoreID)
	require.NoError(t, err)
	require.NotNil(t, engine)
	assert.True(t, engine.SupportRetriever(types.KeywordsRetrieverType))
	assert.True(t, engine.SupportRetriever(types.VectorRetrieverType))
	assert.Zero(t, ownership.calls)
}

func TestCreateRetrieveEngineFromPayloadUsesPostgres(t *testing.T) {
	engine, err := CreateRetrieveEngineFromPayload(
		context.Background(),
		fixedRegistry(t),
		NewFixedStoreOwnership(),
		1,
		[]types.RetrieverEngineParams{{
			RetrieverEngineType: types.RetrieverEngineType("milvus"),
			RetrieverType:       types.VectorRetrieverType,
		}},
		nil,
	)
	require.NoError(t, err)
	assert.True(t, engine.SupportRetriever(types.KeywordsRetrieverType))
	assert.True(t, engine.SupportRetriever(types.VectorRetrieverType))
}

type countingOwnership struct {
	calls int
}

func (o *countingOwnership) StoreOwnedBy(context.Context, string, uint64) (bool, error) {
	o.calls++
	return false, nil
}
