package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	chatpipeline "github.com/xyz2781790037/ZealRAG/internal/application/service/chat_pipeline"
	"github.com/xyz2781790037/ZealRAG/internal/common"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/event"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/models/chat"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// KnowledgeQA performs knowledge base question answering with LLM summarization
// Events are emitted through eventBus (references, answer chunks, completion)
// answerMode is optional - if provided, uses answer mode configuration for multiTurnEnabled and historyTurns
func (s *sessionService) KnowledgeQA(
	ctx context.Context,
	req *types.QARequest,
	eventBus *event.EventBus,
) error {
	logger.Infof(
		ctx,
		"Knowledge base question answering parameters, session ID: %s, query: %s, webSearchEnabled: %v",
		req.Session.ID,
		req.Query,
		req.WebSearchEnabled,
	)

	// Span the request setup (KB / model resolution, search target building,
	// agent override application). This covers the visible gap between trace
	// start and the first stage observation in the Langfuse timeline.
	setupCtx, setupSpan := langfuse.GetManager().StartSpan(ctx, langfuse.SpanOptions{
		Name: "qa.setup",
		Metadata: map[string]interface{}{
			"session_id": req.Session.ID,
		},
	})
	ctx = setupCtx

	// Resolve knowledge bases using shared helper
	knowledgeBaseIDs, knowledgeIDs := s.resolveKnowledgeBases(ctx, req)

	// Initialize ChatManage defaults from config.yaml
	summaryConfig := types.SummaryConfig{
		Prompt:              s.cfg.Conversation.Summary.Prompt,
		ContextTemplate:     s.cfg.Conversation.Summary.ContextTemplate,
		Temperature:         s.cfg.Conversation.Summary.Temperature,
		NoMatchPrefix:       s.cfg.Conversation.Summary.NoMatchPrefix,
		MaxCompletionTokens: s.cfg.Conversation.Summary.MaxCompletionTokens,
		Thinking:            s.cfg.Conversation.Summary.Thinking,
	}
	fallbackStrategy := types.FallbackStrategy(s.cfg.Conversation.FallbackStrategy)
	if fallbackStrategy == "" {
		fallbackStrategy = types.FallbackStrategyFixed
		logger.Infof(ctx, "Fallback strategy not set, using default: %v", fallbackStrategy)
	}

	// Resolve chat model vision capability and VLM model ID for image routing
	// Build unified search targets first: they carry the immutable storage
	// tenant for each owned/shared KB after ACL validation.
	retrievalTenantID := s.resolveRetrievalTenantID(ctx, req)
	searchTargets, err := s.buildSearchTargets(ctx, retrievalTenantID, knowledgeBaseIDs, knowledgeIDs)
	if err != nil {
		return err
	}
	modelTenantID := retrievalTenantID
	if len(searchTargets) > 0 && searchTargets[0].TenantID != 0 {
		modelTenantID = searchTargets[0].TenantID
	}
	modelCtx := types.WithTenantID(ctx, modelTenantID)

	// The knowledge-base owner pays for and controls the answer model.
	chatModelID, err := s.resolveChatModelID(modelCtx, req, knowledgeBaseIDs, knowledgeIDs)
	if err != nil {
		return err
	}

	var chatModelSupportsVision bool
	var vlmModelID string
	if chatModelID != "" {
		if chatModelInfo, err := s.modelService.GetModelByID(modelCtx, chatModelID); err == nil && chatModelInfo != nil {
			chatModelSupportsVision = chatModelInfo.Parameters.SupportsVision
		}
	}
	if vlmModel, err := s.modelService.GetDefaultModel(modelCtx, types.ModelTypeVLLM); err == nil {
		vlmModelID = vlmModel.ID
	}

	// Create chat management object with session settings
	logger.Infof(
		ctx,
		"Creating chat manage object, knowledge base IDs: %v, knowledge IDs: %v, chat model ID: %s, search targets: %d",
		knowledgeBaseIDs,
		knowledgeIDs,
		chatModelID,
		len(searchTargets),
	)

	// Get UserID from context
	userID, _ := types.UserIDFromContext(ctx)

	chatManage := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			Query:                   req.Query,
			SessionID:               req.Session.ID,
			UserID:                  userID,
			MaxRounds:               s.cfg.Conversation.MaxRounds,
			KnowledgeBaseIDs:        knowledgeBaseIDs,
			KnowledgeIDs:            knowledgeIDs,
			SearchTargets:           searchTargets,
			VectorThreshold:         s.cfg.Conversation.VectorThreshold,
			KeywordThreshold:        s.cfg.Conversation.KeywordThreshold,
			EmbeddingTopK:           s.cfg.Conversation.EmbeddingTopK,
			RerankTopK:              s.cfg.Conversation.RerankTopK,
			RerankThreshold:         s.cfg.Conversation.RerankThreshold,
			ChatModelID:             chatModelID,
			ModelTenantID:           modelTenantID,
			SummaryConfig:           summaryConfig,
			FallbackStrategy:        fallbackStrategy,
			FallbackResponse:        s.cfg.Conversation.FallbackResponse,
			FallbackPrompt:          s.cfg.Conversation.FallbackPrompt,
			EnableRewrite:           s.cfg.Conversation.EnableRewrite,
			EnableQueryExpansion:    s.cfg.Conversation.EnableQueryExpansion,
			RewritePromptSystem:     s.cfg.Conversation.RewritePromptSystem,
			RewritePromptUser:       s.cfg.Conversation.RewritePromptUser,
			WebSearchEnabled:        req.WebSearchEnabled,
			WebSearchProviderID:     s.resolveWebSearchProviderID(ctx, req, retrievalTenantID),
			WebSearchMaxResults:     s.resolveWebSearchMaxResults(ctx, req),
			WebFetchEnabled:         s.resolveWebFetchEnabled(req),
			WebFetchTopN:            s.resolveWebFetchTopN(req),
			TenantID:                modelTenantID,
			Images:                  req.ImageURLs,
			VLMModelID:              vlmModelID,
			ChatModelSupportsVision: chatModelSupportsVision,
			Attachments:             req.Attachments,
			Language:                types.LanguageNameFromContext(ctx),
		},
		PipelineState: types.PipelineState{
			RewriteQuery:     req.Query,
			ImageDescription: req.ImageDescription,
			QuotedContext:    req.QuotedContext,
		},
		PipelineContext: types.PipelineContext{
			EventBus:      eventBus.AsEventBusInterface(),
			MessageID:     req.AssistantMessageID,
			UserMessageID: req.UserMessageID,
		},
	}

	// Apply answer mode overrides (system prompt, temperature, retrieval params,
	// rewrite, fallback, FAQ strategy, history turns)
	s.applyAnswerModeOverridesToChatManage(ctx, req.AnswerMode, chatManage)
	// Reranking is also charged to the KB owner. Prefer their configured model,
	// otherwise use the first active reranker visible in the owner tenant.
	var ownerRetrievalConfig *types.RetrievalConfig
	if tenant, tenantErr := s.tenantService.GetTenantByID(modelCtx, modelTenantID); tenantErr == nil && tenant != nil {
		ownerRetrievalConfig = tenant.RetrievalConfig
	}
	if ownerRetrievalConfig != nil && ownerRetrievalConfig.RerankModelID != "" {
		chatManage.RerankModelID = ownerRetrievalConfig.RerankModelID
	} else if models, modelErr := s.modelService.ListModels(modelCtx); modelErr == nil {
		chatManage.RerankModelID = ""
		for _, model := range models {
			if model != nil && model.Type == types.ModelTypeRerank {
				chatManage.RerankModelID = model.ID
				break
			}
		}
	}

	// Determine pipeline based on knowledge bases availability and web search setting
	hasKB := len(knowledgeBaseIDs) > 0 || len(knowledgeIDs) > 0
	needsRAG := hasKB || req.WebSearchEnabled
	hasHistory := chatManage.MaxRounds > 0

	var pipeline []types.EventType
	if !needsRAG {
		// Pure chat — no retrieval needed.
		userContent := req.Query
		if req.ImageDescription != "" && !chatModelSupportsVision {
			userContent += "\n\n[用户上传图片内容]\n" + req.ImageDescription
		}
		if req.QuotedContext != "" {
			userContent += "\n\n" + req.QuotedContext
		}
		// Inject attachment content for pure-chat path (RAG path handles this in INTO_CHAT_MESSAGE).
		if len(req.Attachments) > 0 {
			userContent += req.Attachments.BuildPrompt()
		}
		chatManage.UserContent = userContent

		pipeline = types.NewPipelineBuilder().
			AddIf(hasHistory, types.LOAD_HISTORY).
			Add(types.CHAT_COMPLETION_STREAM).
			Build()
	} else {
		// RAG — dynamically assemble based on feature flags.
		pipeline = types.NewPipelineBuilder().
			AddIf(hasHistory, types.LOAD_HISTORY).
			Add(types.QUERY_UNDERSTAND).
			Add(types.CHUNK_SEARCH).
			Add(types.CHUNK_RERANK).
			AddIf(req.WebSearchEnabled, types.WEB_FETCH).
			Add(types.CHUNK_MERGE).
			Add(types.FILTER_TOP_K).
			Add(types.INTO_CHAT_MESSAGE).
			Add(types.CHAT_COMPLETION_STREAM).
			Build()
	}

	logger.Infof(ctx, "Assembled pipeline (%d stages), hasKB=%v, webSearch=%v, history=%v",
		len(pipeline), hasKB, req.WebSearchEnabled, hasHistory)

	// Start knowledge QA event processing.
	logger.Info(ctx, "Triggering question answering event")
	setupSpan.Finish(map[string]interface{}{
		"stages":             len(pipeline),
		"knowledge_base_ids": knowledgeBaseIDs,
		"search_targets":     len(searchTargets),
	}, nil, nil)
	err = s.KnowledgeQAByEvent(ctx, chatManage, pipeline)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"session_id": req.Session.ID,
		})
		return err
	}

	// Note: Answer events are now emitted directly by chat_completion_stream plugin
	// Completion event will be emitted when the last answer event has Done=true
	// We can optionally add a completion watcher here if needed, but for now
	// the frontend can detect completion from the Done flag

	logger.Info(ctx, "Knowledge base question answering initiated")
	return nil
}

