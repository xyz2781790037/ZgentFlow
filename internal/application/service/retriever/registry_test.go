package retriever

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xyz2781790037/ZealRAG/internal/models/embedding"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

type mockEngineService struct {
	engineType types.RetrieverEngineType
}

func (m *mockEngineService) EngineType() types.RetrieverEngineType { return m.engineType }
func (m *mockEngineService) Retrieve(context.Context, types.RetrieveParams) ([]*types.RetrieveResult, error) {
	return nil, nil
}
func (m *mockEngineService) Support() []types.RetrieverType {
	return []types.RetrieverType{types.KeywordsRetrieverType, types.VectorRetrieverType}
}
func (m *mockEngineService) Index(context.Context, embedding.Embedder, *types.IndexInfo, []types.RetrieverType) error {
	return nil
}
func (m *mockEngineService) BatchIndex(context.Context, embedding.Embedder, []*types.IndexInfo, []types.RetrieverType) error {
	return nil
}
func (m *mockEngineService) EstimateStorageSize(context.Context, embedding.Embedder, []*types.IndexInfo, []types.RetrieverType) int64 {
	return 0
}
func (m *mockEngineService) CopyIndices(context.Context, string, map[string]string, map[string]string, string, int, string) error {
	return nil
}
func (m *mockEngineService) DeleteByChunkIDList(context.Context, []string, int, string) error {
	return nil
}
func (m *mockEngineService) DeleteBySourceIDList(context.Context, []string, int, string) error {
	return nil
}
func (m *mockEngineService) DeleteByKnowledgeIDList(context.Context, []string, int, string) error {
	return nil
}
func (m *mockEngineService) BatchUpdateChunkEnabledStatus(context.Context, map[string]bool) error {
	return nil
}
func (m *mockEngineService) BatchUpdateChunkTagID(context.Context, map[string]string) error {
	return nil
}

func newMock(engineType types.RetrieverEngineType) interfaces.RetrieveEngineService {
	return &mockEngineService{engineType: engineType}
}

func TestRegistryUsesSinglePostgresEngine(t *testing.T) {
	registry := NewRetrieveEngineRegistry()
	require.NoError(t, registry.Register(newMock(types.PostgresRetrieverEngineType)))

	engine, err := registry.GetRetrieveEngineService(types.PostgresRetrieverEngineType)
	require.NoError(t, err)
	assert.Equal(t, types.PostgresRetrieverEngineType, engine.EngineType())
	assert.Len(t, registry.GetAllRetrieveEngineServices(), 1)

	err = registry.Register(newMock(types.PostgresRetrieverEngineType))
	assert.ErrorContains(t, err, "already registered")
}

func TestRegistryRejectsOtherEngines(t *testing.T) {
	registry := NewRetrieveEngineRegistry()
	err := registry.Register(newMock(types.RetrieverEngineType("milvus")))
	assert.ErrorContains(t, err, "only supports the postgres")
	assert.Empty(t, registry.GetAllRetrieveEngineServices())
}
