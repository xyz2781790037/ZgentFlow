package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/chat"
	"github.com/xyz2781790037/ZealRAG/internal/models/embedding"
	"github.com/xyz2781790037/ZealRAG/internal/models/rerank"
	"github.com/xyz2781790037/ZealRAG/internal/models/utils/ollama"
	"github.com/xyz2781790037/ZealRAG/internal/models/vlm"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"github.com/xyz2781790037/ZealRAG/internal/utils"
)

// ErrModelNotFound is returned when a model cannot be found in the repository
var ErrModelNotFound = errors.New("model not found")

// modelService implements the model service interface
type modelService struct {
	repo          interfaces.ModelRepository
	apiConfigRepo interfaces.ModelAPIConfigRepository
	ollamaService *ollama.OllamaService
	pooler        embedding.EmbedderPooler
}

// NewModelService creates a new model service instance
func NewModelService(repo interfaces.ModelRepository,
	apiConfigRepo interfaces.ModelAPIConfigRepository,
	ollamaService *ollama.OllamaService,
	pooler embedding.EmbedderPooler,
) interfaces.ModelService {
	return &modelService{
		repo:          repo,
		apiConfigRepo: apiConfigRepo,
		ollamaService: ollamaService,
		pooler:        pooler,
	}
}

func (s *modelService) getAPIConfig(ctx context.Context, tenantID uint64, id, provider string) (*types.ModelAPIConfig, error) {
	config, err := s.apiConfigRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, errors.New("模型引用的 API 配置已不存在，请重新选择 API 配置")
	}
	if config.Provider != provider {
		return nil, fmt.Errorf("API 配置属于 %s，与模型服务商 %s 不一致，请重新选择", config.Provider, provider)
	}
	return config, nil
}

// ResolveAPIConfigKey returns the current key for connection tests without
// exposing it through an HTTP response.
func (s *modelService) ResolveAPIConfigKey(ctx context.Context, id, provider string) (string, error) {
	config, err := s.getAPIConfig(ctx, types.MustTenantIDFromContext(ctx), id, provider)
	if err != nil {
		return "", err
	}
	if config.APIKey == "" {
		return "", errors.New("API 配置中的 Key 不可用，请重新填写")
	}
	return string(config.APIKey), nil
}

// ResolveModelAPIKey injects a referenced shared key into an in-memory model
// only for runtime client construction. It never persists the copied value.
func (s *modelService) ResolveModelAPIKey(ctx context.Context, model *types.Model) error {
	if model == nil || model.APIConfigID == "" {
		return nil
	}
	config, err := s.getAPIConfig(ctx, model.TenantID, model.APIConfigID, model.Parameters.Provider)
	if err != nil {
		return err
	}
	if config.APIKey == "" {
		return errors.New("API 配置中的 Key 不可用，请重新填写")
	}
	model.Parameters.APIKey = string(config.APIKey)
	return nil
}

func (s *modelService) validateModelAPIConfig(ctx context.Context, model *types.Model) error {
	if model.APIConfigID == "" {
		return nil
	}
	if model.Source != types.ModelSourceRemote {
		return errors.New("本地模型不能引用远程 API 配置")
	}
	if _, err := s.getAPIConfig(ctx, model.TenantID, model.APIConfigID, model.Parameters.Provider); err != nil {
		return err
	}
	// Shared credentials are resolved at call time; do not retain a stale copy.
	model.Parameters.APIKey = ""
	return nil
}

// decryptAppSecret 解密 AppSecret（如果为空或 cryptoSvc 为空则原样返回）
func (s *modelService) decryptAppSecret(encrypted string) string {
	if encrypted == "" {
		return encrypted
	}
	if key := utils.GetAESKey(); key != nil {
		if encrypted, err := utils.DecryptAESGCM(encrypted, key); err == nil {
			return encrypted
		}
	}
	return encrypted
}

