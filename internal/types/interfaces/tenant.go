package interfaces

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// TenantService defines the tenant service interface
type TenantService interface {
	// GetTenantByID gets a tenant by ID
	GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error)
	// UpdateTenant updates a tenant
	UpdateTenant(ctx context.Context, tenant *types.Tenant) (*types.Tenant, error)
}

// TenantRepository defines the tenant repository interface
type TenantRepository interface {
	// CreateTenant creates a tenant
	CreateTenant(ctx context.Context, tenant *types.Tenant) error
	// GetTenantByID gets a tenant by ID
	GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error)
	// UpdateTenant updates a tenant
	UpdateTenant(ctx context.Context, tenant *types.Tenant) error
	// AdjustStorageUsed adjusts the storage used for a tenant
	AdjustStorageUsed(ctx context.Context, tenantID uint64, delta int64) error
}
