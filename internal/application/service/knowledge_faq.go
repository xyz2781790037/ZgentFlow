package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	werrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// ListFAQEntries lists FAQ entries under a FAQ knowledge base.
func (s *knowledgeService) ListFAQEntries(ctx context.Context,
	kbID string, page *types.Pagination, keyword string, searchField string, sortOrder string,
) (*types.PageResult, error) {
	if page == nil {
		page = &types.Pagination{}
	}
	keyword = strings.TrimSpace(keyword)
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	faqKnowledge, err := s.findFAQKnowledge(ctx, tenantID, kb.ID)
	if err != nil {
		return nil, err
	}
	if faqKnowledge == nil {
		return types.NewPageResult(0, page, []*types.FAQEntry{}), nil
	}

	chunkType := []types.ChunkType{types.ChunkTypeFAQ}
	chunks, total, err := s.chunkRepo.ListPagedChunksByKnowledgeID(
		ctx, tenantID, faqKnowledge.ID, page, chunkType, keyword, searchField, sortOrder, types.KnowledgeTypeFAQ,
	)
	if err != nil {
		return nil, err
	}

	kb.EnsureDefaults()
	entries := make([]*types.FAQEntry, 0, len(chunks))
	for _, chunk := range chunks {
		entry, err := s.chunkToFAQEntry(chunk, kb)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return types.NewPageResult(total, page, entries), nil
}

// CreateFAQEntry creates a single FAQ entry synchronously.
func (s *knowledgeService) CreateFAQEntry(ctx context.Context,
	kbID string, payload *types.FAQEntryPayload,
) (*types.FAQEntry, error) {
	if payload == nil {
		return nil, werrors.NewBadRequestError("请求体不能为空")
	}

	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	kb.EnsureDefaults()

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	// 验证并清理输入
	meta, err := sanitizeFAQEntryPayload(payload)
	if err != nil {
		return nil, err
	}

	// 检查标准问和相似问是否与其他条目重复
	if err := s.checkFAQQuestionDuplicate(ctx, tenantID, kb.ID, "", meta); err != nil {
		return nil, err
	}

	// 确保FAQ Knowledge存在
	faqKnowledge, err := s.ensureFAQKnowledge(ctx, tenantID, kb)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure FAQ knowledge: %w", err)
	}

	// 获取索引模式
	indexMode := types.FAQIndexModeQuestionOnly
	if kb.FAQConfig != nil && kb.FAQConfig.IndexMode != "" {
		indexMode = kb.FAQConfig.IndexMode
	}

	// 获取embedding模型
	embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, kb.EmbeddingModelID)
	if err != nil {
		return nil, fmt.Errorf("failed to get embedding model: %w", err)
	}

	// 创建chunk
	isEnabled := true
	if payload.IsEnabled != nil {
		isEnabled = *payload.IsEnabled
	}
	// 默认可推荐
	flags := types.ChunkFlagRecommended
	if payload.IsRecommended != nil && !*payload.IsRecommended {
		flags = 0
	}

	chunk := &types.Chunk{
		ID:              uuid.New().String(),
		TenantID:        tenantID,
		KnowledgeID:     faqKnowledge.ID,
		KnowledgeBaseID: kb.ID,
		Content:         buildFAQChunkContent(meta, indexMode),
		IsEnabled:       isEnabled,
		Flags:           flags,
		ChunkType:       types.ChunkTypeFAQ,
		Status:          int(types.ChunkStatusStored),
	}
	// 如果指定了 ID（用于数据迁移），设置 SeqID
	if payload.ID != nil && *payload.ID > 0 {
		chunk.SeqID = *payload.ID
	}

	if err := chunk.SetFAQMetadata(meta); err != nil {
		return nil, fmt.Errorf("failed to set FAQ metadata: %w", err)
	}

	// 保存chunk
	if err := s.chunkService.CreateChunks(ctx, []*types.Chunk{chunk}); err != nil {
		return nil, fmt.Errorf("failed to create chunk: %w", err)
	}

	// 索引chunk
	if err := s.indexFAQChunks(ctx, kb, faqKnowledge, []*types.Chunk{chunk}, embeddingModel, true, false); err != nil {
		// 如果索引失败，删除已创建的chunk
		_ = s.chunkService.DeleteChunk(ctx, chunk.ID)
		return nil, fmt.Errorf("failed to index chunk: %w", err)
	}

	// 更新chunk状态为已索引
	chunk.Status = int(types.ChunkStatusIndexed)
	if err := s.chunkService.UpdateChunk(ctx, chunk); err != nil {
		return nil, fmt.Errorf("failed to update chunk status: %w", err)
	}

	// 转换为FAQEntry返回
	entry, err := s.chunkToFAQEntry(chunk, kb)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// GetFAQEntry retrieves a single FAQ entry by seq_id.