// CreateModel creates a new model in the repository
// For local models, it initiates an asynchronous download process
// Remote models are immediately set to active status
func (s *modelService) CreateModel(ctx context.Context, model *types.Model) error {
	logger.Infof(ctx, "Creating model: %s, type: %s, source: %s", model.Name, model.Type, model.Source)

	if err := s.validateModelAPIConfig(ctx, model); err != nil {
		return err
	}

	// Handle remote models (e.g., OpenAI, Azure)
	if model.Source == types.ModelSourceRemote {
		logger.Info(ctx, "Remote model detected, setting status to active")
		model.Status = types.ModelStatusActive

		logger.Info(ctx, "Saving remote model to repository")
		err := s.repo.Create(ctx, model)
		if err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"model_name": model.Name,
				"model_type": model.Type,
			})
			return err
		}

		logger.Infof(ctx, "Remote model created successfully: %s", model.ID)
		if _, err := s.GetDefaultModel(ctx, model.Type); err != nil {
			logger.Warnf(ctx, "Failed to select a default %s model: %v", model.Type, err)
		}
		return nil
	}

	// Handle local models (e.g., Ollama)
	logger.Info(ctx, "Local model detected, setting status to downloading")
	model.Status = types.ModelStatusDownloading

	logger.Info(ctx, "Saving local model to repository")
	err := s.repo.Create(ctx, model)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_name": model.Name,
			"model_type": model.Type,
		})
		return err
	}

	// Start asynchronous model download
	logger.Infof(ctx, "Starting background download for model: %s", model.Name)
	newCtx := logger.CloneContext(ctx)
	go func() {
		logger.Info(newCtx, "Background download started")
		err := s.ollamaService.PullModel(newCtx, model.Name)
		if err != nil {
			logger.ErrorWithFields(newCtx, err, map[string]interface{}{
				"model_name": model.Name,
			})
			model.Status = types.ModelStatusDownloadFailed
		} else {
			logger.Infof(newCtx, "Model download completed successfully: %s", model.Name)
			model.Status = types.ModelStatusActive
		}
		logger.Infof(newCtx, "Updating model status to: %s", model.Status)
		if err := s.repo.Update(newCtx, model); err != nil {
			logger.Errorf(newCtx, "Failed to persist model download status: %v", err)
			return
		}
		if model.Status == types.ModelStatusActive {
			if _, err := s.GetDefaultModel(newCtx, model.Type); err != nil {
				logger.Warnf(newCtx, "Failed to select a default %s model: %v", model.Type, err)
			}
		}
	}()

	logger.Infof(ctx, "Model creation initiated successfully: %s", model.ID)
	return nil
}

// GetModelByID retrieves a model by its ID
// Returns an error if the model is not found or is in a non-active state
func (s *modelService) GetModelByID(ctx context.Context, id string) (*types.Model, error) {
	// Check if ID is empty
	if id == "" {
		logger.Error(ctx, "Model ID is empty")
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	// Fetch model from repository
	model, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  id,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	// Check if model exists
	if model == nil {
		logger.Error(ctx, "Model not found")
		return nil, ErrModelNotFound
	}

	logger.Infof(ctx, "Model found, name: %s, status: %s", model.Name, model.Status)

	// Check model status
	if model.Status == types.ModelStatusActive {
		return model, nil
	}

	if model.Status == types.ModelStatusDownloading {
		logger.Warn(ctx, "Model is currently downloading")
		return nil, errors.New("model is currently downloading")
	}

	if model.Status == types.ModelStatusDownloadFailed {
		logger.Error(ctx, "Model download failed")
		return nil, errors.New("model download failed")
	}

	logger.Error(ctx, "Model status is abnormal")
	return nil, errors.New("abnormal model status")
}

// ListModels returns all models belonging to the tenant
func (s *modelService) ListModels(ctx context.Context) ([]*types.Model, error) {
	logger.Info(ctx, "Start listing models")

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Listing models for tenant ID: %d", tenantID)

	// List models from repository with no additional filters
	models, err := s.repo.List(ctx, tenantID, "", "")
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		return nil, err
	}

	defaults := make(map[types.ModelType]bool)
	firstActive := make(map[types.ModelType]*types.Model)
	for _, model := range models {
		if model == nil || model.Status != types.ModelStatusActive {
			continue
		}
		if firstActive[model.Type] == nil {
			firstActive[model.Type] = model
		}
		if model.IsDefault {
			defaults[model.Type] = true
		}
	}
	for modelType, candidate := range firstActive {
		if defaults[modelType] {
			continue
		}
		if err := s.repo.SetDefault(ctx, tenantID, candidate.ID, modelType); err != nil {
			return nil, err
		}
		candidate.IsDefault = true
	}

	logger.Infof(ctx, "Retrieved %d models successfully", len(models))
	return models, nil
}

