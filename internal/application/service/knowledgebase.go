package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// ErrInvalidTenantID represents an error for invalid tenant ID
var ErrInvalidTenantID = errors.New("invalid tenant ID")

// knowledgeBaseService implements the knowledge base service interface
type knowledgeBaseService struct {
	repo           interfaces.KnowledgeBaseRepository
	kgRepo         interfaces.KnowledgeRepository
	chunkRepo      interfaces.ChunkRepository
	modelService   interfaces.ModelService
	retrieveEngine interfaces.RetrieveEngineRegistry
	ownership      retriever.TenantStoreOwnership
	tenantRepo     interfaces.TenantRepository
	fileSvc        interfaces.FileService
	asynqClient    interfaces.TaskEnqueuer
}

// NewKnowledgeBaseService creates a new knowledge base service
func NewKnowledgeBaseService(repo interfaces.KnowledgeBaseRepository,
	kgRepo interfaces.KnowledgeRepository,
	chunkRepo interfaces.ChunkRepository,
	modelService interfaces.ModelService,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	tenantRepo interfaces.TenantRepository,
	fileSvc interfaces.FileService,
	asynqClient interfaces.TaskEnqueuer,
) interfaces.KnowledgeBaseService {
	return &knowledgeBaseService{
		repo:           repo,
		kgRepo:         kgRepo,
		chunkRepo:      chunkRepo,
		modelService:   modelService,
		retrieveEngine: retrieveEngine,
		ownership:      ownership,
		tenantRepo:     tenantRepo,
		fileSvc:        fileSvc,
		asynqClient:    asynqClient,
	}
}

// GetRepository gets the knowledge base repository
// Parameters:
//   - ctx: Context with authentication and request information
//
// Returns:
//   - interfaces.KnowledgeBaseRepository: Knowledge base repository
func (s *knowledgeBaseService) GetRepository() interfaces.KnowledgeBaseRepository {
	return s.repo
}

// CreateKnowledgeBase creates a new knowledge base using ZealRAG's built-in
// PostgreSQL/pgvector retrieval backend.
func (s *knowledgeBaseService) CreateKnowledgeBase(ctx context.Context,
	kb *types.KnowledgeBase,
) (*types.KnowledgeBase, error) {
	// Generate UUID and set creation timestamps
	if kb.ID == "" {
		kb.ID = uuid.New().String()
	}
	kb.CreatedAt = time.Now()
	kb.TenantID = types.MustTenantIDFromContext(ctx)
	if userID, ok := types.UserIDFromContext(ctx); ok {
		kb.OwnerUserID = userID
	}
	kb.UpdatedAt = time.Now()
	kb.EnsureDefaults()

	chatModel, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeKnowledgeQA)
	if err != nil {
		return nil, err
	}
	embeddingModel, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeEmbedding)
	if err != nil {
		return nil, err
	}
	kb.SummaryModelID = chatModel.ID
	kb.EmbeddingModelID = embeddingModel.ID
	if kb.VLMConfig.Enabled {
		vlmModel, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeVLLM)
		if err != nil {
			return nil, err
		}
		kb.VLMConfig.ModelID = vlmModel.ID
	} else {
		kb.VLMConfig.ModelID = ""
	}

	// Ignore legacy client bindings. The column remains readable so existing
	// databases and queued tasks can be upgraded without a destructive migration.
	kb.VectorStoreID = nil
	kb.Normalize()

	logger.Infof(ctx, "Creating knowledge base, ID: %s, tenant ID: %d, name: %s", kb.ID, kb.TenantID, kb.Name)

	if err := s.repo.CreateKnowledgeBase(ctx, kb); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": kb.ID,
			"tenant_id":         kb.TenantID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Knowledge base created successfully, ID: %s, name: %s", kb.ID, kb.Name)
	return kb, nil
}

// GetKnowledgeBaseByID retrieves a knowledge base by its ID
func (s *knowledgeBaseService) GetKnowledgeBaseByID(ctx context.Context, id string) (*types.KnowledgeBase, error) {
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return nil, errors.New("knowledge base ID cannot be empty")
	}

	kb, err := s.repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return nil, err
	}

	kb.EnsureDefaults()
	return kb, nil
}

