package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

const localWorkspaceEmail = "owner@zealrag.local"

type userService struct {
	userRepo   interfaces.UserRepository
	tenantRepo interfaces.TenantRepository
}

func NewUserService(
	userRepo interfaces.UserRepository,
	tenantRepo interfaces.TenantRepository,
) interfaces.UserService {
	return &userService{userRepo: userRepo, tenantRepo: tenantRepo}
}

func (s *userService) ResolveLocalWorkspace(ctx context.Context) (*types.User, *types.Tenant, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, localWorkspaceEmail)
	if user != nil {
		tenant, err := s.tenantRepo.GetTenantByID(ctx, user.TenantID)
		if err != nil {
			return nil, nil, fmt.Errorf("load local workspace: %w", err)
		}
		return user, tenant, nil
	}
	if err != nil && !errors.Is(err, interfaces.ErrUserNotFound) {
		return nil, nil, fmt.Errorf("load local workspace actor: %w", err)
	}

	now := time.Now()
	tenant := &types.Tenant{
		Name:         "ZgentFlow",
		Description:  "Local workspace",
		Status:       "active",
		StorageQuota: 10 * 1024 * 1024 * 1024,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.tenantRepo.CreateTenant(ctx, tenant); err != nil {
		return nil, nil, fmt.Errorf("create local workspace: %w", err)
	}

	user = &types.User{
		ID:           uuid.New().String(),
		Username:     "local",
		Email:        localWorkspaceEmail,
		PasswordHash: "login-disabled",
		TenantID:     tenant.ID,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.userRepo.CreateUser(ctx, user); err != nil {
		return nil, nil, fmt.Errorf("create local workspace actor: %w", err)
	}
	return user, tenant, nil
}
