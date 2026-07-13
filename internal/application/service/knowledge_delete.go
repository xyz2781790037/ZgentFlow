package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"golang.org/x/sync/errgroup"
)

func (s *knowledgeService) TrashKnowledge(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	if _, err := s.repo.GetKnowledgeByID(ctx, tenantID, id); err != nil {
		return err
	}
	return s.repo.DeleteKnowledge(ctx, tenantID, id)
}

func (s *knowledgeService) TrashKnowledgeList(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	tenantID := types.MustTenantIDFromContext(ctx)
	rows, err := s.repo.GetKnowledgeBatch(ctx, tenantID, ids)
	if err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return errors.New("one or more knowledge entries not found")
	}
	return s.repo.DeleteKnowledgeList(ctx, tenantID, ids)
}

func (s *knowledgeService) ListTrashedKnowledge(ctx context.Context) ([]*types.Knowledge, error) {
	return s.repo.ListTrashedKnowledge(ctx, types.MustTenantIDFromContext(ctx))
}

func (s *knowledgeService) RestoreKnowledge(ctx context.Context, id string) error {
	tenantID := types.MustTenantIDFromContext(ctx)
	knowledge, err := s.repo.GetTrashedKnowledge(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if _, err := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID); err != nil {
		return errors.New("restore the parent knowledge base first")
	}
	return s.repo.RestoreKnowledge(ctx, tenantID, id)
}

func (s *knowledgeService) PurgeTrashedKnowledge(ctx context.Context, id string) error {
	return s.PurgeTrashedKnowledgeForTenant(ctx, types.MustTenantIDFromContext(ctx), id)
}

func (s *knowledgeService) PurgeTrashedKnowledgeForTenant(
	ctx context.Context, tenantID uint64, id string,
) error {
	tenant, err := s.tenantRepo.GetTenantByID(ctx, tenantID)
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, tenantID)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
	if _, err := s.repo.GetTrashedKnowledge(ctx, tenantID, id); err != nil {
		return err
	}

	// Re-activate only for the duration of the existing, thoroughly tested
	// cleanup pipeline, then remove the soft-deleted row permanently.
	if err := s.repo.RestoreKnowledge(ctx, tenantID, id); err != nil {
		return err
	}
	if err := s.DeleteKnowledge(ctx, id); err != nil {
		_ = s.repo.DeleteKnowledge(ctx, tenantID, id)
		return err
	}
	return s.repo.PurgeKnowledge(ctx, tenantID, id)
}