// GetKnowledgeBasesByIDsOnly retrieves knowledge bases by IDs without tenant filter (batch).
func (s *knowledgeBaseService) GetKnowledgeBasesByIDsOnly(ctx context.Context, ids []string) ([]*types.KnowledgeBase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	kbs, err := s.repo.GetKnowledgeBaseByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, kb := range kbs {
		if kb != nil {
			kb.EnsureDefaults()
		}
	}
	return kbs, nil
}

// ListKnowledgeBases returns all knowledge bases for a tenant
func (s *knowledgeBaseService) ListKnowledgeBases(ctx context.Context) ([]*types.KnowledgeBase, error) {
	tenantID, _ := types.CallerTenantIDFromContext(ctx)

	kbs, err := s.repo.ListKnowledgeBasesByTenantID(ctx, tenantID)
	if err != nil {
		for _, kb := range kbs {
			kb.EnsureDefaults()
		}

		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"tenant_id": tenantID,
		})
		return nil, err
	}

	// Query knowledge count and chunk count for each knowledge base
	for _, kb := range kbs {
		kb.EnsureDefaults()

		// Get knowledge count
		switch kb.Type {
		case types.KnowledgeBaseTypeDocument:
			knowledgeCount, err := s.kgRepo.CountKnowledgeByKnowledgeBaseID(ctx, tenantID, kb.ID)
			if err != nil {
				logger.Warnf(ctx, "Failed to get knowledge count for knowledge base %s: %v", kb.ID, err)
			} else {
				kb.KnowledgeCount = knowledgeCount
			}
		case types.KnowledgeBaseTypeFAQ:
			// Get chunk count
			chunkCount, err := s.chunkRepo.CountChunksByKnowledgeBaseID(ctx, tenantID, kb.ID)
			if err != nil {
				logger.Warnf(ctx, "Failed to get chunk count for knowledge base %s: %v", kb.ID, err)
			} else {
				kb.ChunkCount = chunkCount
			}
		}

		// Check if there is a processing import task
		processingCount, err := s.kgRepo.CountKnowledgeByStatus(
			ctx,
			tenantID,
			kb.ID,
			[]string{"pending", "processing"},
		)
		if err != nil {
			logger.Warnf(ctx, "Failed to check processing status for knowledge base %s: %v", kb.ID, err)
		} else {
			kb.IsProcessing = processingCount > 0
			kb.ProcessingCount = processingCount
		}
	}

	// Per-user pin stamping + ordering. The "main" list view is the
	// only path that needs to honour the caller's personal pin set;
	// agent/share/IM callers go through ListKnowledgeBasesByTenantID
	// which also enriches but keys off the user in their own context.
	if userID, ok := types.UserIDFromContext(ctx); ok && userID != "" {
		s.applyUserKBPins(ctx, tenantID, userID, kbs)
	}
	return kbs, nil
}

// FillKnowledgeBaseCounts fills KnowledgeCount, ChunkCount, IsProcessing, ProcessingCount for the given KB using kb.TenantID.
func (s *knowledgeBaseService) FillKnowledgeBaseCounts(ctx context.Context, kb *types.KnowledgeBase) error {
	if kb == nil {
		return nil
	}
	tenantID := kb.TenantID
	kb.EnsureDefaults()
	switch kb.Type {
	case types.KnowledgeBaseTypeDocument:
		if cnt, err := s.kgRepo.CountKnowledgeByKnowledgeBaseID(ctx, tenantID, kb.ID); err == nil {
			kb.KnowledgeCount = cnt
		}
	case types.KnowledgeBaseTypeFAQ:
		if cnt, err := s.chunkRepo.CountChunksByKnowledgeBaseID(ctx, tenantID, kb.ID); err == nil {
			kb.ChunkCount = cnt
		}
	}
	if processingCount, err := s.kgRepo.CountKnowledgeByStatus(ctx, tenantID, kb.ID, []string{"pending", "processing"}); err == nil {
		kb.IsProcessing = processingCount > 0
		kb.ProcessingCount = processingCount
	}
	return nil
}

