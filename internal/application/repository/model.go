package repository

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"gorm.io/gorm"
)

// modelRepository implements the model repository interface
type modelRepository struct {
	db *gorm.DB
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *gorm.DB) interfaces.ModelRepository {
	return &modelRepository{db: db}
}

// Create creates a new model
func (r *modelRepository) Create(ctx context.Context, m *types.Model) error {
	return r.db.WithContext(ctx).Create(m).Error
}

// GetByID retrieves a model by ID
func (r *modelRepository) GetByID(ctx context.Context, tenantID uint64, id string) (*types.Model, error) {
	var m types.Model
	if err := r.db.WithContext(ctx).Where("id = ?", id).Where(
		"(tenant_id = ? OR is_builtin = true)", tenantID,
	).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

// List lists models with optional filtering
func (r *modelRepository) List(
	ctx context.Context, tenantID uint64, modelType types.ModelType, source types.ModelSource,
) ([]*types.Model, error) {
	var models []*types.Model
	query := r.db.WithContext(ctx).Where(
		"(tenant_id = ? OR is_builtin = true)", tenantID,
	)

	if modelType != "" {
		query = query.Where("type = ?", modelType)
	}

	if source != "" {
		query = query.Where("source = ?", source)
	}

	if err := query.Find(&models).Error; err != nil {
		return nil, err
	}

	return models, nil
}

// Update updates a model
func (r *modelRepository) Update(ctx context.Context, m *types.Model) error {
	// Use Select to explicitly update all fields, including zero values like false
	return r.db.WithContext(ctx).Debug().Model(&types.Model{}).Where(
		"id = ? AND tenant_id = ?", m.ID, m.TenantID,
	).Select("*").Updates(m).Error
}

// Delete deletes a model
func (r *modelRepository) Delete(ctx context.Context, tenantID uint64, id string) error {
	return r.db.WithContext(ctx).Where(
		"id = ? AND tenant_id = ?", id, tenantID,
	).Delete(&types.Model{}).Error
}

func (r *modelRepository) GetDefaultByType(
	ctx context.Context,
	tenantID uint64,
	modelType types.ModelType,
) (*types.Model, error) {
	var model types.Model
	err := r.db.WithContext(ctx).
		Where("(tenant_id = ? OR is_builtin = true) AND type = ? AND is_default = ? AND status = ?",
			tenantID, modelType, true, types.ModelStatusActive).
		First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &model, nil
}

func (r *modelRepository) SetDefault(
	ctx context.Context,
	tenantID uint64,
	id string,
	modelType types.ModelType,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		visible := tx.Model(&types.Model{}).Where(
			"(tenant_id = ? OR is_builtin = true) AND type = ?",
			tenantID, modelType,
		)
		if err := visible.Update("is_default", false).Error; err != nil {
			return err
		}
		result := tx.Model(&types.Model{}).
			Where("id = ? AND (tenant_id = ? OR is_builtin = true) AND type = ? AND status = ?",
				id, tenantID, modelType, types.ModelStatusActive).
			Update("is_default", true)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("model is unavailable or inactive")
		}
		return nil
	})
}