func (s *knowledgeService) GetFAQEntry(ctx context.Context,
	kbID string, entrySeqID int64,
) (*types.FAQEntry, error) {
	if entrySeqID <= 0 {
		return nil, werrors.NewBadRequestError("条目ID不能为空")
	}

	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	kb.EnsureDefaults()

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	// 获取chunk by seq_id
	chunk, err := s.chunkRepo.GetChunkBySeqID(ctx, tenantID, entrySeqID)
	if err != nil {
		return nil, werrors.NewNotFoundError("FAQ条目不存在")
	}

	// 验证chunk属于当前知识库
	if chunk.KnowledgeBaseID != kb.ID || chunk.TenantID != tenantID {
		return nil, werrors.NewNotFoundError("FAQ条目不存在")
	}

	// 验证是FAQ类型
	if chunk.ChunkType != types.ChunkTypeFAQ {
		return nil, werrors.NewNotFoundError("FAQ条目不存在")
	}

	// 转换为FAQEntry返回
	entry, err := s.chunkToFAQEntry(chunk, kb)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// UpdateFAQEntry updates a single FAQ entry.
func (s *knowledgeService) UpdateFAQEntry(ctx context.Context,
	kbID string, entrySeqID int64, payload *types.FAQEntryPayload,
) (*types.FAQEntry, error) {
	if payload == nil {
		return nil, werrors.NewBadRequestError("请求体不能为空")
	}
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	kb.EnsureDefaults()
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	chunk, err := s.chunkRepo.GetChunkBySeqID(ctx, tenantID, entrySeqID)
	if err != nil {
		return nil, werrors.NewNotFoundError("FAQ条目不存在")
	}
	if chunk.KnowledgeBaseID != kb.ID {
		return nil, werrors.NewForbiddenError("无权操作该 FAQ 条目")
	}
	if chunk.ChunkType != types.ChunkTypeFAQ {
		return nil, werrors.NewBadRequestError("仅支持更新 FAQ 条目")
	}
	meta, err := sanitizeFAQEntryPayload(payload)
	if err != nil {
		return nil, err
	}

	// 检查标准问和相似问是否与其他条目重复
	if err := s.checkFAQQuestionDuplicate(ctx, tenantID, kb.ID, chunk.ID, meta); err != nil {
		return nil, err
	}

	// 获取旧的相似问列表，用于增量更新
	var oldSimilarQuestions []string
	var oldStandardQuestion string
	var oldAnswers []string
	questionIndexMode := types.FAQQuestionIndexModeCombined
	if kb.FAQConfig != nil && kb.FAQConfig.QuestionIndexMode != "" {
		questionIndexMode = kb.FAQConfig.QuestionIndexMode
	}
	if existing, err := chunk.FAQMetadata(); err == nil && existing != nil {
		meta.Version = existing.Version + 1
		// 保存旧的内容用于增量比较
		if questionIndexMode == types.FAQQuestionIndexModeSeparate {
			oldSimilarQuestions = existing.SimilarQuestions
			oldStandardQuestion = existing.StandardQuestion
			oldAnswers = existing.Answers
		}
	}
	if err := chunk.SetFAQMetadata(meta); err != nil {
		return nil, err
	}
	// 获取索引模式
	indexMode := types.FAQIndexModeQuestionOnly
	if kb.FAQConfig != nil && kb.FAQConfig.IndexMode != "" {
		indexMode = kb.FAQConfig.IndexMode
	}
	chunk.Content = buildFAQChunkContent(meta, indexMode)

	if payload.IsEnabled != nil {
		chunk.IsEnabled = *payload.IsEnabled
	}
	// 处理推荐状态
	if payload.IsRecommended != nil {
		if *payload.IsRecommended {
			chunk.Flags = chunk.Flags.SetFlag(types.ChunkFlagRecommended)
		} else {
			chunk.Flags = chunk.Flags.ClearFlag(types.ChunkFlagRecommended)
		}
	}
	chunk.UpdatedAt = time.Now()
	if err := s.chunkService.UpdateChunk(ctx, chunk); err != nil {
		return nil, err
	}

	// Note: We don't need to call BatchUpdateChunkEnabledStatus here because
	// indexFAQChunks will delete old vectors and re-insert with the latest chunk data
	// (including the updated is_enabled status). Calling both would cause version conflicts.

	faqKnowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, chunk.KnowledgeID)
	if err != nil {
		return nil, err
	}

	embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, kb.EmbeddingModelID)
	if err != nil {
		return nil, err
	}

	// 增量索引优化：只对变化的内容进行索引操作
	if questionIndexMode == types.FAQQuestionIndexModeSeparate && len(oldSimilarQuestions) > 0 {
		// 分别索引模式下的增量更新
		if err := s.incrementalIndexFAQEntry(ctx, kb, faqKnowledge, chunk, embeddingModel,
			oldStandardQuestion, oldSimilarQuestions, oldAnswers, meta); err != nil {
			return nil, err
		}
	} else {
		// Combined 模式或首次创建，使用全量索引
		// 增量删除：只删除被移除的相似问索引
		oldSimilarQuestionCount := len(oldSimilarQuestions)
		newSimilarQuestionCount := len(meta.SimilarQuestions)
		if questionIndexMode == types.FAQQuestionIndexModeSeparate && oldSimilarQuestionCount > newSimilarQuestionCount {
			retrieveEngine, engineErr := retriever.CreateRetrieveEngineForKB(
				ctx, s.retrieveEngine, s.ownership, types.MustTenantIDFromContext(ctx), kb.VectorStoreID)
			if engineErr == nil {
				sourceIDsToDelete := make([]string, 0, oldSimilarQuestionCount-newSimilarQuestionCount)
				for i := newSimilarQuestionCount; i < oldSimilarQuestionCount; i++ {
					sourceIDsToDelete = append(sourceIDsToDelete, fmt.Sprintf("%s-%d", chunk.ID, i))
				}
				if len(sourceIDsToDelete) > 0 {
					logger.Debugf(ctx, "UpdateFAQEntry: incremental delete %d obsolete source IDs", len(sourceIDsToDelete))
					if delErr := retrieveEngine.DeleteBySourceIDList(ctx, sourceIDsToDelete, embeddingModel.GetDimensions(), types.KnowledgeTypeFAQ); delErr != nil {
						logger.Warnf(ctx, "UpdateFAQEntry: failed to delete obsolete source IDs: %v", delErr)
					}
				}
			}
		}

		// 使用 needDelete=false，因为 EFPutDocument 会自动覆盖相同 SourceID 的文档
		if err := s.indexFAQChunks(ctx, kb, faqKnowledge, []*types.Chunk{chunk}, embeddingModel, false, false); err != nil {
			return nil, err
		}
	}

	// 转换为FAQEntry返回
	entry, err := s.chunkToFAQEntry(chunk, kb)
	if err != nil {
		return nil, err
	}

	return entry, nil
}

