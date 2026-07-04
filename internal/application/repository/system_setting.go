package repository

import (
	"context"
	"errors"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"gorm.io/gorm"
)

// systemSettingRepository implements interfaces.SystemSettingRepository
// against the system_settings table (migration 000053). The table is
// system-scoped (no tenant_id column) and intentionally tiny — single-
// digit rows in P1 — so List does not paginate.
type systemSettingRepository struct {
	db *gorm.DB
}

// NewSystemSettingRepository wires the repo into the dig container.
// Receives the shared *gorm.DB; no other deps.
func NewSystemSettingRepository(db *gorm.DB) interfaces.SystemSettingRepository {
	return &systemSettingRepository{db: db}
}

// Get fetches a system setting by key. Returns (nil, nil) when the row
// does not exist — the resolver service treats "missing" as "fall back
// to ENV / default", so a 404 here is a normal control-flow signal,
// not an error. Real DB errors (connection lost, etc.) surface up.
func (r *systemSettingRepository) Get(ctx context.Context, key string) (*types.SystemSetting, error) {
	var s types.SystemSetting
	err := r.db.WithContext(ctx).Where("key = ?", key).First(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

// List returns every system_settings row, ordered by category then key
// for stable management-UI rendering. No pagination — see type comment.
func (r *systemSettingRepository) List(ctx context.Context) ([]*types.SystemSetting, error) {
	var rows []*types.SystemSetting
	err := r.db.WithContext(ctx).Order("category ASC, key ASC").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}