// selectChatModelID returns the single workspace-wide chat model.
func (s *sessionService) selectChatModelID(
	ctx context.Context,
	_ *types.Session,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
) (string, error) {
	if len(knowledgeBaseIDs) > 0 {
		kbs, err := s.knowledgeBaseService.GetKnowledgeBasesByIDsOnly(ctx, knowledgeBaseIDs[:1])
		if err == nil && len(kbs) > 0 && kbs[0] != nil && kbs[0].SummaryModelID != "" {
			return kbs[0].SummaryModelID, nil
		}
	}
	if len(knowledgeIDs) > 0 {
		if kb, _, _, err := s.shareService.ResolveKnowledgeAccess(ctx, knowledgeIDs[0], types.KBRoleReader); err == nil && kb.SummaryModelID != "" {
			return kb.SummaryModelID, nil
		}
	}
	model, err := s.modelService.GetDefaultModel(ctx, types.ModelTypeKnowledgeQA)
	if err != nil {
		return "", err
	}
	return model.ID, nil
}

// resolveKnowledgeBasesFromMode resolves the quick mode's configured
// knowledge-base scope when the user did not make an explicit selection.
func (s *sessionService) resolveKnowledgeBasesFromMode(
	ctx context.Context,
	answerMode *types.AnswerMode,
) []string {
	if answerMode == nil {
		return nil
	}

	switch answerMode.Config.KBSelectionMode {
	case "all":
		allKBs, err := s.knowledgeBaseService.ListKnowledgeBases(ctx)
		if err != nil {
			logger.Warnf(ctx, "Failed to list all knowledge bases: %v", err)
		}
		kbIDs := make([]string, 0, len(allKBs))
		for _, kb := range allKBs {
			if kb == nil || kb.IsTemporary {
				continue
			}
			kbIDs = append(kbIDs, kb.ID)
		}
		logger.Infof(ctx, "KBSelectionMode=all: loaded %d knowledge bases", len(kbIDs))
		return kbIDs
	case "selected":
		logger.Infof(ctx, "KBSelectionMode=selected: using %d configured knowledge bases", len(answerMode.Config.KnowledgeBases))
		return answerMode.Config.KnowledgeBases
	case "none":
		logger.Infof(ctx, "KBSelectionMode=none: no knowledge bases configured")
		return nil
	default:
		// Default to "selected" behavior for backward compatibility
		if len(answerMode.Config.KnowledgeBases) > 0 {
			logger.Infof(ctx, "KBSelectionMode not set: using %d configured knowledge bases", len(answerMode.Config.KnowledgeBases))
		}
		return answerMode.Config.KnowledgeBases
	}
}