// AddSimilarQuestions adds similar questions to a FAQ entry.
// This will append the new questions to the existing similar questions list.
func (s *knowledgeService) AddSimilarQuestions(ctx context.Context,
	kbID string, entrySeqID int64, questions []string,
) (*types.FAQEntry, error) {
	if len(questions) == 0 {
		return nil, werrors.NewBadRequestError("相似问列表不能为空")
	}

	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	kb.EnsureDefaults()
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	// Get existing FAQ entry
	chunk, err := s.chunkRepo.GetChunkBySeqID(ctx, tenantID, entrySeqID)
	if err != nil {
		return nil, werrors.NewNotFoundError("FAQ条目不存在")
	}
	if chunk.KnowledgeBaseID != kb.ID {
		return nil, werrors.NewForbiddenError("无权操作该 FAQ 条目")
	}
	if chunk.ChunkType != types.ChunkTypeFAQ {
		return nil, werrors.NewBadRequestError("仅支持更新 FAQ 条目")
	}

	// Get existing metadata
	meta, err := chunk.FAQMetadata()
	if err != nil || meta == nil {
		return nil, werrors.NewBadRequestError("获取 FAQ 元数据失败")
	}

	// Deduplicate and sanitize new questions
	existingSet := make(map[string]struct{})
	for _, q := range meta.SimilarQuestions {
		existingSet[q] = struct{}{}
	}
	// Also add standard question to prevent duplicates
	existingSet[meta.StandardQuestion] = struct{}{}

	newQuestions := make([]string, 0, len(questions))
	for _, q := range questions {
		q = strings.TrimSpace(q)
		if q == "" {
			continue
		}
		if _, exists := existingSet[q]; exists {
			continue
		}
		existingSet[q] = struct{}{}
		newQuestions = append(newQuestions, q)
	}

	if len(newQuestions) == 0 {
		return s.chunkToFAQEntry(chunk, kb)
	}

	// Check for duplicates with other entries
	tempMeta := &types.FAQChunkMetadata{
		StandardQuestion: meta.StandardQuestion,
		SimilarQuestions: append(meta.SimilarQuestions, newQuestions...),
	}
	if err := s.checkFAQQuestionDuplicate(ctx, tenantID, kb.ID, chunk.ID, tempMeta); err != nil {
		return nil, err
	}

	// Update metadata
	oldSimilarQuestions := meta.SimilarQuestions
	meta.SimilarQuestions = append(meta.SimilarQuestions, newQuestions...)
	meta.Version++

	if err := chunk.SetFAQMetadata(meta); err != nil {
		return nil, err
	}

	// Update chunk content
	indexMode := types.FAQIndexModeQuestionOnly
	if kb.FAQConfig != nil && kb.FAQConfig.IndexMode != "" {
		indexMode = kb.FAQConfig.IndexMode
	}
	chunk.Content = buildFAQChunkContent(meta, indexMode)
	chunk.UpdatedAt = time.Now()

	if err := s.chunkService.UpdateChunk(ctx, chunk); err != nil {
		return nil, err
	}

	// Index new similar questions
	faqKnowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, chunk.KnowledgeID)
	if err != nil {
		return nil, err
	}

	embeddingModel, err := s.modelService.GetEmbeddingModel(ctx, kb.EmbeddingModelID)
	if err != nil {
		return nil, err
	}

	questionIndexMode := types.FAQQuestionIndexModeCombined
	if kb.FAQConfig != nil && kb.FAQConfig.QuestionIndexMode != "" {
		questionIndexMode = kb.FAQConfig.QuestionIndexMode
	}

	if questionIndexMode == types.FAQQuestionIndexModeSeparate {
		// Only index the new similar questions
		if err := s.incrementalIndexFAQEntry(ctx, kb, faqKnowledge, chunk, embeddingModel,
			meta.StandardQuestion, oldSimilarQuestions, meta.Answers, meta); err != nil {
			return nil, err
		}
	} else {
		// Combined mode, re-index the whole entry
		if err := s.indexFAQChunks(ctx, kb, faqKnowledge, []*types.Chunk{chunk}, embeddingModel, false, false); err != nil {
			return nil, err
		}
	}

	return s.chunkToFAQEntry(chunk, kb)
}