func (s *modelService) GetDefaultModel(
	ctx context.Context,
	modelType types.ModelType,
) (*types.Model, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	model, err := s.repo.GetDefaultByType(ctx, tenantID, modelType)
	if err != nil || model != nil {
		return model, err
	}

	models, err := s.repo.List(ctx, tenantID, modelType, "")
	if err != nil {
		return nil, err
	}
	for _, candidate := range models {
		if candidate == nil || candidate.Status != types.ModelStatusActive {
			continue
		}
		if err := s.repo.SetDefault(ctx, tenantID, candidate.ID, modelType); err != nil {
			return nil, err
		}
		candidate.IsDefault = true
		return candidate, nil
	}
	return nil, fmt.Errorf("no active %s model configured", modelType)
}

func (s *modelService) SetDefaultModel(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	model, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if model == nil {
		return ErrModelNotFound
	}
	if model.Status != types.ModelStatusActive {
		return errors.New("only active models can be selected as default")
	}
	return s.repo.SetDefault(ctx, tenantID, id, model.Type)
}

// UpdateModel updates an existing model in the repository
func (s *modelService) UpdateModel(ctx context.Context, model *types.Model) error {
	logger.Info(ctx, "Start updating model")
	logger.Infof(ctx, "Updating model ID: %s, name: %s", model.ID, model.Name)

	// Check if the model is builtin - builtin models cannot be updated
	tenantID := types.MustTenantIDFromContext(ctx)
	existingModel, err := s.repo.GetByID(ctx, tenantID, model.ID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": model.ID,
		})
		return err
	}
	if existingModel != nil && existingModel.IsBuiltin {
		logger.Warnf(ctx, "Attempted to update builtin model: %s", model.ID)
		return errors.New("builtin models cannot be updated")
	}
	if err := s.validateModelAPIConfig(ctx, model); err != nil {
		return err
	}

	// Update model in repository
	err = s.repo.Update(ctx, model)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return err
	}

	logger.Infof(ctx, "Model updated successfully: %s", model.ID)
	return nil
}

// UpdateModelCredentials writes one or more credential fields on the model's
// Parameters jsonb. Models are not pooled per-instance (each call to
// GetEmbeddingModel/GetChatModel rebuilds the client from
// the current Parameters), so no explicit cache invalidation is required —
// the next call will pick up the new credential automatically.
func (s *modelService) UpdateModelCredentials(
	ctx context.Context, id string, apiKey, appSecret *string,
) (*types.Model, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrModelNotFound
	}
	if existing.IsBuiltin {
		return nil, errors.New("builtin models cannot have credentials modified")
	}

	changed := false
	if apiKey != nil && *apiKey != "" && *apiKey != existing.Parameters.APIKey {
		existing.Parameters.APIKey = *apiKey
		changed = true
	}
	if appSecret != nil && *appSecret != "" && *appSecret != existing.Parameters.AppSecret {
		existing.Parameters.AppSecret = *appSecret
		changed = true
	}
	if !changed {
		return existing, nil
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return nil, err
	}
	logger.Infof(ctx, "Model credentials updated: id=%s", id)
	return existing, nil
}

// ClearModelCredential removes a single credential field. Idempotent.
func (s *modelService) ClearModelCredential(ctx context.Context, id, field string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return ErrModelNotFound
	}
	if existing.IsBuiltin {
		return errors.New("builtin models cannot have credentials modified")
	}

	changed := false
	switch field {
	case "api_key":
		if existing.Parameters.APIKey != "" {
			existing.Parameters.APIKey = ""
			changed = true
		}
	case "app_secret":
		if existing.Parameters.AppSecret != "" {
			existing.Parameters.AppSecret = ""
			changed = true
		}
	default:
		return errors.New("unknown credential field: " + field)
	}
	if !changed {
		return nil
	}
	if err := s.repo.Update(ctx, existing); err != nil {
		return err
	}
	logger.Infof(ctx, "Model credential cleared by user: id=%s field=%s", id, field)
	return nil
}

