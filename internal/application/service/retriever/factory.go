package retriever

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// TenantStoreOwnership remains in service constructors so queued tasks and
// older integrations can be upgraded without changing their payload shape.
// ZealRAG has one built-in pgvector backend, so there is no store ownership
// decision at runtime.
type TenantStoreOwnership interface {
	StoreOwnedBy(ctx context.Context, storeID string, tenantID uint64) (bool, error)
}

type fixedStoreOwnership struct{}

// NewFixedStoreOwnership provides the compatibility dependency used by
// services that still deserialize a legacy vector_store_id field.
func NewFixedStoreOwnership() TenantStoreOwnership {
	return fixedStoreOwnership{}
}

func (fixedStoreOwnership) StoreOwnedBy(context.Context, string, uint64) (bool, error) {
	return true, nil
}

// CreateRetrieveEngineForKB always resolves ZealRAG's built-in pgvector
// keyword and vector retrievers. Legacy bindings are intentionally ignored.
func CreateRetrieveEngineForKB(
	_ context.Context,
	registry interfaces.RetrieveEngineRegistry,
	_ TenantStoreOwnership,
	_ uint64,
	_ *string,
) (*CompositeRetrieveEngine, error) {
	return NewCompositeRetrieveEngine(registry, postgresEngineParams())
}

// CreateRetrieveEngineFromPayload is the queued-task equivalent. Old payload
// fields remain accepted, but they cannot change the selected backend.
func CreateRetrieveEngineFromPayload(
	_ context.Context,
	registry interfaces.RetrieveEngineRegistry,
	_ TenantStoreOwnership,
	_ uint64,
	_ []types.RetrieverEngineParams,
	_ *string,
) (*CompositeRetrieveEngine, error) {
	return NewCompositeRetrieveEngine(registry, postgresEngineParams())
}

func postgresEngineParams() []types.RetrieverEngineParams {
	return []types.RetrieverEngineParams{
		{RetrieverType: types.KeywordsRetrieverType, RetrieverEngineType: types.PostgresRetrieverEngineType},
		{RetrieverType: types.VectorRetrieverType, RetrieverEngineType: types.PostgresRetrieverEngineType},
	}
}