// UpdateFAQEntryStatus updates enable status for a FAQ entry.
func (s *knowledgeService) UpdateFAQEntryStatus(ctx context.Context,
	kbID string, entryID string, isEnabled bool,
) error {
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return err
	}
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	chunk, err := s.chunkRepo.GetChunkByID(ctx, tenantID, entryID)
	if err != nil {
		return err
	}
	if chunk.KnowledgeBaseID != kb.ID || chunk.ChunkType != types.ChunkTypeFAQ {
		return werrors.NewBadRequestError("仅支持更新 FAQ 条目")
	}
	if chunk.IsEnabled == isEnabled {
		return nil
	}
	chunk.IsEnabled = isEnabled
	chunk.UpdatedAt = time.Now()
	if err := s.chunkService.UpdateChunk(ctx, chunk); err != nil {
		return err
	}

	// Sync update to retriever engines
	chunkStatusMap := map[string]bool{chunk.ID: isEnabled}
	retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
		ctx, s.retrieveEngine, s.ownership, tenantID, kb.VectorStoreID)
	if err != nil {
		return err
	}
	if err := retrieveEngine.BatchUpdateChunkEnabledStatus(ctx, chunkStatusMap); err != nil {
		return err
	}

	return nil
}

// UpdateFAQEntryFieldsBatch updates enabled and recommended fields by entry ID.
func (s *knowledgeService) UpdateFAQEntryFieldsBatch(
	ctx context.Context,
	kbID string,
	req *types.FAQEntryFieldsBatchUpdate,
) error {
	if req == nil || len(req.ByID) == 0 {
		return nil
	}
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return err
	}
	tenantID := types.MustTenantIDFromContext(ctx)

	entrySeqIDs := make([]int64, 0, len(req.ByID))
	for entrySeqID := range req.ByID {
		entrySeqIDs = append(entrySeqIDs, entrySeqID)
	}
	chunks, err := s.chunkRepo.ListChunksBySeqID(ctx, tenantID, entrySeqIDs)
	if err != nil {
		return err
	}
	chunkBySeqID := make(map[int64]*types.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkBySeqID[chunk.SeqID] = chunk
	}

	enabledUpdates := make(map[string]bool)
	setFlags := make(map[string]types.ChunkFlags)
	clearFlags := make(map[string]types.ChunkFlags)
	chunksToUpdate := make([]*types.Chunk, 0)

	for entrySeqID, update := range req.ByID {
		chunk := chunkBySeqID[entrySeqID]
		if chunk == nil || chunk.KnowledgeBaseID != kb.ID || chunk.ChunkType != types.ChunkTypeFAQ {
			continue
		}
		if update.IsEnabled != nil && chunk.IsEnabled != *update.IsEnabled {
			chunk.IsEnabled = *update.IsEnabled
			chunk.UpdatedAt = time.Now()
			enabledUpdates[chunk.ID] = *update.IsEnabled
			chunksToUpdate = append(chunksToUpdate, chunk)
		}
		if update.IsRecommended != nil {
			current := chunk.Flags.HasFlag(types.ChunkFlagRecommended)
			if current != *update.IsRecommended {
				if *update.IsRecommended {
					setFlags[chunk.ID] = types.ChunkFlagRecommended
				} else {
					clearFlags[chunk.ID] = types.ChunkFlagRecommended
				}
			}
		}
	}

	if len(chunksToUpdate) > 0 {
		if err := s.chunkRepo.UpdateChunks(ctx, chunksToUpdate); err != nil {
			return err
		}
	}
	if len(setFlags) > 0 || len(clearFlags) > 0 {
		if err := s.chunkRepo.UpdateChunkFlagsBatch(ctx, tenantID, kb.ID, setFlags, clearFlags); err != nil {
			return err
		}
	}
	if len(enabledUpdates) == 0 {
		return nil
	}
	retrieveEngine, err := retriever.CreateRetrieveEngineForKB(
		ctx, s.retrieveEngine, s.ownership, tenantID, kb.VectorStoreID)
	if err != nil {
		return err
	}
	return retrieveEngine.BatchUpdateChunkEnabledStatus(ctx, enabledUpdates)
}

