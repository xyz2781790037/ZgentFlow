package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"go.uber.org/dig"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	kbRebuildTaskTimeout = 24 * time.Hour
	kbRebuildVectorBatch = 80
)

type KBFullRebuildParams struct {
	dig.In

	DB             *gorm.DB
	KBRepo         interfaces.KnowledgeBaseRepository
	KnowledgeRepo  interfaces.KnowledgeRepository
	TenantRepo     interfaces.TenantRepository
	ModelService   interfaces.ModelService
	RetrieveEngine interfaces.RetrieveEngineRegistry
	Ownership      retriever.TenantStoreOwnership
	Task           interfaces.TaskEnqueuer
}

type kbFullRebuildService struct {
	db             *gorm.DB
	kbRepo         interfaces.KnowledgeBaseRepository
	knowledgeRepo  interfaces.KnowledgeRepository
	tenantRepo     interfaces.TenantRepository
	modelService   interfaces.ModelService
	retrieveEngine interfaces.RetrieveEngineRegistry
	ownership      retriever.TenantStoreOwnership
	task           interfaces.TaskEnqueuer
}

func NewKBFullRebuildService(p KBFullRebuildParams) interfaces.KBFullRebuildService {
	return &kbFullRebuildService{
		db: p.DB, kbRepo: p.KBRepo, knowledgeRepo: p.KnowledgeRepo,
		tenantRepo: p.TenantRepo, modelService: p.ModelService,
		retrieveEngine: p.RetrieveEngine, ownership: p.Ownership,
		task: p.Task,
	}
}

func rebuildState(kb *types.KnowledgeBase) *types.KBRebuildState {
	if kb == nil {
		return nil
	}
	return &types.KBRebuildState{
		KnowledgeBaseID: kb.ID, ActiveGeneration: kb.ActiveGeneration,
		BuildingGeneration: kb.BuildingGeneration, Status: kb.RebuildStatus,
		Error: kb.RebuildError, StartedAt: kb.RebuildStartedAt,
		CompletedAt: kb.RebuildCompletedAt,
	}
}

func (s *kbFullRebuildService) GetState(ctx context.Context, kbID string) (*types.KBRebuildState, error) {
	tenantID := types.MustTenantIDFromContext(ctx)
	kb, err := s.kbRepo.GetKnowledgeBaseByIDAndTenant(ctx, kbID, tenantID)
	if err != nil {
		return nil, err
	}
	return rebuildState(kb), nil
}

func (s *kbFullRebuildService) Start(ctx context.Context, kbID string) (*types.KBRebuildState, error) {
	if strings.TrimSpace(kbID) == "" {
		return nil, errors.New("knowledge base ID cannot be empty")
	}
	embeddingModel, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeEmbedding)
	if err != nil {
		return nil, fmt.Errorf("load workspace embedding model: %w", err)
	}
	chatModel, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeKnowledgeQA)
	if err != nil {
		return nil, fmt.Errorf("load workspace chat model: %w", err)
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	var state *types.KBRebuildState
	var generation int64

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var kb types.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND tenant_id = ?", kbID, tenantID).First(&kb).Error; err != nil {
			return err
		}
		if kb.RebuildStatus == types.KBRebuildStatusPending || kb.RebuildStatus == types.KBRebuildStatusRunning {
			return fmt.Errorf("knowledge base rebuild is already %s", kb.RebuildStatus)
		}
		if kb.ActiveGeneration < 1 {
			kb.ActiveGeneration = 1
		}
		baseGeneration := kb.ActiveGeneration
		if kb.BuildingGeneration != nil && *kb.BuildingGeneration > baseGeneration {
			baseGeneration = *kb.BuildingGeneration
		}
		generation = baseGeneration + 1
		now := time.Now().UTC()
		updates := map[string]interface{}{
			"building_generation":  generation,
			"rebuild_stage_id":     "",
			"rebuild_status":       types.KBRebuildStatusPending,
			"rebuild_error":        "",
			"rebuild_started_at":   now,
			"rebuild_completed_at": nil,
		}
		if err := tx.Table("knowledge_bases").Where("id = ?", kbID).Updates(updates).Error; err != nil {
			return err
		}
		kb.BuildingGeneration = &generation
		kb.RebuildStatus = types.KBRebuildStatusPending
		kb.RebuildError = ""
		kb.RebuildStartedAt = &now
		kb.RebuildCompletedAt = nil
		state = rebuildState(&kb)
		return nil
	})
	if err != nil {
		return nil, err
	}

	payload := types.KBFullRebuildPayload{
		TenantID: tenantID, KnowledgeBaseID: kbID, Generation: generation,
		EmbeddingModelID: embeddingModel.ID, KnowledgeQAModelID: chatModel.ID,
	}
	langfuse.InjectTracing(ctx, &payload)
	body, err := json.Marshal(payload)
	if err != nil {
		s.markFailed(context.Background(), payload, err)
		return nil, err
	}
	if _, err := s.task.Enqueue(
		asynq.NewTask(types.TypeKBFullRebuild, body),
		asynq.Queue(types.QueueLow), asynq.MaxRetry(0), asynq.Timeout(kbRebuildTaskTimeout),
	); err != nil {
		s.markFailed(context.Background(), payload, fmt.Errorf("enqueue rebuild: %w", err))
		return nil, err
	}
	return state, nil
}

