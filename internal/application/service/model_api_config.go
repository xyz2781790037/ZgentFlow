package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

var (
	ErrModelAPIConfigNotFound = errors.New("model API config not found")
	ErrModelAPIConfigNameUsed = errors.New("an API config with this name already exists for the provider")
)

var enabledModelAPIConfigProviders = map[string]struct{}{
	"siliconflow": {},
	"deepseek":    {},
	"hunyuan":     {},
	"generic":     {},
}

type modelAPIConfigService struct {
	repo interfaces.ModelAPIConfigRepository
}

func NewModelAPIConfigService(repo interfaces.ModelAPIConfigRepository) interfaces.ModelAPIConfigService {
	return &modelAPIConfigService{repo: repo}
}

func normalizeModelAPIConfig(name, provider string) (string, string, error) {
	name = strings.TrimSpace(name)
	provider = strings.ToLower(strings.TrimSpace(provider))
	if name == "" || len([]rune(name)) > 100 {
		return "", "", errors.New("API config name must be between 1 and 100 characters")
	}
	if _, ok := enabledModelAPIConfigProviders[provider]; !ok {
		return "", "", errors.New("unsupported model API config provider")
	}
	return name, provider, nil
}

func (s *modelAPIConfigService) ensureUnique(
	ctx context.Context, tenantID uint64, provider, name, excludeID string,
) error {
	existing, err := s.repo.GetByName(ctx, tenantID, provider, name, excludeID)
	if err != nil {
		return err
	}
	if existing != nil {
		return ErrModelAPIConfigNameUsed
	}
	return nil
}

func (s *modelAPIConfigService) Create(ctx context.Context, name, provider, apiKey string) (*types.ModelAPIConfig, error) {
	name, provider, err := normalizeModelAPIConfig(name, provider)
	if err != nil {
		return nil, err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, errors.New("API key cannot be empty")
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	if err := s.ensureUnique(ctx, tenantID, provider, name, ""); err != nil {
		return nil, err
	}
	config := &types.ModelAPIConfig{
		TenantID: tenantID,
		Name:     name,
		Provider: provider,
		APIKey:   types.ModelAPIKey(apiKey),
	}
	if err := s.repo.Create(ctx, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (s *modelAPIConfigService) Get(ctx context.Context, id string) (*types.ModelAPIConfig, error) {
	config, err := s.repo.GetByID(ctx, types.MustTenantIDFromContext(ctx), id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, ErrModelAPIConfigNotFound
	}
	return config, nil
}

func (s *modelAPIConfigService) List(ctx context.Context) ([]*types.ModelAPIConfig, error) {
	return s.repo.List(ctx, types.MustTenantIDFromContext(ctx))
}

func (s *modelAPIConfigService) Update(
	ctx context.Context, id, name, provider string, apiKey *string,
) (*types.ModelAPIConfig, error) {
	config, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	name, provider, err = normalizeModelAPIConfig(name, provider)
	if err != nil {
		return nil, err
	}
	if err := s.ensureUnique(ctx, config.TenantID, provider, name, id); err != nil {
		return nil, err
	}
	config.Name = name
	config.Provider = provider
	if apiKey != nil {
		trimmed := strings.TrimSpace(*apiKey)
		if trimmed == "" {
			return nil, errors.New("API key cannot be empty")
		}
		config.APIKey = types.ModelAPIKey(trimmed)
	}
	config.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, config); err != nil {
		return nil, err
	}
	config.ModelCount, err = s.repo.CountModelReferences(ctx, config.TenantID, config.ID)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func (s *modelAPIConfigService) Delete(ctx context.Context, id string) (int64, error) {
	config, err := s.Get(ctx, id)
	if err != nil {
		return 0, err
	}
	count, err := s.repo.CountModelReferences(ctx, config.TenantID, config.ID)
	if err != nil {
		return 0, err
	}
	if err := s.repo.Delete(ctx, config.TenantID, config.ID); err != nil {
		return 0, err
	}
	return count, nil
}