// DeleteKnowledge deletes a knowledge entry and all related resources
func (s *knowledgeService) DeleteKnowledge(ctx context.Context, id string) error {
	// Get the knowledge entry
	knowledge, err := s.repo.GetKnowledgeByID(ctx, ctx.Value(types.TenantIDContextKey).(uint64), id)
	if err != nil {
		return err
	}

	// Mark as deleting first to prevent async task conflicts
	// This ensures that any running async tasks will detect the deletion and abort
	originalStatus := knowledge.ParseStatus
	knowledge.ParseStatus = types.ParseStatusDeleting
	knowledge.UpdatedAt = time.Now()
	if err := s.repo.UpdateKnowledge(ctx, knowledge); err != nil {
		logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge failed to mark as deleting")
		// Continue with deletion even if marking fails
	} else {
		logger.Infof(ctx, "Marked knowledge %s as deleting (previous status: %s)", id, originalStatus)
	}

	// Best-effort: purge any queued downstream tasks for this knowledge
	// (multimodal / post-process / question / summary).
	// Worker checkpoints already drop them on the floor, but dequeuing
	// here avoids waking workers just to no-op when the parse was still
	// in flight at delete time. Completed rows have no queued descendants
	// (no queued descendants anyway).
	if originalStatus == types.ParseStatusPending ||
		originalStatus == types.ParseStatusProcessing {
		s.dequeueKnowledgeTasks(ctx, id)
	}

	// Resolve file service for this KB before spawning goroutines
	kb, _ := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
	kbFileSvc := s.resolveFileService(ctx, kb)

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	wg := errgroup.Group{}
	// Delete knowledge embeddings from vector store.
	// Skip entirely when the knowledge has no embedding model (e.g. Wiki-only KB):
	// nothing was ever written to the vector store, so there is nothing to delete,
	// and GetEmbeddingModel would fail with "model ID cannot be empty".
	if strings.TrimSpace(knowledge.EmbeddingModelID) != "" {
		wg.Go(func() error {
			// kb was already loaded above for resolveFileService — reuse its
			// VectorStoreID for engine routing.
			var boundStoreID *string
			if kb != nil {
				boundStoreID = kb.VectorStoreID
			}
			retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
				ctx,
				s.retrieveEngine,
				s.ownership,
				tenantID,
				boundStoreID,
			)
			if err != nil {
				logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete knowledge embedding failed")
				return err
			}
			embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, knowledge.EmbeddingModelID)
			if err != nil {
				logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete knowledge embedding failed")
				return err
			}
			if err := retrieveEngine.DeleteByKnowledgeIDList(ctx, []string{knowledge.ID}, embeddingModel.GetDimensions(), knowledge.Type); err != nil {
				logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete knowledge embedding failed")
				return err
			}
			return nil
		})
	} else {
		logger.Infof(ctx, "Knowledge %s has no embedding model, skipping vector store cleanup", knowledge.ID)
	}

	// Delete all chunks associated with this knowledge
	wg.Go(func() error {
		if err := s.chunkService.DeleteChunksByKnowledgeID(ctx, knowledge.ID); err != nil {
			logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete chunks failed")
			return err
		}
		return nil
	})

	// Delete the physical source file if it exists.
	wg.Go(func() error {
		if knowledge.FilePath != "" {
			if err := kbFileSvc.DeleteFile(ctx, knowledge.FilePath); err != nil {
				logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete file failed")
			}
		}
		tenantInfo := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
		tenantInfo.StorageUsed -= knowledge.StorageSize
		if err := s.tenantRepo.AdjustStorageUsed(ctx, tenantInfo.ID, -knowledge.StorageSize); err != nil {
			logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge update tenant storage used failed")
		}
		return nil
	})

	if err = wg.Wait(); err != nil {
		return err
	}
	// Delete the knowledge entry itself from the database
	return s.repo.DeleteKnowledge(ctx, ctx.Value(types.TenantIDContextKey).(uint64), id)
}