func (s *kbFullRebuildService) Handle(ctx context.Context, task *asynq.Task) error {
	var payload types.KBFullRebuildPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("decode full rebuild payload: %w", err)
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)

	if err := s.run(ctx, payload); err != nil {
		logger.Errorf(ctx, "full KB rebuild failed: kb=%s generation=%d error=%v",
			payload.KnowledgeBaseID, payload.Generation, err)
		s.markFailed(context.Background(), payload, err)
		return nil
	}
	return nil
}

func (s *kbFullRebuildService) run(ctx context.Context, payload types.KBFullRebuildPayload) error {
	kb, err := s.kbRepo.GetKnowledgeBaseByIDAndTenant(ctx, payload.KnowledgeBaseID, payload.TenantID)
	if err != nil {
		return fmt.Errorf("load knowledge base: %w", err)
	}
	if kb.BuildingGeneration == nil || *kb.BuildingGeneration != payload.Generation {
		return fmt.Errorf("rebuild generation %d is stale", payload.Generation)
	}

	tenant, err := s.tenantRepo.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		return fmt.Errorf("load tenant: %w", err)
	}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	stage, err := s.createStage(ctx, kb, payload)
	if err != nil {
		return err
	}
	if err := s.markRunning(ctx, payload, stage.ID); err != nil {
		return err
	}

	chunks, err := s.listSourceChunks(ctx, payload.TenantID, kb.ID)
	if err != nil {
		return err
	}
	if len(chunks) == 0 {
		return errors.New("no indexed document chunks are available for rebuild")
	}

	snapshot := fingerprintSourceChunks(chunks)
	vectorCount, err := s.rebuildVectors(ctx, kb, stage, chunks)
	if err != nil {
		return fmt.Errorf("vector rebuild failed: %w", err)
	}
	if err := s.validateStage(ctx, stage.ID, vectorCount); err != nil {
		return fmt.Errorf("rebuild validation failed: %w", err)
	}
	if err := s.publish(ctx, payload, stage.ID, snapshot); err != nil {
		return fmt.Errorf("publish generation failed: %w", err)
	}
	logger.Infof(ctx, "full KB rebuild published: kb=%s generation=%d", kb.ID, payload.Generation)
	return nil
}

func (s *kbFullRebuildService) createStage(
	ctx context.Context, source *types.KnowledgeBase, payload types.KBFullRebuildPayload,
) (*types.KnowledgeBase, error) {
	stage := *source
	stage.ID = uuid.NewString()
	stage.Name = fmt.Sprintf("__zealrag_rebuild_%s_%d", source.ID, payload.Generation)
	stage.IsTemporary = true
	stage.ActiveGeneration = payload.Generation
	if payload.EmbeddingModelID != "" {
		stage.EmbeddingModelID = payload.EmbeddingModelID
	}
	if payload.KnowledgeQAModelID != "" {
		stage.SummaryModelID = payload.KnowledgeQAModelID
	}
	stage.BuildingGeneration = nil
	stage.RebuildStageID = ""
	stage.RebuildStatus = types.KBRebuildStatusIdle
	stage.RebuildError = ""
	stage.RebuildStartedAt = nil
	stage.RebuildCompletedAt = nil
	stage.CreatedAt = time.Now().UTC()
	stage.UpdatedAt = stage.CreatedAt
	stage.DeletedAt = gorm.DeletedAt{}
	stage.IndexingStrategy.VectorEnabled = true
	stage.IndexingStrategy.KeywordEnabled = true
	if err := s.kbRepo.CreateKnowledgeBase(ctx, &stage); err != nil {
		return nil, fmt.Errorf("create staging knowledge base: %w", err)
	}
	return &stage, nil
}