// buildSearchTargets computes the unified search targets from knowledgeBaseIDs and knowledgeIDs.
// tenantID is the current retrieval scope.
// This is called once at the request entry point to avoid repeated queries later in the pipeline.
// Logic:
//   - For each knowledgeBaseID: include it only when owned by tenantID.
//   - For each knowledgeID: find its knowledgeBaseID; if the KB is already in the list, skip; otherwise add SearchTargetTypeKnowledge
func (s *sessionService) buildSearchTargets(
	ctx context.Context,
	tenantID uint64,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
) (types.SearchTargets, error) {
	var targets types.SearchTargets

	kbTenantMap := make(map[string]uint64)

	// Track which KBs are fully searched
	fullKBSet := make(map[string]bool)

	if len(knowledgeBaseIDs) > 0 {
		kbs, _ := s.knowledgeBaseService.GetKnowledgeBasesByIDsOnly(ctx, knowledgeBaseIDs)
		kbByID := make(map[string]*types.KnowledgeBase, len(kbs))
		for _, kb := range kbs {
			if kb != nil {
				kbByID[kb.ID] = kb
			}
		}
		for _, kbID := range knowledgeBaseIDs {
			kb := kbByID[kbID]
			if kb == nil {
				return nil, apperrors.NewNotFoundError("知识库不存在")
			}
			resolvedKB, _, accessErr := s.shareService.ResolveAccess(ctx, kbID, types.KBRoleReader)
			if accessErr != nil {
				return nil, accessErr
			}
			fullKBSet[kbID] = true
			kbTenantMap[kbID] = resolvedKB.TenantID
			targets = append(targets, &types.SearchTarget{
				Type:            types.SearchTargetTypeKnowledgeBase,
				KnowledgeBaseID: kbID,
				TenantID:        resolvedKB.TenantID,
			})
		}
	}

	// Process individual knowledge IDs in the current tenant.
	if len(knowledgeIDs) > 0 {
		knowledgeList := make([]*types.Knowledge, 0, len(knowledgeIDs))
		for _, knowledgeID := range knowledgeIDs {
			_, _, knowledge, accessErr := s.shareService.ResolveKnowledgeAccess(ctx, knowledgeID, types.KBRoleReader)
			if accessErr != nil {
				return nil, accessErr
			}
			knowledgeList = append(knowledgeList, knowledge)
		}

		// Group knowledge IDs by their KB, excluding those already covered by full KB search
		// Also track KB tenant IDs from knowledge items
		kbToKnowledgeIDs := make(map[string][]string)
		for _, k := range knowledgeList {
			if k == nil || k.KnowledgeBaseID == "" {
				continue
			}
			// Track KB -> TenantID mapping from knowledge items
			if kbTenantMap[k.KnowledgeBaseID] == 0 {
				kbTenantMap[k.KnowledgeBaseID] = k.TenantID
			}
			// Skip if this KB is already fully searched
			if fullKBSet[k.KnowledgeBaseID] {
				continue
			}
			kbToKnowledgeIDs[k.KnowledgeBaseID] = append(kbToKnowledgeIDs[k.KnowledgeBaseID], k.ID)
		}

		// Create SearchTargetTypeKnowledge targets for each KB with specific files
		for kbID, kidList := range kbToKnowledgeIDs {
			kbTenant := kbTenantMap[kbID]
			if kbTenant == 0 {
				kbTenant = tenantID // fallback
			}
			targets = append(targets, &types.SearchTarget{
				Type:            types.SearchTargetTypeKnowledge,
				KnowledgeBaseID: kbID,
				TenantID:        kbTenant,
				KnowledgeIDs:    kidList,
			})
		}
	}

	logger.Infof(ctx, "Built %d search targets: %d full KB, %d partial KB, kbTenantMap=%v",
		len(targets), len(knowledgeBaseIDs), len(targets)-len(knowledgeBaseIDs), kbTenantMap)

	return targets, nil
}