// SearchFAQEntries searches enabled FAQ entries using the knowledge base retriever.
func (s *knowledgeService) SearchFAQEntries(
	ctx context.Context,
	kbID string,
	req *types.FAQSearchRequest,
) ([]*types.FAQEntry, error) {
	if req == nil {
		return nil, werrors.NewBadRequestError("请求体不能为空")
	}
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}
	query := strings.TrimSpace(req.QueryText)
	if query == "" {
		return nil, werrors.NewBadRequestError("查询内容不能为空")
	}
	if req.VectorThreshold <= 0 {
		req.VectorThreshold = 0.7
	}
	if req.MatchCount <= 0 {
		req.MatchCount = 10
	}
	if req.MatchCount > 50 {
		req.MatchCount = 50
	}

	searchResults, err := s.kbService.HybridSearch(ctx, kbID, types.SearchParams{
		QueryText:            query,
		VectorThreshold:      req.VectorThreshold,
		MatchCount:           req.MatchCount,
		DisableKeywordsMatch: true,
		OnlyRecommended:      req.OnlyRecommended,
	})
	if err != nil {
		return nil, err
	}
	if len(searchResults) == 0 {
		return []*types.FAQEntry{}, nil
	}

	tenantID := types.MustTenantIDFromContext(ctx)
	chunkIDs := make([]string, 0, len(searchResults))
	for _, result := range searchResults {
		chunkIDs = append(chunkIDs, result.ID)
	}
	chunks, err := s.chunkRepo.ListChunksByID(ctx, tenantID, chunkIDs)
	if err != nil {
		return nil, err
	}
	chunkByID := make(map[string]*types.Chunk, len(chunks))
	for _, chunk := range chunks {
		chunkByID[chunk.ID] = chunk
	}

	kb.EnsureDefaults()
	entries := make([]*types.FAQEntry, 0, len(searchResults))
	for _, result := range searchResults {
		chunk := chunkByID[result.ID]
		if chunk == nil || chunk.ChunkType != types.ChunkTypeFAQ || !chunk.IsEnabled {
			continue
		}
		entry, err := s.chunkToFAQEntry(chunk, kb)
		if err != nil {
			logger.Warnf(ctx, "Failed to convert chunk to FAQ entry: %v", err)
			continue
		}
		entry.Score = result.Score
		entry.MatchType = result.MatchType
		entry.MatchedQuestion = result.MatchedContent
		entries = append(entries, entry)
		if len(entries) == req.MatchCount {
			break
		}
	}
	return entries, nil
}

// DeleteFAQEntries deletes FAQ entries in batch by seq_id.
func (s *knowledgeService) DeleteFAQEntries(ctx context.Context,
	kbID string, entrySeqIDs []int64,
) error {
	if len(entrySeqIDs) == 0 {
		return werrors.NewBadRequestError("请选择需要删除的 FAQ 条目")
	}
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return err
	}

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	var faqKnowledge *types.Knowledge
	chunksToRemove := make([]*types.Chunk, 0, len(entrySeqIDs))
	for _, seqID := range entrySeqIDs {
		if seqID <= 0 {
			continue
		}
		chunk, err := s.chunkRepo.GetChunkBySeqID(ctx, tenantID, seqID)
		if err != nil {
			return werrors.NewNotFoundError("FAQ条目不存在")
		}
		if chunk.KnowledgeBaseID != kb.ID || chunk.ChunkType != types.ChunkTypeFAQ {
			return werrors.NewBadRequestError("包含无效的 FAQ 条目")
		}
		if err := s.chunkService.DeleteChunk(ctx, chunk.ID); err != nil {
			return err
		}
		if faqKnowledge == nil {
			faqKnowledge, err = s.repo.GetKnowledgeByID(ctx, tenantID, chunk.KnowledgeID)
			if err != nil {
				return err
			}
		}
		chunksToRemove = append(chunksToRemove, chunk)
	}
	if len(chunksToRemove) > 0 && faqKnowledge != nil {
		if err := s.deleteFAQChunkVectors(ctx, kb, faqKnowledge, chunksToRemove); err != nil {
			return err
		}
	}
	return nil
}

// ExportFAQEntries exports all FAQ entries for a knowledge base as CSV data.
// The CSV format matches the import example format with 7 columns:
// 问题(必填), 相似问题(选填-多个用##分隔), 反例问题(选填-多个用##分隔),
// 机器人回答(必填-多个用##分隔), 是否全部回复(选填-默认FALSE), 是否停用(选填-默认FALSE),
// 是否禁止被推荐(选填-默认False 可被推荐)
func (s *knowledgeService) ExportFAQEntries(ctx context.Context, kbID string) ([]byte, error) {
	kb, err := s.validateFAQKnowledgeBase(ctx, kbID)
	if err != nil {
		return nil, err
	}

	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	faqKnowledge, err := s.findFAQKnowledge(ctx, tenantID, kb.ID)
	if err != nil {
		return nil, err
	}
	if faqKnowledge == nil {
		// Return empty CSV with headers only
		return s.buildFAQCSV(nil), nil
	}

	// Get all FAQ chunks
	chunks, err := s.chunkRepo.ListAllFAQChunksForExport(ctx, tenantID, faqKnowledge.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list FAQ chunks: %w", err)
	}

	return s.buildFAQCSV(chunks), nil
}

// buildFAQCSV builds CSV content from FAQ chunks.
func (s *knowledgeService) buildFAQCSV(chunks []*types.Chunk) []byte {
	var buf strings.Builder

	// Write CSV header (matching import example format)
	headers := []string{
		"问题(必填)",
		"相似问题(选填-多个用##分隔)",
		"反例问题(选填-多个用##分隔)",
		"机器人回答(必填-多个用##分隔)",
		"是否全部回复(选填-默认FALSE)",
		"是否停用(选填-默认FALSE)",
		"是否禁止被推荐(选填-默认False 可被推荐)",
	}
	buf.WriteString(strings.Join(headers, ","))
	buf.WriteString("\n")

	// Write data rows
	for _, chunk := range chunks {
		meta, err := chunk.FAQMetadata()
		if err != nil || meta == nil {
			continue
		}

		// Build row
		row := []string{
			escapeCSVField(meta.StandardQuestion),
			escapeCSVField(strings.Join(meta.SimilarQuestions, "##")),
			escapeCSVField(strings.Join(meta.NegativeQuestions, "##")),
			escapeCSVField(strings.Join(meta.Answers, "##")),
			boolToCSV(meta.AnswerStrategy == types.AnswerStrategyAll),
			boolToCSV(!chunk.IsEnabled),                                 // 是否停用：取反
			boolToCSV(!chunk.Flags.HasFlag(types.ChunkFlagRecommended)), // 是否禁止被推荐：取反
		}
		buf.WriteString(strings.Join(row, ","))
		buf.WriteString("\n")
	}

	return []byte(buf.String())
}