func (s *kbFullRebuildService) markRunning(
	ctx context.Context, payload types.KBFullRebuildPayload, stageID string,
) error {
	result := s.db.WithContext(ctx).Table("knowledge_bases").
		Where("id = ? AND building_generation = ?", payload.KnowledgeBaseID, payload.Generation).
		Updates(map[string]interface{}{
			"rebuild_stage_id": stageID,
			"rebuild_status":   types.KBRebuildStatusRunning,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("rebuild was superseded before it started")
	}
	return nil
}

func (s *kbFullRebuildService) listSourceChunks(
	ctx context.Context, tenantID uint64, kbID string,
) ([]*types.Chunk, error) {
	return s.listSourceChunksWithDB(ctx, s.db, tenantID, kbID)
}

func (s *kbFullRebuildService) listSourceChunksWithDB(
	ctx context.Context, db *gorm.DB, tenantID uint64, kbID string,
) ([]*types.Chunk, error) {
	var chunks []*types.Chunk
	err := db.WithContext(ctx).
		Where("tenant_id = ? AND knowledge_base_id = ?", tenantID, kbID).
		Where("status IN ?", []int{int(types.ChunkStatusDefault), int(types.ChunkStatusIndexed)}).
		Where("is_enabled = ?", true).
		Where("chunk_type <> ?", types.ChunkTypeParentText).
		Order("knowledge_id ASC, chunk_index ASC").Find(&chunks).Error
	if err != nil {
		return nil, fmt.Errorf("list source chunks: %w", err)
	}
	return chunks, nil
}

func (s *kbFullRebuildService) rebuildVectors(
	ctx context.Context,
	source *types.KnowledgeBase,
	stage *types.KnowledgeBase,
	chunks []*types.Chunk,
) (int, error) {
	embedder, err := s.modelService.GetEmbeddingModel(ctx, stage.EmbeddingModelID)
	if err != nil {
		return 0, fmt.Errorf("load embedding model: %w", err)
	}
	engine, err := retriever.CreateRetrieveEngineForKB(
		ctx, s.retrieveEngine, s.ownership, stage.TenantID, stage.VectorStoreID,
	)
	if err != nil {
		return 0, fmt.Errorf("initialize pgvector engine: %w", err)
	}

	chunkIDs := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		if chunk != nil && strings.TrimSpace(chunk.Content) != "" {
			chunkIDs = append(chunkIDs, chunk.ID)
		}
	}
	reusedIDs := make(map[string]struct{})
	if source.EmbeddingModelID == stage.EmbeddingModelID {
		reusedIDs, err = s.copyReusableEmbeddings(
			ctx, source.ID, stage.ID, chunkIDs, embedder.GetDimensions(),
		)
		if err != nil {
			return 0, fmt.Errorf("copy reusable embeddings: %w", err)
		}
	}

	infos := make([]*types.IndexInfo, 0, kbRebuildVectorBatch)
	generated := 0
	flush := func() error {
		if len(infos) == 0 {
			return nil
		}
		if err := engine.BatchIndex(ctx, embedder, infos); err != nil {
			return err
		}
		generated += len(infos)
		infos = infos[:0]
		return nil
	}
	for _, chunk := range chunks {
		content := strings.TrimSpace(chunk.Content)
		if content == "" {
			continue
		}
		if _, reused := reusedIDs[chunk.ID]; reused {
			continue
		}
		infos = append(infos, &types.IndexInfo{
			Content: content, SourceID: chunk.ID, SourceType: types.ChunkSourceType,
			ChunkID: chunk.ID, KnowledgeID: chunk.KnowledgeID,
			KnowledgeBaseID: stage.ID, IsEnabled: chunk.IsEnabled,
			IsRecommended: chunk.Flags.HasFlag(types.ChunkFlagRecommended),
		})
		if len(infos) >= kbRebuildVectorBatch {
			if err := flush(); err != nil {
				return 0, err
			}
		}
	}
	if err := flush(); err != nil {
		return 0, err
	}
	indexed := len(reusedIDs) + generated
	logger.Infof(ctx, "full rebuild embedding reuse: reused=%d generated=%d total=%d", len(reusedIDs), generated, indexed)
	if indexed == 0 {
		return 0, errors.New("all source chunks have empty content")
	}
	return indexed, nil
}

func (s *kbFullRebuildService) copyReusableEmbeddings(
	ctx context.Context,
	sourceKBID string,
	stageID string,
	chunkIDs []string,
	dimension int,
) (map[string]struct{}, error) {
	reused := make(map[string]struct{})
	if len(chunkIDs) == 0 || dimension <= 0 {
		return reused, nil
	}

	// The caller only reaches this method when the source and target model IDs
	// match. Missing rows are generated normally after this copy.
	const copySQL = `
		INSERT INTO embeddings (
			created_at, updated_at, source_id, source_type, chunk_id,
			knowledge_id, knowledge_base_id, content, dimension, embedding,
			is_enabled
		)
		SELECT CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, source_id, source_type, chunk_id,
			knowledge_id, ?, content, dimension, embedding, is_enabled
		FROM embeddings
		WHERE knowledge_base_id = ?
		  AND source_type = ?
		  AND dimension = ?
		  AND embedding IS NOT NULL
		  AND source_id IN ?
		ON CONFLICT (knowledge_base_id, source_id, source_type) DO NOTHING`
	if err := s.db.WithContext(ctx).Exec(
		copySQL, stageID, sourceKBID, int(types.ChunkSourceType), dimension, chunkIDs,
	).Error; err != nil {
		return nil, err
	}

	var rows []struct{ SourceID string }
	if err := s.db.WithContext(ctx).Table("embeddings").
		Select("source_id").
		Where(
			"knowledge_base_id = ? AND source_type = ? AND dimension = ? AND source_id IN ?",
			stageID, int(types.ChunkSourceType), dimension, chunkIDs,
		).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		reused[row.SourceID] = struct{}{}
	}
	return reused, nil
}