// KnowledgeQAByEvent processes knowledge QA through a series of events in the pipeline
func (s *sessionService) KnowledgeQAByEvent(ctx context.Context,
	chatManage *types.ChatManage, eventList []types.EventType,
) error {
	logger.Info(ctx, "Start processing knowledge base question answering through events")
	logger.Infof(ctx, "Knowledge base question answering parameters, session ID: %s, query: %s",
		chatManage.SessionID, chatManage.Query)

	methods := make([]string, len(eventList))
	for i, event := range eventList {
		methods[i] = string(event)
	}
	logger.Infof(ctx, "Trigger event list: %v", methods)

	pipelineStart := time.Now()
	lastRetrievalStage := chatpipeline.LastConsolidatedRetrievalStage(eventList, chatManage)
	var retrievalProgress *chatpipeline.StageProgress
	var retrievalStart time.Time
	var understandProgress *chatpipeline.StageProgress
	var understandStart time.Time
	for _, eventType := range eventList {
		stageStart := time.Now()
		// Wrap each pipeline stage in a Langfuse span so the trace timeline
		// shows the gaps between LLM/embedding/rerank generations (the work
		// that happens between them — vector DB search, merge, filter, prompt
		// assembly — was previously invisible). Generations created inside
		// the stage automatically nest under this span.
		//
		// CHAT_COMPLETION_STREAM is intentionally skipped: its OnEvent kicks
		// off a streaming goroutine and returns immediately, so a span would
		// finish well before the chat.completion.stream generation does. The
		// generation already captures the full stream duration; adding a
		// stage span here would just produce a child observation that
		// visually exceeds its parent.
		stageCtx := ctx
		var stageSpan *langfuse.Span
		if eventType != types.CHAT_COMPLETION_STREAM {
			stageCtx, stageSpan = langfuse.GetManager().StartSpan(ctx, langfuse.SpanOptions{
				Name: "pipeline." + string(eventType),
				Metadata: map[string]interface{}{
					"event_type": string(eventType),
					"session_id": chatManage.SessionID,
				},
			})
		}
		if eventType == types.QUERY_UNDERSTAND && chatpipeline.ShouldEmitQueryUnderstandProgress(chatManage) {
			understandStart = stageStart
			understandProgress = chatpipeline.BeginQueryUnderstandProgress(stageCtx, chatManage)
		}
		if chatpipeline.IsConsolidatedRetrievalStage(eventType, chatManage) && retrievalProgress == nil {
			retrievalStart = stageStart
			retrievalProgress = chatpipeline.BeginRetrievalProgress(stageCtx, chatManage)
		}
		// Emit references before answer streaming so the SSE client receives
		// them while the connection is still open. Previously references were
		// emitted after the pipeline returned — by then the `complete` event had
		// already closed the stream, so the frontend only saw citations on refresh.
		if eventType == types.CHAT_COMPLETION_STREAM {
			emitKnowledgeReferencesEvent(ctx, chatManage)
		}
		err := s.eventManager.Trigger(stageCtx, eventType, chatManage)
		if understandProgress != nil && eventType == types.QUERY_UNDERSTAND {
			chatpipeline.EndQueryUnderstandProgress(stageCtx, chatManage, understandProgress, understandStart, err)
			understandProgress = nil
		}
		if retrievalProgress != nil && eventType == lastRetrievalStage {
			chatpipeline.EndRetrievalProgress(stageCtx, chatManage, retrievalProgress, retrievalStart, err)
			retrievalProgress = nil
		}
		stageDuration := time.Since(stageStart)
		var spanErr error
		if err != nil && err != chatpipeline.ErrSearchNothing {
			spanErr = err.Err
		}
		if stageSpan != nil {
			stageSpan.Finish(map[string]interface{}{
				"duration_ms": stageDuration.Milliseconds(),
			}, nil, spanErr)
		}

		// If the user stopped generation, the context is cancelled. A cancelled
		// retrieval stage surfaces as ErrSearchNothing (the search goroutines
		// return no results when their embedding/vector calls are aborted), so
		// this check MUST come before the ErrSearchNothing handling below.
		// Otherwise we would persist the fixed fallback response ("Sorry, I am
		// unable to answer this question.") over the intentionally-empty stopped
		// message, and the user would see the fallback text after refreshing.
		// This is not single-machine specific: the stop arrives via the shared
		// StreamManager and cancels asyncCtx on whichever node is generating.
		if ctxErr := ctx.Err(); ctxErr != nil {
			common.PipelineWarn(ctx, "Pipeline", "stage_cancelled", map[string]interface{}{
				"event":       string(eventType),
				"duration_ms": stageDuration.Milliseconds(),
				"reason":      ctxErr.Error(),
			})
			return ctxErr
		}

		if err == chatpipeline.ErrSearchNothing {
			common.PipelineWarn(ctx, "Pipeline", "stage_fallback", map[string]interface{}{
				"event":       string(eventType),
				"duration_ms": stageDuration.Milliseconds(),
				"reason":      "search_nothing",
				"strategy":    string(chatManage.FallbackStrategy),
			})
			s.handleFallbackResponse(ctx, chatManage)
			return nil
		}

		if err != nil {
			common.PipelineError(ctx, "Pipeline", "stage_failed", map[string]interface{}{
				"event":       string(eventType),
				"duration_ms": stageDuration.Milliseconds(),
				"error_type":  err.ErrorType,
				"description": err.Description,
			})
			return err.Err
		}

		common.PipelineInfo(ctx, "Pipeline", "stage_complete", map[string]interface{}{
			"event":       string(eventType),
			"duration_ms": stageDuration.Milliseconds(),
		})
	}

	common.PipelineInfo(ctx, "Pipeline", "all_stages_complete", map[string]interface{}{
		"session_id":        chatManage.SessionID,
		"total_stages":      len(eventList),
		"total_duration_ms": time.Since(pipelineStart).Milliseconds(),
	})
	return nil
}

