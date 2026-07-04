package repository

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"gorm.io/gorm"
)

type modelAPIConfigRepository struct {
	db *gorm.DB
}

func NewModelAPIConfigRepository(db *gorm.DB) interfaces.ModelAPIConfigRepository {
	return &modelAPIConfigRepository{db: db}
}

func (r *modelAPIConfigRepository) Create(ctx context.Context, config *types.ModelAPIConfig) error {
	return r.db.WithContext(ctx).Create(config).Error
}

func (r *modelAPIConfigRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.ModelAPIConfig, error) {
	var config types.ModelAPIConfig
	if err := r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *modelAPIConfigRepository) GetByName(
	ctx context.Context, tenantID uint64, provider, name, excludeID string,
) (*types.ModelAPIConfig, error) {
	var config types.ModelAPIConfig
	query := r.db.WithContext(ctx).Where(
		"tenant_id = ? AND provider = ? AND name = ?", tenantID, provider, name,
	)
	if excludeID != "" {
		query = query.Where("id <> ?", excludeID)
	}
	if err := query.First(&config).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &config, nil
}

func (r *modelAPIConfigRepository) List(ctx context.Context, tenantID uint64) ([]*types.ModelAPIConfig, error) {
	var configs []*types.ModelAPIConfig
	if err := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).
		Order("provider ASC, created_at ASC").Find(&configs).Error; err != nil {
		return nil, err
	}
	for _, config := range configs {
		count, err := r.CountModelReferences(ctx, tenantID, config.ID)
		if err != nil {
			return nil, err
		}
		config.ModelCount = count
	}
	return configs, nil
}

func (r *modelAPIConfigRepository) Update(ctx context.Context, config *types.ModelAPIConfig) error {
	return r.db.WithContext(ctx).Model(&types.ModelAPIConfig{}).
		Where("id = ? AND tenant_id = ?", config.ID, config.TenantID).
		Select("name", "provider", "api_key", "updated_at").Updates(config).Error
}

func (r *modelAPIConfigRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where("id = ? AND tenant_id = ?", id, tenantID).
		Delete(&types.ModelAPIConfig{}).Error
}

func (r *modelAPIConfigRepository) CountModelReferences(ctx context.Context, tenantID uint64, id string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.Model{}).
		Where("tenant_id = ? AND api_config_id = ?", tenantID, id).Count(&count).Error
	return count, err
}