type chunkSnapshot struct {
	Count       int
	Fingerprint string
}

func fingerprintSourceChunks(chunks []*types.Chunk) chunkSnapshot {
	hash := sha256.New()
	for _, chunk := range chunks {
		fmt.Fprintf(hash, "%s\x00%d\x00%d\x00%t\x00%s\x00",
			chunk.ID, chunk.UpdatedAt.UnixNano(), chunk.Status, chunk.IsEnabled, chunk.Content)
	}
	return chunkSnapshot{Count: len(chunks), Fingerprint: hex.EncodeToString(hash.Sum(nil))}
}

func (s *kbFullRebuildService) validateStage(
	ctx context.Context, stageID string, expectedVectorCount int,
) error {
	var vectorCount int64
	if err := s.db.WithContext(ctx).Table("embeddings").
		Where("knowledge_base_id = ?", stageID).Count(&vectorCount).Error; err != nil {
		return err
	}
	if vectorCount < int64(expectedVectorCount) {
		return fmt.Errorf("staging vectors incomplete: got %d, expected at least %d", vectorCount, expectedVectorCount)
	}
	return nil
}

func (s *kbFullRebuildService) publish(
	ctx context.Context, payload types.KBFullRebuildPayload, stageID string, snapshot chunkSnapshot,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked types.KnowledgeBase
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ?", payload.KnowledgeBaseID).First(&locked).Error; err != nil {
			return err
		}
		if locked.BuildingGeneration == nil || *locked.BuildingGeneration != payload.Generation ||
			locked.RebuildStageID != stageID {
			return errors.New("rebuild was superseded before publish")
		}

		liveID := payload.KnowledgeBaseID
		currentChunks, err := s.listSourceChunksWithDB(ctx, tx, payload.TenantID, liveID)
		if err != nil {
			return err
		}
		currentSnapshot := fingerprintSourceChunks(currentChunks)
		if currentSnapshot != snapshot {
			return errors.New("source documents changed during rebuild; start a new full rebuild")
		}
		statements := []struct {
			sql  string
			args []interface{}
		}{
			{"DELETE FROM embeddings WHERE knowledge_base_id = ?", []interface{}{liveID}},
			{"UPDATE embeddings SET knowledge_base_id = ? WHERE knowledge_base_id = ?", []interface{}{liveID, stageID}},
		}
		for _, statement := range statements {
			if err := tx.Exec(statement.sql, statement.args...).Error; err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		updates := map[string]interface{}{
			"active_generation":    payload.Generation,
			"building_generation":  nil,
			"rebuild_stage_id":     "",
			"rebuild_status":       types.KBRebuildStatusSucceeded,
			"rebuild_error":        "",
			"rebuild_completed_at": now,
		}
		if payload.EmbeddingModelID != "" {
			updates["embedding_model_id"] = payload.EmbeddingModelID
		}
		if payload.KnowledgeQAModelID != "" {
			updates["summary_model_id"] = payload.KnowledgeQAModelID
		}
		result := tx.Table("knowledge_bases").
			Where("id = ? AND building_generation = ?", liveID, payload.Generation).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("active generation switch affected no rows")
		}
		return tx.Unscoped().Where("id = ?", stageID).Delete(&types.KnowledgeBase{}).Error
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})
}

func (s *kbFullRebuildService) markFailed(
	ctx context.Context, payload types.KBFullRebuildPayload, rebuildErr error,
) {
	message := strings.TrimSpace(rebuildErr.Error())
	if len(message) > 4000 {
		message = message[:4000]
	}
	now := time.Now().UTC()
	result := s.db.WithContext(ctx).Table("knowledge_bases").
		Where("id = ? AND building_generation = ?", payload.KnowledgeBaseID, payload.Generation).
		Updates(map[string]interface{}{
			"rebuild_status":       types.KBRebuildStatusFailed,
			"rebuild_error":        message,
			"rebuild_completed_at": now,
		})
	if result.Error != nil {
		logger.Errorf(ctx, "failed to persist rebuild error: %v", result.Error)
	}
}