// DeleteKnowledgeList deletes a knowledge entry and all related resources
func (s *knowledgeService) DeleteKnowledgeList(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	// 1. Get the knowledge entry
	tenantInfo := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	knowledgeList, err := s.repo.GetKnowledgeBatch(ctx, tenantInfo.ID, ids)
	if err != nil {
		return err
	}

	// Mark all as deleting first to prevent async task conflicts.
	// Remember which entries still had queued / in-flight downstream tasks
	// so we can dequeue them in one pass after marking.
	var inFlightIDs []string
	for _, knowledge := range knowledgeList {
		prev := knowledge.ParseStatus
		knowledge.ParseStatus = types.ParseStatusDeleting
		knowledge.UpdatedAt = time.Now()
		if err := s.repo.UpdateKnowledge(ctx, knowledge); err != nil {
			logger.GetLogger(ctx).WithField("error", err).WithField("knowledge_id", knowledge.ID).
				Errorf("DeleteKnowledgeList failed to mark as deleting")
			// Continue with deletion even if marking fails
		}
		if prev == types.ParseStatusPending || prev == types.ParseStatusProcessing {
			inFlightIDs = append(inFlightIDs, knowledge.ID)
		}
	}
	logger.Infof(ctx, "Marked %d knowledge entries as deleting", len(knowledgeList))

	// Best-effort dequeue of downstream tasks for in-flight entries.
	// See DeleteKnowledge for the rationale; loop is per-knowledge because
	// the inspector only filters by knowledge_id, not by ID set.
	for _, kid := range inFlightIDs {
		s.dequeueKnowledgeTasks(ctx, kid)
	}

	// Pre-resolve file services per KB so goroutines don't need DB access
	kbFileServices := make(map[string]interfaces.FileService)
	for _, knowledge := range knowledgeList {
		if _, ok := kbFileServices[knowledge.KnowledgeBaseID]; !ok {
			kb, _ := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
			kbFileServices[knowledge.KnowledgeBaseID] = s.resolveFileService(ctx, kb)
		}
	}

	wg := errgroup.Group{}
	// 2. Delete knowledge embeddings from vector store
	wg.Go(func() error {
		tenantID := types.MustTenantIDFromContext(ctx)
		// Batch cleanup spans multiple KBs that may be bound to different
		// VectorStores; routing this batch through tenant effective engines
		// keeps the legacy behavior intact.
		// TODO: fan out the batch per-store using each KB's own
		// VectorStoreID so cleanup hits the right backend for bound KBs.
		retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
			ctx, s.retrieveEngine, s.ownership, tenantID, nil)
		if err != nil {
			logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete knowledge embedding failed")
			return err
		}
		// Group by EmbeddingModelID and Type
		type groupKey struct {
			EmbeddingModelID string
			Type             string
		}
		group := map[groupKey][]string{}
		for _, knowledge := range knowledgeList {
			key := groupKey{EmbeddingModelID: knowledge.EmbeddingModelID, Type: knowledge.Type}
			group[key] = append(group[key], knowledge.ID)
		}
		for key, knowledgeIDs := range group {
			// Wiki-only knowledge never had embeddings written to the vector store,
			// and its EmbeddingModelID is intentionally empty. Skip the whole group
			// to avoid the spurious "model ID cannot be empty" failure.
			if strings.TrimSpace(key.EmbeddingModelID) == "" {
				logger.Infof(ctx, "Skipping vector store cleanup for %d knowledge entries without embedding model", len(knowledgeIDs))
				continue
			}
			embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, key.EmbeddingModelID)
			if err != nil {
				logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge get embedding model failed")
				return err
			}
			if err := retrieveEngine.DeleteByKnowledgeIDList(ctx, knowledgeIDs, embeddingModel.GetDimensions(), key.Type); err != nil {
				logger.GetLogger(ctx).
					WithField("error", err).
					Errorf("DeleteKnowledge delete knowledge embedding failed")
				return err
			}
		}
		return nil
	})

	// Delete all chunks associated with this knowledge.
	wg.Go(func() error {
		if err := s.chunkService.DeleteByKnowledgeList(ctx, ids); err != nil {
			logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete chunks failed")
			return err
		}
		return nil
	})

	// Delete the physical source files and adjust storage.
	wg.Go(func() error {
		storageAdjust := int64(0)
		for _, knowledge := range knowledgeList {
			if knowledge.FilePath != "" {
				fSvc := kbFileServices[knowledge.KnowledgeBaseID]
				if err := fSvc.DeleteFile(ctx, knowledge.FilePath); err != nil {
					logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge delete file failed")
				}
			}
			storageAdjust -= knowledge.StorageSize
		}
		tenantInfo.StorageUsed += storageAdjust
		if err := s.tenantRepo.AdjustStorageUsed(ctx, tenantInfo.ID, storageAdjust); err != nil {
			logger.GetLogger(ctx).WithField("error", err).Errorf("DeleteKnowledge update tenant storage used failed")
		}
		return nil
	})

	if err = wg.Wait(); err != nil {
		return err
	}
	// 6. Delete the knowledge entry itself from the database
	return s.repo.DeleteKnowledgeList(ctx, tenantInfo.ID, ids)
}

