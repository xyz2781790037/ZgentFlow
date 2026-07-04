package repository

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrTenantNotFound = errors.New("workspace not found")

type tenantRepository struct {
	db *gorm.DB
}

func NewTenantRepository(db *gorm.DB) interfaces.TenantRepository {
	return &tenantRepository{db: db}
}

func (r *tenantRepository) CreateTenant(ctx context.Context, tenant *types.Tenant) error {
	return r.db.WithContext(ctx).Create(tenant).Error
}

func (r *tenantRepository) GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error) {
	var tenant types.Tenant
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&tenant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	return &tenant, nil
}

func (r *tenantRepository) UpdateTenant(ctx context.Context, tenant *types.Tenant) error {
	return r.db.WithContext(ctx).
		Model(&types.Tenant{}).
		Where("id = ?", tenant.ID).
		Updates(tenant).Error
}

func (r *tenantRepository) AdjustStorageUsed(ctx context.Context, tenantID uint64, delta int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var tenant types.Tenant
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&tenant, tenantID).Error; err != nil {
			return err
		}
		tenant.StorageUsed += delta
		if tenant.StorageUsed < 0 {
			logger.Errorf(ctx, "workspace storage used is negative %d: %d", tenant.ID, tenant.StorageUsed)
			tenant.StorageUsed = 0
		}
		return tx.Model(&types.Tenant{}).
			Where("id = ?", tenant.ID).
			Update("storage_used", tenant.StorageUsed).Error
	})
}