// UpdateKnowledgeBase updates a knowledge base's mutable properties.
//
// IMPORTANT — vector_store_id immutability contract:
// The vector_store_id binding is deliberately not accepted by this method.
// Two layers enforce immutability:
//
//  1. ORM layer: the GORM tag `<-:create` on KnowledgeBase.VectorStoreID
//     makes every UPDATE path (Save / Updates / Select-Updates) a no-op for
//     that column. Verified by repository/knowledgebase_sqlite_test.go.
//  2. Service layer: this method intentionally omits VectorStoreID from its
//     parameter list, and the matching handler DTO UpdateKnowledgeBaseRequest
//     omits the field as well. A reflection-based regression test
//     (handler/knowledgebase_request_test.go) fails if either DTO field
//     is added back, alerting future maintainers.
//
// Any future cross-store rebind workflow must use raw SQL through a
// dedicated repository method — the only sanctioned write path post-creation.
func (s *knowledgeBaseService) UpdateKnowledgeBase(ctx context.Context,
	id string,
	name string,
	description string,
	config *types.KnowledgeBaseConfig,
) (*types.KnowledgeBase, error) {
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return nil, errors.New("knowledge base ID cannot be empty")
	}

	logger.Infof(ctx, "Updating knowledge base, ID: %s, name: %s", id, name)

	// Get existing knowledge base
	kb, err := s.repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return nil, err
	}

	// Update the knowledge base properties
	kb.Name = name
	kb.Description = description
	if config != nil {
		if config.MaxFileSizeMB != nil {
			kb.MaxFileSizeMB = *config.MaxFileSizeMB
		}
		kb.ChunkingConfig = config.ChunkingConfig
		kb.ImageProcessingConfig = config.ImageProcessingConfig
		if config.FAQConfig != nil {
			kb.FAQConfig = config.FAQConfig
		}
		// Update indexing strategy.
		if config.IndexingStrategy != nil {
			if !config.IndexingStrategy.HasAnyIndexing() {
				return nil, errors.New("at least one indexing strategy must be enabled")
			}
			kb.IndexingStrategy = *config.IndexingStrategy
		}
	}
	kb.UpdatedAt = time.Now()
	kb.EnsureDefaults()

	logger.Info(ctx, "Saving knowledge base update")
	if err := s.repo.UpdateKnowledgeBase(ctx, kb); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return nil, err
	}

	logger.Infof(ctx, "Knowledge base updated successfully, ID: %s, name: %s", kb.ID, kb.Name)
	return kb, nil
}

// TogglePinKnowledgeBase toggles whether the calling user has pinned
// this knowledge base. Pin state is per-(user, kb) as of migration
// 000050; previously this method flipped a tenant-wide column on the
// KB row which broke down under RBAC (only Admin/creator could pin,
// and the pin reordered the list for everyone in the tenant). The
// public signature is unchanged so the HTTP handler / CLI / SDK don't
// move.
//
// The KB still has to belong to the caller's tenant — the route is
// already gated behind KBAccessRead, but we re-check via
// GetKnowledgeBaseByIDAndTenant so a stale param survives a tenant
// switch cleanly.
func (s *knowledgeBaseService) TogglePinKnowledgeBase(
	ctx context.Context, id string,
) (*types.KnowledgeBase, error) {
	if id == "" {
		return nil, errors.New("knowledge base ID cannot be empty")
	}
	tenantID, _ := types.CallerTenantIDFromContext(ctx)
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		// API-key callers without a user identity can't have a personal
		// pin set. We surface this rather than silently flipping a
		// shared-tenant flag like the old behaviour.
		return nil, errors.New("pin requires an authenticated user")
	}

	// Look the KB up without a tenant filter: the route's KBAccessRead
	// guard already validated that this caller can see this KB (own,
	// org-shared, or agent-shared). Filtering by the caller's tenant
	// here would 404 every legitimate pin against a shared KB whose
	// owning tenant differs from the caller's active tenant.
	kb, err := s.repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
			"tenant_id":         tenantID,
		})
		return nil, err
	}

	// Read current pin state to decide direction. ListUserKBPinIDs is
	// already optimised for the "many KBs at once" path; for a single-id
	// check the round-trip is acceptable and avoids leaking a second
	// repository method just for this.
	pins, err := s.repo.ListUserKBPinIDs(ctx, tenantID, userID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
			"tenant_id":         tenantID,
			"user_id":           userID,
		})
		return nil, err
	}
	_, currentlyPinned := pins[id]

	pinnedAt, err := s.repo.SetUserKBPin(ctx, tenantID, userID, id, !currentlyPinned)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
			"tenant_id":         tenantID,
			"user_id":           userID,
			"target_pinned":     !currentlyPinned,
		})
		return nil, err
	}

	kb.EnsureDefaults()
	kb.IsPinned = !currentlyPinned
	kb.PinnedAt = pinnedAt
	logger.Infof(ctx, "Knowledge base pin toggled, ID: %s, user: %s, is_pinned: %v",
		id, userID, kb.IsPinned)
	return kb, nil
}