func (s *knowledgeService) cleanupKnowledgeResources(ctx context.Context, knowledge *types.Knowledge) error {
	logger.GetLogger(ctx).Infof("Cleaning knowledge resources before manual update, knowledge ID: %s", knowledge.ID)

	var cleanupErr error

	tenantInfo := ctx.Value(types.TenantInfoContextKey).(*types.Tenant)
	if knowledge.EmbeddingModelID != "" {
		// Load KB to discover its VectorStoreID binding. Falls back to tenant
		// effective engines if the KB has no binding or the load fails.
		//
		// Silent fallback risk: if a bound KB fails to load here due to a
		// transient DB error, the cleanup will delete from env engines and
		// leave orphan vectors in the bound store. Warn so operators can spot it.
		var boundStoreID *string
		if kb, loadErr := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID); loadErr == nil && kb != nil {
			boundStoreID = kb.VectorStoreID
		} else if loadErr != nil {
			logger.GetLogger(ctx).WithField("error", loadErr).WithField("knowledge_base_id", knowledge.KnowledgeBaseID).
				Warnf("cleanupKnowledgeResources: failed to load KB for vector store resolution; falling back to tenant effective engines")
		}
		retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
			ctx, s.retrieveEngine, s.ownership, tenantInfo.ID, boundStoreID)
		if err != nil {
			logger.GetLogger(ctx).WithField("error", err).Error("Failed to init retrieve engine during cleanup")
			cleanupErr = errors.Join(cleanupErr, err)
		} else {
			embeddingModel, modelErr := s.modelService.GetEmbeddingModel(ctx, knowledge.EmbeddingModelID)
			if modelErr != nil {
				logger.GetLogger(ctx).WithField("error", modelErr).Error("Failed to get embedding model during cleanup")
				cleanupErr = errors.Join(cleanupErr, modelErr)
			} else {
				if err := retrieveEngine.DeleteByKnowledgeIDList(ctx, []string{knowledge.ID}, embeddingModel.GetDimensions(), knowledge.Type); err != nil {
					logger.GetLogger(ctx).WithField("error", err).Error("Failed to delete manual knowledge index")
					cleanupErr = errors.Join(cleanupErr, err)
				}
			}
		}
	}

	if err := s.chunkService.DeleteChunksByKnowledgeID(ctx, knowledge.ID); err != nil {
		logger.GetLogger(ctx).WithField("error", err).Error("Failed to delete manual knowledge chunks")
		cleanupErr = errors.Join(cleanupErr, err)
	}

	if knowledge.StorageSize > 0 {
		tenantInfo.StorageUsed -= knowledge.StorageSize
		if tenantInfo.StorageUsed < 0 {
			tenantInfo.StorageUsed = 0
		}
		if err := s.tenantRepo.AdjustStorageUsed(ctx, tenantInfo.ID, -knowledge.StorageSize); err != nil {
			logger.GetLogger(ctx).WithField("error", err).Error("Failed to adjust storage usage during manual cleanup")
			cleanupErr = errors.Join(cleanupErr, err)
		}
		knowledge.StorageSize = 0
	}

	return cleanupErr
}

// ProcessKnowledgeListDelete handles Asynq knowledge list delete tasks
func (s *knowledgeService) ProcessKnowledgeListDelete(ctx context.Context, t *asynq.Task) error {
	var payload types.KnowledgeListDeletePayload
	if err := json.Unmarshal(t.Payload(), &payload); err != nil {
		logger.Errorf(ctx, "Failed to unmarshal knowledge list delete payload: %v", err)
		return err
	}

	logger.Infof(ctx, "Processing knowledge list delete task for %d knowledge items", len(payload.KnowledgeIDs))

	// Get tenant info
	tenant, err := s.tenantRepo.GetTenantByID(ctx, payload.TenantID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get tenant %d: %v", payload.TenantID, err)
		return err
	}

	// Set context values
	ctx = context.WithValue(ctx, types.TenantIDContextKey, payload.TenantID)
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)

	// Delete knowledge list
	if err := s.DeleteKnowledgeList(ctx, payload.KnowledgeIDs); err != nil {
		logger.Errorf(ctx, "Failed to delete knowledge list: %v", err)
		return err
	}

	logger.Infof(ctx, "Successfully deleted %d knowledge items", len(payload.KnowledgeIDs))
	return nil
}