// SearchKnowledge performs knowledge base search without LLM summarization
// knowledgeBaseIDs: list of knowledge base IDs to search (supports multi-KB)
// knowledgeIDs: list of specific knowledge (file) IDs to search
func (s *sessionService) SearchKnowledge(ctx context.Context,
	knowledgeBaseIDs []string, knowledgeIDs []string, query string,
) ([]*types.SearchResult, error) {
	logger.Info(ctx, "Start knowledge base search without LLM summary")
	logger.Infof(ctx, "Knowledge base search parameters, knowledge base IDs: %v, knowledge IDs: %v, query: %s",
		knowledgeBaseIDs, knowledgeIDs, query)

	// Get tenant ID from context
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok {
		logger.Error(ctx, "Failed to get tenant ID from context")
		return nil, fmt.Errorf("tenant ID not found in context")
	}

	// Build unified search targets (computed once, used throughout pipeline)
	searchTargets, err := s.buildSearchTargets(ctx, tenantID, knowledgeBaseIDs, knowledgeIDs)
	if err != nil {
		return nil, err
	}

	if len(searchTargets) == 0 {
		logger.Warn(ctx, "No search targets available, returning empty results")
		return []*types.SearchResult{}, nil
	}
	resourceTenantID := tenantID
	if searchTargets[0].TenantID != 0 {
		resourceTenantID = searchTargets[0].TenantID
	}
	resourceCtx := types.WithTenantID(ctx, resourceTenantID)

	// Create default retrieval parameters — prefer tenant RetrievalConfig, fallback to built-in defaults
	userID, _ := types.UserIDFromContext(ctx)

	// Load tenant-level retrieval config (nil is safe — GetEffective* methods handle nil receiver)
	var rc *types.RetrievalConfig
	if tenant, err2 := s.tenantService.GetTenantByID(resourceCtx, resourceTenantID); err2 == nil {
		rc = tenant.RetrievalConfig
	}

	chatManage := &types.ChatManage{
		PipelineRequest: types.PipelineRequest{
			Query:            query,
			UserID:           userID,
			KnowledgeBaseIDs: knowledgeBaseIDs,
			KnowledgeIDs:     knowledgeIDs,
			SearchTargets:    searchTargets,
			TenantID:         resourceTenantID,
			ModelTenantID:    resourceTenantID,
			MaxRounds:        s.cfg.Conversation.MaxRounds,
			EmbeddingTopK:    rc.GetEffectiveEmbeddingTopK(),
			VectorThreshold:  rc.GetEffectiveVectorThreshold(),
			KeywordThreshold: rc.GetEffectiveKeywordThreshold(),
			RerankTopK:       rc.GetEffectiveRerankTopK(),
			RerankThreshold:  rc.GetEffectiveRerankThreshold(),
		},
		PipelineState: types.PipelineState{
			RewriteQuery: query,
		},
	}

	// Get default models
	models, err := s.modelService.ListModels(resourceCtx)
	if err != nil {
		logger.Errorf(ctx, "Failed to get models: %v", err)
		return nil, err
	}

	// Use rerank model from RetrievalConfig if set, otherwise auto-select the first available
	if rc != nil && rc.RerankModelID != "" {
		chatManage.RerankModelID = rc.RerankModelID
	} else {
		for _, model := range models {
			if model == nil {
				continue
			}
			if model.Type == types.ModelTypeRerank {
				chatManage.RerankModelID = model.ID
				break
			}
		}
	}

	// Use specific event list, only including retrieval-related events, not LLM summarization
	searchEvents := []types.EventType{
		types.CHUNK_SEARCH, // Vector search
		types.CHUNK_RERANK, // Rerank search results
		types.CHUNK_MERGE,  // Merge search results
		types.FILTER_TOP_K, // Filter top K results
	}

	logger.Infof(ctx, "Trigger search event list: %v", searchEvents)

	for _, event := range searchEvents {
		logger.Infof(ctx, "Starting to trigger search event: %v", event)
		stageCtx, stageSpan := langfuse.GetManager().StartSpan(ctx, langfuse.SpanOptions{
			Name: "pipeline." + string(event),
			Metadata: map[string]interface{}{
				"event_type": string(event),
				"flow":       "search_knowledge",
			},
		})
		err := s.eventManager.Trigger(stageCtx, event, chatManage)
		var spanErr error
		if err != nil && err != chatpipeline.ErrSearchNothing {
			spanErr = err.Err
		}
		stageSpan.Finish(nil, nil, spanErr)

		if err == chatpipeline.ErrSearchNothing {
			logger.Warnf(ctx, "Event %v triggered, search result is empty", event)
			return []*types.SearchResult{}, nil
		}

		if err != nil {
			logger.Errorf(ctx, "Event triggering failed, event: %v, error type: %s, description: %s, error: %v",
				event, err.ErrorType, err.Description, err.Err)
			return nil, err.Err
		}
		logger.Infof(ctx, "Event %v triggered successfully", event)
	}

	logger.Infof(ctx, "Knowledge base search completed, found %d results", len(chatManage.MergeResult))
	return chatManage.MergeResult, nil
}