// applyUserKBPins stamps IsPinned / PinnedAt onto each KB in the slice
// from the caller's perspective and sorts the slice so pinned rows
// float to the top (newest pin first, ties broken by created_at desc).
// Safe to call with an empty userID (no-op stamp; default sort by
// created_at preserved).
func (s *knowledgeBaseService) applyUserKBPins(
	ctx context.Context, tenantID uint64, userID string, kbs []*types.KnowledgeBase,
) {
	if len(kbs) == 0 || userID == "" {
		return
	}
	pins, err := s.repo.ListUserKBPinIDs(ctx, tenantID, userID)
	if err != nil {
		// Pin enrichment is best-effort: a transient DB blip here
		// should not break listing KBs. Log and bail without altering
		// the slice — caller still gets a valid list, just unsorted by
		// pin.
		logger.Warnf(ctx, "applyUserKBPins: failed to load pins for tenant=%d user=%s: %v",
			tenantID, userID, err)
		return
	}
	if len(pins) == 0 {
		return
	}
	for _, kb := range kbs {
		if ts, ok := pins[kb.ID]; ok {
			kb.IsPinned = true
			t := ts
			kb.PinnedAt = &t
		}
	}
	sort.SliceStable(kbs, func(i, j int) bool {
		a, b := kbs[i], kbs[j]
		if a.IsPinned != b.IsPinned {
			return a.IsPinned
		}
		if a.IsPinned && b.IsPinned {
			at, bt := a.PinnedAt, b.PinnedAt
			if at != nil && bt != nil && !at.Equal(*bt) {
				return at.After(*bt)
			}
		}
		return a.CreatedAt.After(b.CreatedAt)
	})
}

// DeleteKnowledgeBase deletes a knowledge base by its ID
// This method marks the knowledge base as deleted and enqueues an async task
// to handle the heavy cleanup operations (embeddings, chunks, files, graph data)
func (s *knowledgeBaseService) DeleteKnowledgeBase(ctx context.Context, id string) error {
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return errors.New("knowledge base ID cannot be empty")
	}

	logger.Infof(ctx, "Deleting knowledge base, ID: %s", id)

	kb, err := s.repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return err
	}
	if err := s.repo.DeleteKnowledgeBase(ctx, id); err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return err
	}
	return s.enqueueKnowledgeBasePurge(ctx, kb.TenantID, kb)
}

func (s *knowledgeBaseService) enqueueKnowledgeBasePurge(
	ctx context.Context, tenantID uint64, kb *types.KnowledgeBase,
) error {
	payload := types.KBDeletePayload{
		TenantID:         tenantID,
		KnowledgeBaseID:  kb.ID,
		EffectiveEngines: types.GetDefaultRetrieverEngines(),
		VectorStoreID:    kb.VectorStoreID,
	}
	langfuse.InjectTracing(ctx, &payload)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Warnf(ctx, "Failed to marshal KB delete payload: %v", err)
		// Don't fail the request, the KB record is already deleted
		return nil
	}

	task := asynq.NewTask(types.TypeKBDelete, payloadBytes)
	info, err := s.asynqClient.Enqueue(task, asynq.Queue("low"), asynq.MaxRetry(3))
	if err != nil {
		logger.Warnf(ctx, "Failed to enqueue KB delete task: %v", err)
		// Don't fail the request, the KB record is already deleted
		return nil
	}

	logger.Infof(ctx, "KB purge task enqueued: %s, knowledge base ID: %s", info.ID, kb.ID)
	return nil
}

