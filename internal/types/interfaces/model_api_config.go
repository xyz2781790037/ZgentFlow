package interfaces

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

type ModelAPIConfigRepository interface {
	Create(ctx context.Context, config *types.ModelAPIConfig) error
	GetByID(ctx context.Context, tenantID uint64, id string) (*types.ModelAPIConfig, error)
	GetByName(ctx context.Context, tenantID uint64, provider, name, excludeID string) (*types.ModelAPIConfig, error)
	List(ctx context.Context, tenantID uint64) ([]*types.ModelAPIConfig, error)
	Update(ctx context.Context, config *types.ModelAPIConfig) error
	Delete(ctx context.Context, tenantID uint64, id string) error
	CountModelReferences(ctx context.Context, tenantID uint64, id string) (int64, error)
}

type ModelAPIConfigService interface {
	Create(ctx context.Context, name, provider, apiKey string) (*types.ModelAPIConfig, error)
	Get(ctx context.Context, id string) (*types.ModelAPIConfig, error)
	List(ctx context.Context) ([]*types.ModelAPIConfig, error)
	Update(ctx context.Context, id, name, provider string, apiKey *string) (*types.ModelAPIConfig, error)
	Delete(ctx context.Context, id string) (int64, error)
}