// DeleteModel removes a model from the repository
func (s *modelService) DeleteModel(ctx context.Context, id string) error {
	logger.Info(ctx, "Start deleting model")
	logger.Infof(ctx, "Deleting model ID: %s", id)

	tenantID := types.MustTenantIDFromContext(ctx)
	logger.Infof(ctx, "Tenant ID: %d", tenantID)

	// Check if the model is builtin - builtin models cannot be deleted
	existingModel, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": id,
		})
		return err
	}
	if existingModel != nil && existingModel.IsBuiltin {
		logger.Warnf(ctx, "Attempted to delete builtin model: %s", id)
		return errors.New("builtin models cannot be deleted")
	}
	if existingModel == nil {
		return ErrModelNotFound
	}

	// Delete model from repository
	err = s.repo.Delete(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  id,
			"tenant_id": tenantID,
		})
		return err
	}
	if existingModel.IsDefault {
		if _, err := s.GetDefaultModel(ctx, existingModel.Type); err != nil {
			logger.Warnf(ctx, "No replacement default model for type %s: %v", existingModel.Type, err)
		}
	}

	logger.Infof(ctx, "Model deleted successfully: %s", id)
	return nil
}

// GetEmbeddingModel retrieves and initializes an embedding model instance
// Takes a model ID and returns an Embedder interface implementation
func (s *modelService) GetEmbeddingModel(ctx context.Context, modelId string) (embedding.Embedder, error) {
	// Get the model details
	model, err := s.GetModelByID(ctx, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": modelId,
		})
		return nil, err
	}
	if err := s.ResolveModelAPIKey(ctx, model); err != nil {
		return nil, err
	}

	logger.Infof(ctx, "Getting embedding model: %s, source: %s", model.Name, model.Source)

	embedder, err := embedding.NewEmbedder(embedding.ConfigFromModel(model), s.pooler, s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	logger.Info(ctx, "Embedding model initialized successfully")
	return embedder, nil
}

// GetRerankModel retrieves and initializes a reranking model instance
// Takes a model ID and returns a Reranker interface implementation
func (s *modelService) GetRerankModel(ctx context.Context, modelId string) (rerank.Reranker, error) {
	// Get the model details
	model, err := s.GetModelByID(ctx, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id": modelId,
		})
		return nil, err
	}
	if err := s.ResolveModelAPIKey(ctx, model); err != nil {
		return nil, err
	}

	logger.Infof(ctx, "Getting rerank model: %s, source: %s", model.Name, model.Source)

	appSecret := s.decryptAppSecret(model.Parameters.AppSecret)
	reranker, err := rerank.NewReranker(rerank.ConfigFromModel(model, appSecret))
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	logger.Info(ctx, "Rerank model initialized successfully")
	return reranker, nil
}

// GetChatModel retrieves and initializes a chat model instance
// Takes a model ID and returns a Chat interface implementation
func (s *modelService) GetChatModel(ctx context.Context, modelId string) (chat.Chat, error) {
	// Check if model ID is empty
	if modelId == "" {
		logger.Error(ctx, "Model ID is empty")
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	// Get the model directly from repository to avoid status checks
	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		logger.Error(ctx, "Chat model not found")
		return nil, ErrModelNotFound
	}
	if err := s.ResolveModelAPIKey(ctx, model); err != nil {
		return nil, err
	}

	logger.Infof(ctx, "Getting chat model: %s, source: %s", model.Name, model.Source)

	chatModel, err := chat.NewChat(chat.ConfigFromModel(model), s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	return chatModel, nil
}

// GetVLMModel retrieves and initializes a vision language model instance.
func (s *modelService) GetVLMModel(ctx context.Context, modelId string) (vlm.VLM, error) {
	if modelId == "" {
		return nil, errors.New("model ID cannot be empty")
	}

	tenantID := types.MustTenantIDFromContext(ctx)

	model, err := s.repo.GetByID(ctx, tenantID, modelId)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":  modelId,
			"tenant_id": tenantID,
		})
		return nil, err
	}

	if model == nil {
		return nil, ErrModelNotFound
	}
	if err := s.ResolveModelAPIKey(ctx, model); err != nil {
		return nil, err
	}

	logger.Infof(ctx, "Getting VLM model: %s, source: %s", model.Name, model.Source)

	vlmModel, err := vlm.NewVLM(vlm.ConfigFromModel(model), s.ollamaService)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"model_id":   model.ID,
			"model_name": model.Name,
		})
		return nil, err
	}

	return vlmModel, nil
}

// Note: default model selection logic has been removed; models no longer
// maintain a per-type default flag at the service layer.