// TrashKnowledgeBase moves a regular knowledge base into the recycle bin
// without touching its files, chunks, indexes or generated Wiki pages.
func (s *knowledgeBaseService) TrashKnowledgeBase(ctx context.Context, id string) error {
	if id == "" {
		return errors.New("knowledge base ID cannot be empty")
	}
	kb, err := s.repo.GetKnowledgeBaseByIDAndTenant(ctx, id, types.MustTenantIDFromContext(ctx))
	if err != nil {
		return err
	}
	if kb.IsTemporary {
		return errors.New("temporary knowledge bases do not use the recycle bin")
	}
	return s.repo.DeleteKnowledgeBase(ctx, id)
}

func (s *knowledgeBaseService) ListTrashedKnowledgeBases(ctx context.Context) ([]*types.KnowledgeBase, error) {
	return s.repo.ListTrashedKnowledgeBases(ctx, types.MustTenantIDFromContext(ctx))
}

func (s *knowledgeBaseService) RestoreKnowledgeBase(ctx context.Context, id string) error {
	return s.repo.RestoreKnowledgeBase(ctx, types.MustTenantIDFromContext(ctx), id)
}

func (s *knowledgeBaseService) PurgeTrashedKnowledgeBase(ctx context.Context, id string) error {
	return s.PurgeTrashedKnowledgeBaseForTenant(ctx, types.MustTenantIDFromContext(ctx), id)
}

func (s *knowledgeBaseService) PurgeTrashedKnowledgeBaseForTenant(
	ctx context.Context, tenantID uint64, id string,
) error {
	kb, err := s.repo.GetTrashedKnowledgeBase(ctx, tenantID, id)
	if err != nil {
		return err
	}
	return s.enqueueKnowledgeBasePurge(ctx, tenantID, kb)
}

