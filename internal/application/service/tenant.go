package service

import (
	"context"
	"errors"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

type tenantService struct {
	repo interfaces.TenantRepository
}

func NewTenantService(repo interfaces.TenantRepository) interfaces.TenantService {
	return &tenantService{repo: repo}
}

func (s *tenantService) GetTenantByID(ctx context.Context, id uint64) (*types.Tenant, error) {
	if id == 0 {
		return nil, errors.New("workspace ID cannot be 0")
	}
	return s.repo.GetTenantByID(ctx, id)
}

func (s *tenantService) UpdateTenant(ctx context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	if tenant == nil || tenant.ID == 0 {
		return nil, errors.New("workspace ID cannot be 0")
	}
	tenant.UpdatedAt = time.Now()
	if err := s.repo.UpdateTenant(ctx, tenant); err != nil {
		return nil, err
	}
	return tenant, nil
}