// escapeCSVField escapes a field for CSV format.
func escapeCSVField(field string) string {
	// If field contains comma, newline, or quote, wrap in quotes and escape internal quotes
	if strings.ContainsAny(field, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(field, "\"", "\"\"") + "\""
	}
	return field
}

// boolToCSV converts a boolean to CSV TRUE/FALSE string.
func boolToCSV(b bool) string {
	if b {
		return "TRUE"
	}
	return "FALSE"
}

func (s *knowledgeService) validateFAQKnowledgeBase(ctx context.Context, kbID string) (*types.KnowledgeBase, error) {
	if kbID == "" {
		return nil, werrors.NewBadRequestError("知识库 ID 不能为空")
	}
	kb, err := s.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		return nil, err
	}
	kb.EnsureDefaults()
	tenantID := types.MustTenantIDFromContext(ctx)
	if kb.TenantID != tenantID {
		return nil, werrors.NewForbiddenError("无权访问该知识库")
	}
	if kb.Type != types.KnowledgeBaseTypeFAQ {
		return nil, werrors.NewBadRequestError("仅 FAQ 知识库支持该操作")
	}
	return kb, nil
}

func (s *knowledgeService) findFAQKnowledge(
	ctx context.Context,
	tenantID uint64,
	kbID string,
) (*types.Knowledge, error) {
	knowledges, err := s.repo.ListKnowledgeByKnowledgeBaseID(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	for _, knowledge := range knowledges {
		if knowledge.Type == types.KnowledgeTypeFAQ {
			return knowledge, nil
		}
	}
	return nil, nil
}

func (s *knowledgeService) ensureFAQKnowledge(
	ctx context.Context,
	tenantID uint64,
	kb *types.KnowledgeBase,
) (*types.Knowledge, error) {
	existing, err := s.findFAQKnowledge(ctx, tenantID, kb.ID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}
	knowledge := &types.Knowledge{
		TenantID:         tenantID,
		KnowledgeBaseID:  kb.ID,
		Type:             types.KnowledgeTypeFAQ,
		Title:            buildFAQKnowledgeTitle(kb.Name),
		Description:      "FAQ 条目容器",
		Source:           types.KnowledgeTypeFAQ,
		ParseStatus:      "completed",
		EnableStatus:     "enabled",
		EmbeddingModelID: kb.EmbeddingModelID,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	if err := s.repo.CreateKnowledge(ctx, knowledge); err != nil {
		return nil, err
	}
	return knowledge, nil
}

// buildFAQKnowledgeTitle derives the display title for the FAQ container
// knowledge. The title is simply the knowledge base name (falling back to "FAQ"
// when empty) — no suffix is appended, so a KB named "FAQ" stays "FAQ" instead
// of becoming the redundant "FAQ - FAQ".
func buildFAQKnowledgeTitle(kbName string) string {
	name := strings.TrimSpace(kbName)
	if name == "" {
		return "FAQ"
	}
	return name
}

func (s *knowledgeService) chunkToFAQEntry(chunk *types.Chunk, kb *types.KnowledgeBase) (*types.FAQEntry, error) {
	meta, err := chunk.FAQMetadata()
	if err != nil {
		return nil, err
	}
	if meta == nil {
		meta = &types.FAQChunkMetadata{StandardQuestion: chunk.Content}
	}
	// 默认使用 all 策略
	answerStrategy := meta.AnswerStrategy
	if answerStrategy == "" {
		answerStrategy = types.AnswerStrategyAll
	}

	entry := &types.FAQEntry{
		ID:                chunk.SeqID,
		ChunkID:           chunk.ID,
		KnowledgeID:       chunk.KnowledgeID,
		KnowledgeBaseID:   chunk.KnowledgeBaseID,
		IsEnabled:         chunk.IsEnabled,
		IsRecommended:     chunk.Flags.HasFlag(types.ChunkFlagRecommended),
		StandardQuestion:  meta.StandardQuestion,
		SimilarQuestions:  meta.SimilarQuestions,
		NegativeQuestions: meta.NegativeQuestions,
		Answers:           meta.Answers,
		AnswerStrategy:    answerStrategy,
		IndexMode:         kb.FAQConfig.IndexMode,
		UpdatedAt:         chunk.UpdatedAt,
		CreatedAt:         chunk.CreatedAt,
		ChunkType:         chunk.ChunkType,
	}
	return entry, nil
}

func buildFAQChunkContent(meta *types.FAQChunkMetadata, mode types.FAQIndexMode) string {
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("Q: %s\n", meta.StandardQuestion))
	if len(meta.SimilarQuestions) > 0 {
		builder.WriteString("Similar Questions:\n")
		for _, q := range meta.SimilarQuestions {
			builder.WriteString(fmt.Sprintf("- %s\n", q))
		}
	}
	// 负例不应该包含在 Content 中，因为它们不应该被索引
	// 答案根据索引模式决定是否包含
	if mode == types.FAQIndexModeQuestionAnswer && len(meta.Answers) > 0 {
		builder.WriteString("Answers:\n")
		for _, ans := range meta.Answers {
			builder.WriteString(fmt.Sprintf("- %s\n", ans))
		}
	}
	return builder.String()
}

// checkFAQQuestionDuplicate 检查标准问和相似问是否与知识库中其他条目重复
// excludeChunkID 用于排除当前正在编辑的条目（更新时使用）
// 按照批量导入时的检查方式：先构建已存在问题集合，再统一检查
func (s *knowledgeService) checkFAQQuestionDuplicate(
	ctx context.Context,
	tenantID uint64,
	kbID string,
	excludeChunkID string,
	meta *types.FAQChunkMetadata,
) error {
	// 1. 首先检查当前条目自身的相似问是否与标准问重复
	for _, q := range meta.SimilarQuestions {
		if q == meta.StandardQuestion {
			return werrors.NewBadRequestError(fmt.Sprintf("相似问「%s」不能与标准问相同", q))
		}
	}

	// 2. 检查当前条目自身的相似问之间是否有重复
	seen := make(map[string]struct{})
	for _, q := range meta.SimilarQuestions {
		if _, exists := seen[q]; exists {
			return werrors.NewBadRequestError(fmt.Sprintf("相似问「%s」重复", q))
		}
		seen[q] = struct{}{}
	}

	// 3. 检查反例问题是否与标准问或相似问重复（反例不能和正例相同）
	positiveQuestions := make(map[string]struct{})
	positiveQuestions[meta.StandardQuestion] = struct{}{}
	for _, q := range meta.SimilarQuestions {
		positiveQuestions[q] = struct{}{}
	}
	negativeQuestionsSeen := make(map[string]struct{})
	for _, q := range meta.NegativeQuestions {
		if q == "" {
			continue
		}
		// 检查反例是否与标准问重复
		if q == meta.StandardQuestion {
			return werrors.NewBadRequestError(fmt.Sprintf("反例问题「%s」不能与标准问相同", q))
		}
		// 检查反例是否与相似问重复
		if _, exists := positiveQuestions[q]; exists {
			return werrors.NewBadRequestError(fmt.Sprintf("反例问题「%s」不能与相似问相同", q))
		}
		// 检查反例之间是否重复
		if _, exists := negativeQuestionsSeen[q]; exists {
			return werrors.NewBadRequestError(fmt.Sprintf("反例问题「%s」重复", q))
		}
		negativeQuestionsSeen[q] = struct{}{}
	}

	// 4. 将标准问和所有相似问合并，用一条 DB 查询检查是否与其他条目冲突（替代全量扫描）
	allQuestions := make([]string, 0, 1+len(meta.SimilarQuestions))
	allQuestions = append(allQuestions, meta.StandardQuestion)
	allQuestions = append(allQuestions, meta.SimilarQuestions...)

	dupChunk, err := s.chunkRepo.FindFAQChunkWithDuplicateQuestion(ctx, tenantID, kbID, excludeChunkID, allQuestions)
	if err != nil {
		return fmt.Errorf("failed to check FAQ question duplicate: %w", err)
	}
	if dupChunk == nil {
		return nil
	}

	existingMeta, err := dupChunk.FAQMetadata()
	if err != nil || existingMeta == nil {
		return werrors.NewBadRequestError("标准问或相似问与已有条目重复")
	}

	// 5–7. 与原先全量扫描一致的报错语义：先检查标准问，再逐条检查相似问
	existingSimilarSet := make(map[string]struct{}, len(existingMeta.SimilarQuestions))
	for _, q := range existingMeta.SimilarQuestions {
		if q != "" {
			existingSimilarSet[q] = struct{}{}
		}
	}

	if meta.StandardQuestion != "" {
		if meta.StandardQuestion == existingMeta.StandardQuestion {
			return werrors.NewBadRequestError(fmt.Sprintf("标准问「%s」已存在", meta.StandardQuestion))
		}
		if _, ok := existingSimilarSet[meta.StandardQuestion]; ok {
			return werrors.NewBadRequestError(fmt.Sprintf("标准问「%s」已存在", meta.StandardQuestion))
		}
	}

	for _, q := range meta.SimilarQuestions {
		if q == "" {
			continue
		}
		if q == existingMeta.StandardQuestion {
			return werrors.NewBadRequestError(fmt.Sprintf("相似问「%s」已存在", q))
		}
		if _, ok := existingSimilarSet[q]; ok {
			return werrors.NewBadRequestError(fmt.Sprintf("相似问「%s」已存在", q))
		}
	}

	return werrors.NewBadRequestError("标准问或相似问与已有条目重复")
}

func sanitizeFAQEntryPayload(payload *types.FAQEntryPayload) (*types.FAQChunkMetadata, error) {
	// 处理 AnswerStrategy，默认为 all
	answerStrategy := types.AnswerStrategyAll
	if payload.AnswerStrategy != nil && *payload.AnswerStrategy != "" {
		switch *payload.AnswerStrategy {
		case types.AnswerStrategyAll, types.AnswerStrategyRandom:
			answerStrategy = *payload.AnswerStrategy
		default:
			return nil, werrors.NewBadRequestError("answer_strategy 必须是 'all' 或 'random'")
		}
	}
	meta := &types.FAQChunkMetadata{
		StandardQuestion:  strings.TrimSpace(payload.StandardQuestion),
		SimilarQuestions:  payload.SimilarQuestions,
		NegativeQuestions: payload.NegativeQuestions,
		Answers:           payload.Answers,
		AnswerStrategy:    answerStrategy,
		Version:           1,
		Source:            "faq",
	}
	meta.Normalize()
	if meta.StandardQuestion == "" {
		return nil, werrors.NewBadRequestError("标准问不能为空")
	}
	if len(meta.Answers) == 0 {
		return nil, werrors.NewBadRequestError("至少提供一个答案")
	}
	return meta, nil
}

func buildFAQIndexContent(meta *types.FAQChunkMetadata, mode types.FAQIndexMode) string {
	var builder strings.Builder
	builder.WriteString(meta.StandardQuestion)
	for _, q := range meta.SimilarQuestions {
		builder.WriteString("\n")
		builder.WriteString(q)
	}
	if mode == types.FAQIndexModeQuestionAnswer {
		for _, ans := range meta.Answers {
			builder.WriteString("\n")
			builder.WriteString(ans)
		}
	}
	return builder.String()
}

// buildFAQIndexInfoList 构建FAQ索引信息列表，支持分别索引模式
func (s *knowledgeService) buildFAQIndexInfoList(
	ctx context.Context,
	kb *types.KnowledgeBase,
	chunk *types.Chunk,
) ([]*types.IndexInfo, error) {
	indexMode := types.FAQIndexModeQuestionAnswer
	questionIndexMode := types.FAQQuestionIndexModeCombined
	if kb.FAQConfig != nil {
		if kb.FAQConfig.IndexMode != "" {
			indexMode = kb.FAQConfig.IndexMode
		}
		if kb.FAQConfig.QuestionIndexMode != "" {
			questionIndexMode = kb.FAQConfig.QuestionIndexMode
		}
	}

	meta, err := chunk.FAQMetadata()
	if err != nil {
		return nil, err
	}
	if meta == nil {
		meta = &types.FAQChunkMetadata{StandardQuestion: chunk.Content}
	}

	// 如果是一起索引模式，使用原有逻辑
	if questionIndexMode == types.FAQQuestionIndexModeCombined {
		content := buildFAQIndexContent(meta, indexMode)
		return []*types.IndexInfo{
			{
				Content:         content,
				SourceID:        chunk.ID,
				SourceType:      types.ChunkSourceType,
				ChunkID:         chunk.ID,
				KnowledgeID:     chunk.KnowledgeID,
				KnowledgeBaseID: chunk.KnowledgeBaseID,
				KnowledgeType:   types.KnowledgeTypeFAQ,
				IsEnabled:       chunk.IsEnabled,
				IsRecommended:   chunk.Flags.HasFlag(types.ChunkFlagRecommended),
			},
		}, nil
	}

	// 分别索引模式：为每个问题创建独立的索引项
	indexInfoList := make([]*types.IndexInfo, 0)

	// 标准问索引项
	standardContent := meta.StandardQuestion
	if indexMode == types.FAQIndexModeQuestionAnswer && len(meta.Answers) > 0 {
		var builder strings.Builder
		builder.WriteString(meta.StandardQuestion)
		for _, ans := range meta.Answers {
			builder.WriteString("\n")
			builder.WriteString(ans)
		}
		standardContent = builder.String()
	}
	indexInfoList = append(indexInfoList, &types.IndexInfo{
		Content:         standardContent,
		SourceID:        chunk.ID,
		SourceType:      types.ChunkSourceType,
		ChunkID:         chunk.ID,
		KnowledgeID:     chunk.KnowledgeID,
		KnowledgeBaseID: chunk.KnowledgeBaseID,
		KnowledgeType:   types.KnowledgeTypeFAQ,
		IsEnabled:       chunk.IsEnabled,
		IsRecommended:   chunk.Flags.HasFlag(types.ChunkFlagRecommended),
	})

	// 每个相似问创建一个索引项
	for i, similarQ := range meta.SimilarQuestions {
		similarContent := similarQ
		if indexMode == types.FAQIndexModeQuestionAnswer && len(meta.Answers) > 0 {
			var builder strings.Builder
			builder.WriteString(similarQ)
			for _, ans := range meta.Answers {
				builder.WriteString("\n")
				builder.WriteString(ans)
			}
			similarContent = builder.String()
		}
		sourceID := fmt.Sprintf("%s-%d", chunk.ID, i)
		indexInfoList = append(indexInfoList, &types.IndexInfo{
			Content:         similarContent,
			SourceID:        sourceID,
			SourceType:      types.ChunkSourceType,
			ChunkID:         chunk.ID,
			KnowledgeID:     chunk.KnowledgeID,
			KnowledgeBaseID: chunk.KnowledgeBaseID,
			KnowledgeType:   types.KnowledgeTypeFAQ,
			IsEnabled:       chunk.IsEnabled,
			IsRecommended:   chunk.Flags.HasFlag(types.ChunkFlagRecommended),
		})
	}

	return indexInfoList, nil
}