// ProcessKBDelete handles async knowledge base deletion task
// This method performs heavy cleanup operations: deleting embeddings, chunks, files, and graph data
func (s *knowledgeBaseService) ProcessKBDelete(ctx context.Context, t *asynq.Task) error {
	var payload types.KBDeletePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "Failed to unmarshal KB delete payload: %v", err)
		return err
	}

	tenantID := payload.TenantID
	kbID := payload.KnowledgeBaseID

	// Set tenant context for downstream services
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)

	logger.Infof(ctx, "Processing KB delete task for knowledge base: %s", kbID)

	// Step 1: Get all knowledge entries in this knowledge base
	logger.Infof(ctx, "Fetching all knowledge entries in knowledge base, ID: %s", kbID)
	knowledgeList, err := s.kgRepo.ListKnowledgeByKnowledgeBaseID(ctx, tenantID, kbID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": kbID,
		})
		return err
	}
	logger.Infof(ctx, "Found %d knowledge entries to delete", len(knowledgeList))

	// Step 2: Delete all knowledge entries and their resources
	if len(knowledgeList) > 0 {
		knowledgeIDs := make([]string, 0, len(knowledgeList))
		for _, knowledge := range knowledgeList {
			knowledgeIDs = append(knowledgeIDs, knowledge.ID)
		}

		logger.Infof(ctx, "Deleting all knowledge entries and their resources")

		// Delete embeddings through the fixed pgvector engine. Legacy payload
		// routing fields are accepted but ignored by the factory.
		logger.Infof(ctx, "Deleting embeddings from vector store")
		retrieveEngine, err := retriever.CreateRetrieveEngineFromPayload(
			ctx,
			s.retrieveEngine,
			s.ownership,
			payload.TenantID,
			payload.EffectiveEngines,
			payload.VectorStoreID,
		)
		if err != nil {
			logger.Warnf(ctx, "Failed to create retrieve engine: %v", err)
		} else {
			// Group knowledge by embedding model and type
			type groupKey struct {
				EmbeddingModelID string
				Type             string
			}
			embeddingGroups := make(map[groupKey][]string)
			for _, knowledge := range knowledgeList {
				key := groupKey{EmbeddingModelID: knowledge.EmbeddingModelID, Type: knowledge.Type}
				embeddingGroups[key] = append(embeddingGroups[key], knowledge.ID)
			}

			for key, knowledgeGroup := range embeddingGroups {
				embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, key.EmbeddingModelID)
				if err != nil {
					logger.Warnf(ctx, "Failed to get embedding model %s: %v", key.EmbeddingModelID, err)
					continue
				}
				if err := retrieveEngine.DeleteByKnowledgeIDList(ctx, knowledgeGroup, embeddingModel.GetDimensions(), key.Type); err != nil {
					logger.Warnf(ctx, "Failed to delete embeddings for model %s: %v", key.EmbeddingModelID, err)
				}
			}
		}

		// Delete all chunks
		logger.Infof(ctx, "Deleting all chunks in knowledge base")
		for _, knowledgeID := range knowledgeIDs {
			if err := s.chunkRepo.DeleteChunksByKnowledgeID(ctx, tenantID, knowledgeID); err != nil {
				logger.Warnf(ctx, "Failed to delete chunks for knowledge %s: %v", knowledgeID, err)
			}
		}

		// Delete physical source files and adjust storage.
		logger.Infof(ctx, "Deleting physical files")
		storageAdjust := int64(0)
		for _, knowledge := range knowledgeList {
			if knowledge.FilePath != "" {
				if err := s.fileSvc.DeleteFile(ctx, knowledge.FilePath); err != nil {
					logger.Warnf(ctx, "Failed to delete file %s: %v", knowledge.FilePath, err)
				}
			}
			storageAdjust -= knowledge.StorageSize
		}
		if storageAdjust != 0 {
			if err := s.tenantRepo.AdjustStorageUsed(ctx, tenantID, storageAdjust); err != nil {
				logger.Warnf(ctx, "Failed to adjust tenant storage: %v", err)
			}
		}

		// Delete all knowledge entries from database
		logger.Infof(ctx, "Deleting knowledge entries from database")
		if err := s.kgRepo.DeleteKnowledgeList(ctx, tenantID, knowledgeIDs); err != nil {
			logger.ErrorWithFields(ctx, err, map[string]interface{}{
				"knowledge_base_id": kbID,
			})
			return err
		}
		if err := s.kgRepo.PurgeKnowledgeList(ctx, tenantID, knowledgeIDs); err != nil {
			return err
		}
	}

	if err := s.repo.PurgeKnowledgeBase(ctx, tenantID, kbID); err != nil {
		return err
	}
	logger.Infof(ctx, "KB delete task completed successfully, knowledge base ID: %s", kbID)
	return nil
}

// SetEmbeddingModel sets the embedding model for a knowledge base
func (s *knowledgeBaseService) SetEmbeddingModel(ctx context.Context, id string, modelID string) error {
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return errors.New("knowledge base ID cannot be empty")
	}

	if modelID == "" {
		logger.Error(ctx, "Model ID is empty")
		return errors.New("model ID cannot be empty")
	}

	logger.Infof(ctx, "Setting embedding model for knowledge base, knowledge base ID: %s, model ID: %s", id, modelID)

	// Get the knowledge base
	kb, err := s.repo.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id": id,
		})
		return err
	}

	// Update the knowledge base's embedding model
	kb.EmbeddingModelID = modelID
	kb.UpdatedAt = time.Now()

	logger.Info(ctx, "Saving knowledge base embedding model update")
	err = s.repo.UpdateKnowledgeBase(ctx, kb)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_base_id":  id,
			"embedding_model_id": modelID,
		})
		return err
	}

	logger.Infof(
		ctx,
		"Knowledge base embedding model set successfully, knowledge base ID: %s, model ID: %s",
		id,
		modelID,
	)
	return nil
}