// handleFallbackResponse handles fallback response based on strategy
func (s *sessionService) handleFallbackResponse(ctx context.Context, chatManage *types.ChatManage) {
	if chatManage.FallbackStrategy == types.FallbackStrategyModel {
		s.handleModelFallback(ctx, chatManage)
	} else {
		s.handleFixedFallback(ctx, chatManage)
	}
}

// handleFixedFallback handles fixed fallback response
func (s *sessionService) handleFixedFallback(ctx context.Context, chatManage *types.ChatManage) {
	fallbackContent := chatManage.FallbackResponse
	chatManage.ChatResponse = &types.ChatResponse{Content: fallbackContent}
	s.emitFallbackAnswer(ctx, chatManage, fallbackContent)
}

// handleModelFallback handles model-based fallback response using streaming
func (s *sessionService) handleModelFallback(ctx context.Context, chatManage *types.ChatManage) {
	// Check if FallbackPrompt is available
	if chatManage.FallbackPrompt == "" {
		logger.Warnf(ctx, "Fallback strategy is 'model' but FallbackPrompt is empty, falling back to fixed response")
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	// Render template with Query variable
	promptContent, err := s.renderFallbackPrompt(ctx, chatManage)
	if err != nil {
		logger.Errorf(ctx, "Failed to render fallback prompt: %v, falling back to fixed response", err)
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	// Check if EventBus is available for streaming
	if chatManage.EventBus == nil {
		logger.Warnf(ctx, "EventBus not available for streaming fallback, falling back to fixed response")
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	// Get chat model
	chatModel, err := s.modelService.GetChatModel(types.WithTenantID(ctx, chatManage.ModelTenantID), chatManage.ChatModelID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get chat model for fallback: %v, falling back to fixed response", err)
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	// Prepare chat options
	thinking := false
	opt := &chat.ChatOptions{
		Temperature:         chatManage.SummaryConfig.Temperature,
		MaxCompletionTokens: chatManage.SummaryConfig.MaxCompletionTokens,
		Thinking:            &thinking,
	}

	// Start streaming response
	responseChan, err := chatModel.ChatStream(ctx, buildFallbackMessages(chatManage, promptContent), opt)
	if err != nil {
		logger.Errorf(ctx, "Failed to start streaming fallback response: %v, falling back to fixed response", err)
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	if responseChan == nil {
		logger.Errorf(ctx, "Chat stream returned nil channel, falling back to fixed response")
		s.handleFixedFallback(ctx, chatManage)
		return
	}

	// Start goroutine to consume stream and emit events
	go s.consumeFallbackStream(ctx, chatManage, responseChan)
}

func buildFallbackMessages(chatManage *types.ChatManage, promptContent string) []chat.Message {
	messages := make([]chat.Message, 0, len(chatManage.History)*2+1)
	messages = chatpipeline.AppendHistoryMessages(messages, chatManage.History)

	userMsg := chat.Message{Role: "user", Content: promptContent}
	if chatManage.ChatModelSupportsVision && len(chatManage.Images) > 0 {
		userMsg.Images = chatManage.Images
	}

	return append(messages, userMsg)
}

// renderFallbackPrompt renders the fallback prompt template with query and image context.
func (s *sessionService) renderFallbackPrompt(ctx context.Context, chatManage *types.ChatManage) (string, error) {
	query := chatManage.Query
	if rq := strings.TrimSpace(chatManage.RewriteQuery); rq != "" {
		query = rq
	}

	kbDocuments := s.buildKBDocumentListing(ctx, chatManage)

	result := types.RenderPromptPlaceholders(chatManage.FallbackPrompt, types.PlaceholderValues{
		"query":        query,
		"language":     chatManage.Language,
		"kb_documents": kbDocuments,
	})

	if chatManage.ImageDescription != "" && !chatManage.ChatModelSupportsVision {
		result += "\n\n[用户上传图片内容]\n" + chatManage.ImageDescription
	}
	if chatManage.QuotedContext != "" {
		result += "\n\n" + chatManage.QuotedContext
	}
	return result, nil
}

// buildKBDocumentListing returns a concise listing of documents in the knowledge bases
// associated with the current pipeline. This gives the LLM visibility into KB contents
// when vector/keyword search returns empty (e.g., broad browse queries).
func (s *sessionService) buildKBDocumentListing(ctx context.Context, chatManage *types.ChatManage) string {
	// Collect unique KB IDs from search targets
	kbIDs := make(map[string]uint64)
	for _, t := range chatManage.SearchTargets {
		kbIDs[t.KnowledgeBaseID] = t.TenantID
	}
	for _, id := range chatManage.KnowledgeBaseIDs {
		if _, ok := kbIDs[id]; !ok {
			kbIDs[id] = chatManage.TenantID
		}
	}
	if len(kbIDs) == 0 {
		return ""
	}

	const maxDocuments = 50
	var b strings.Builder
	total := 0

	for kbID, tenantID := range kbIDs {
		if total >= maxDocuments {
			break
		}
		knowledges, err := s.knowledgeService.ListKnowledgeByKnowledgeBaseID(types.WithTenantID(ctx, tenantID), kbID)
		if err != nil {
			logger.Warnf(ctx, "buildKBDocumentListing: failed to list knowledge for KB %s: %v", kbID, err)
			continue
		}
		for _, k := range knowledges {
			if total >= maxDocuments {
				break
			}
			if k.EnableStatus != "enabled" {
				continue
			}
			title := k.Title
			if title == "" {
				title = k.FileName
			}
			if title == "" {
				continue
			}
			fmt.Fprintf(&b, "- %s", title)
			if k.FileType != "" {
				fmt.Fprintf(&b, " (%s)", k.FileType)
			}
			if k.Description != "" {
				desc := k.Description
				if len([]rune(desc)) > 100 {
					desc = string([]rune(desc)[:100]) + "..."
				}
				fmt.Fprintf(&b, ": %s", desc)
			}
			b.WriteString("\n")
			total++
		}
	}

	if b.Len() == 0 {
		return ""
	}

	if total >= maxDocuments {
		fmt.Fprintf(&b, "... (showing first %d documents)\n", maxDocuments)
	}

	return b.String()
}

// consumeFallbackStream consumes the streaming response and emits events
func (s *sessionService) consumeFallbackStream(
	ctx context.Context,
	chatManage *types.ChatManage,
	responseChan <-chan types.StreamResponse,
) {
	fallbackID := generateEventID("fallback")
	eventBus := chatManage.EventBus
	var finalContent string
	streamCompleted := false

	for response := range responseChan {
		// Emit event for each answer chunk
		if response.ResponseType == types.ResponseTypeAnswer {
			finalContent += response.Content
			if err := eventBus.Emit(ctx, types.Event{
				ID:        fallbackID,
				Type:      types.EventType(event.EventAgentFinalAnswer),
				SessionID: chatManage.SessionID,
				Data: event.AgentFinalAnswerData{
					Content:    response.Content,
					Done:       response.Done,
					IsFallback: true,
				},
			}); err != nil {
				logger.Errorf(ctx, "Failed to emit fallback answer chunk event: %v", err)
			}

			// Update ChatResponse with final content when done
			if response.Done {
				chatManage.ChatResponse = &types.ChatResponse{Content: finalContent}
				streamCompleted = true
				logger.Infof(ctx, "Fallback streaming response completed")
				break
			}
		}
	}

	// If channel closed without Done=true, emit final event with fixed response
	if !streamCompleted {
		logger.Warnf(ctx, "Fallback stream closed without completion, emitting final event with fixed response")
		s.emitFallbackAnswer(ctx, chatManage, chatManage.FallbackResponse)
	}
}

// emitKnowledgeReferencesEvent streams retrieved chunks to the client as a
// `references` SSE event. Must run before CHAT_COMPLETION_STREAM so citations
// arrive while the connection is still open (complete closes the stream).
func emitKnowledgeReferencesEvent(ctx context.Context, chatManage *types.ChatManage) {
	if chatManage == nil || chatManage.EventBus == nil || len(chatManage.MergeResult) == 0 {
		return
	}
	logger.Infof(ctx, "Emitting references event with %d results (pre-answer)", len(chatManage.MergeResult))
	if err := chatManage.EventBus.Emit(ctx, types.Event{
		ID:        generateEventID("references"),
		Type:      types.EventType(event.EventAgentReferences),
		SessionID: chatManage.SessionID,
		Data: event.AgentReferencesData{
			References: chatManage.MergeResult,
		},
	}); err != nil {
		logger.Errorf(ctx, "Failed to emit references event: %v", err)
	}
}

// emitFallbackAnswer emits fallback answer event
func (s *sessionService) emitFallbackAnswer(ctx context.Context, chatManage *types.ChatManage, content string) {
	if chatManage.EventBus == nil {
		return
	}

	fallbackID := generateEventID("fallback")
	if err := chatManage.EventBus.Emit(ctx, types.Event{
		ID:        fallbackID,
		Type:      types.EventType(event.EventAgentFinalAnswer),
		SessionID: chatManage.SessionID,
		Data: event.AgentFinalAnswerData{
			Content:    content,
			Done:       true,
			IsFallback: true,
		},
	}); err != nil {
		logger.Errorf(ctx, "Failed to emit fallback answer event: %v", err)
	} else {
		logger.Infof(ctx, "Fallback answer event emitted successfully")
	}
}

// resolveWebSearchProviderID returns the web search provider ID to use for a pipeline request.
// Priority: agent config > tenant default (is_default=true)
func (s *sessionService) resolveWebSearchProviderID(ctx context.Context, req *types.QARequest, tenantID uint64) string {
	// 1. Agent-level override
	if req.AnswerMode != nil && req.AnswerMode.Config.WebSearchProviderID != "" {
		return req.AnswerMode.Config.WebSearchProviderID
	}
	// 2. Tenant default
	if s.webSearchProviderRepo != nil {
		if defaultProvider, err := s.webSearchProviderRepo.GetDefault(ctx, tenantID); err == nil && defaultProvider != nil {
			return defaultProvider.ID
		}
	}
	return ""
}

// resolveWebFetchEnabled returns whether auto web fetch is enabled for this request.
func (s *sessionService) resolveWebFetchEnabled(req *types.QARequest) bool {
	if req.AnswerMode != nil {
		return req.AnswerMode.Config.WebFetchEnabled
	}
	return false
}

// resolveWebFetchTopN returns how many pages to fetch after rerank.
func (s *sessionService) resolveWebFetchTopN(req *types.QARequest) int {
	if req.AnswerMode != nil && req.AnswerMode.Config.WebFetchTopN > 0 {
		return req.AnswerMode.Config.WebFetchTopN
	}
	return 3
}

// resolveWebSearchMaxResults returns the max results for web search.
// Priority: agent config > tenant default > default (10)
func (s *sessionService) resolveWebSearchMaxResults(ctx context.Context, req *types.QARequest) int {
	if req.AnswerMode != nil && req.AnswerMode.Config.WebSearchMaxResults > 0 {
		return req.AnswerMode.Config.WebSearchMaxResults
	}
	tenantInfo, _ := types.TenantInfoFromContext(ctx)
	if tenantInfo != nil {
		return types.EffectiveWebSearchConfig(tenantInfo.WebSearchConfig).MaxResults
	}
	return types.DefaultWebSearchMaxResults
}
