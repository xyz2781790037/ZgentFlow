package service

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// ---------------------------------------------------------------------------
// Shared quick-QA helpers: KB resolution, model resolution, retrieval tenant
// ---------------------------------------------------------------------------

// resolveKnowledgeBases resolves the effective knowledge base IDs and knowledge IDs
// for a QA request. Priority:
//  1. Explicit @mentions (request-specified kbIDs / knowledgeIDs)
//  2. RetrieveKBOnlyWhenMentioned -> disable KB if no mention
//  3. Quick mode's configured knowledge bases (via KBSelectionMode)
func (s *sessionService) resolveKnowledgeBases(
	ctx context.Context,
	req *types.QARequest,
) (kbIDs []string, knowledgeIDs []string) {
	kbIDs = req.KnowledgeBaseIDs
	knowledgeIDs = req.KnowledgeIDs
	answerMode := req.AnswerMode

	hasExplicitMention := len(kbIDs) > 0 || len(knowledgeIDs) > 0
	if answerMode != nil {
		logger.Infof(ctx, "KB resolution: hasExplicitMention=%v, RetrieveKBOnlyWhenMentioned=%v, KBSelectionMode=%s",
			hasExplicitMention, answerMode.Config.RetrieveKBOnlyWhenMentioned, answerMode.Config.KBSelectionMode)
	}

	if hasExplicitMention {
		logger.Infof(ctx, "Using request-specified targets: kbs=%v, docs=%v", kbIDs, knowledgeIDs)
	} else if answerMode != nil && answerMode.Config.RetrieveKBOnlyWhenMentioned {
		kbIDs = nil
		knowledgeIDs = nil
		logger.Infof(ctx, "RetrieveKBOnlyWhenMentioned is enabled and no @ mention found, KB retrieval disabled for this request")
	} else if answerMode != nil {
		kbIDs = s.resolveKnowledgeBasesFromMode(ctx, answerMode)
	}
	return kbIDs, knowledgeIDs
}

// resolveChatModelID resolves the effective chat model ID for a QA request.
// The workspace default is authoritative for every built-in answer mode.
func (s *sessionService) resolveChatModelID(
	ctx context.Context,
	req *types.QARequest,
	knowledgeBaseIDs []string,
	knowledgeIDs []string,
) (string, error) {
	return s.selectChatModelID(ctx, req.Session, knowledgeBaseIDs, knowledgeIDs)
}

// resolveRetrievalTenantID returns the fixed workspace partition used by the request.
func (s *sessionService) resolveRetrievalTenantID(
	ctx context.Context,
	req *types.QARequest,
) uint64 {
	session := req.Session

	retrievalTenantID := session.TenantID
	if v := ctx.Value(types.TenantIDContextKey); v != nil {
		if tid, ok := v.(uint64); ok && tid != 0 {
			retrievalTenantID = tid
		}
	}
	return retrievalTenantID
}

// applyAnswerModeOverridesToChatManage applies answer mode configuration overrides
// to a ChatManage object that was initialized with system defaults.
// This covers: system prompt, context template, temperature, max tokens, thinking,
// retrieval thresholds, rewrite settings, fallback settings, FAQ strategy, and history turns.
func (s *sessionService) applyAnswerModeOverridesToChatManage(
	ctx context.Context,
	answerMode *types.AnswerMode,
	cm *types.ChatManage,
) {
	if answerMode == nil {
		return
	}

	// Ensure defaults are set
	answerMode.EnsureDefaults()

	// Override summary config fields
	if answerMode.Config.SystemPrompt != "" {
		cm.SummaryConfig.Prompt = answerMode.Config.SystemPrompt
		logger.Infof(ctx, "Using answer mode's system_prompt")
	}
	if answerMode.Config.ContextTemplate != "" {
		cm.SummaryConfig.ContextTemplate = answerMode.Config.ContextTemplate
		logger.Infof(ctx, "Using answer mode's context_template")
	}
	if answerMode.Config.Temperature >= 0 {
		cm.SummaryConfig.Temperature = answerMode.Config.Temperature
		logger.Infof(ctx, "Using answer mode's temperature: %f", answerMode.Config.Temperature)
	}
	if answerMode.Config.MaxCompletionTokens > 0 {
		cm.SummaryConfig.MaxCompletionTokens = answerMode.Config.MaxCompletionTokens
		logger.Infof(ctx, "Using answer mode's max_completion_tokens: %d", answerMode.Config.MaxCompletionTokens)
	}
	// Mode-level thinking setting takes full control (no global fallback)
	cm.SummaryConfig.Thinking = answerMode.Config.Thinking
	if answerMode.Config.Thinking != nil {
		logger.Infof(ctx, "Using answer mode's thinking: %v", *answerMode.Config.Thinking)
	}

	// Override retrieval strategy settings
	if answerMode.Config.EmbeddingTopK > 0 {
		cm.EmbeddingTopK = answerMode.Config.EmbeddingTopK
	}
	if answerMode.Config.KeywordThreshold > 0 {
		cm.KeywordThreshold = answerMode.Config.KeywordThreshold
	}
	if answerMode.Config.VectorThreshold > 0 {
		cm.VectorThreshold = answerMode.Config.VectorThreshold
	}
	if answerMode.Config.RerankTopK > 0 {
		cm.RerankTopK = answerMode.Config.RerankTopK
	}
	cm.RerankThreshold = answerMode.Config.RerankThreshold
	if answerMode.Config.RerankModelID != "" {
		cm.RerankModelID = answerMode.Config.RerankModelID
	}

	// Override rewrite settings
	cm.EnableRewrite = answerMode.Config.EnableRewrite
	cm.EnableQueryExpansion = answerMode.Config.EnableQueryExpansion
	if answerMode.Config.RewritePromptSystem != "" {
		cm.RewritePromptSystem = answerMode.Config.RewritePromptSystem
	}
	if answerMode.Config.RewritePromptUser != "" {
		cm.RewritePromptUser = answerMode.Config.RewritePromptUser
	}
	if answerMode.Config.QueryUnderstandModelID != "" {
		cm.QueryUnderstandModelID = answerMode.Config.QueryUnderstandModelID
		logger.Infof(ctx, "Using answer mode's query_understand_model_id: %s",
			answerMode.Config.QueryUnderstandModelID)
	}

	// Override fallback settings
	if answerMode.Config.FallbackStrategy != "" {
		cm.FallbackStrategy = types.FallbackStrategy(answerMode.Config.FallbackStrategy)
	}
	if answerMode.Config.FallbackResponse != "" {
		cm.FallbackResponse = answerMode.Config.FallbackResponse
	}
	if answerMode.Config.FallbackPrompt != "" {
		cm.FallbackPrompt = answerMode.Config.FallbackPrompt
	}

	// Override web search settings
	if answerMode.Config.WebSearchMaxResults > 0 {
		cm.WebSearchMaxResults = answerMode.Config.WebSearchMaxResults
	}

	// Override history turns
	if answerMode.Config.HistoryTurns > 0 {
		cm.MaxRounds = answerMode.Config.HistoryTurns
		logger.Infof(ctx, "Using answer mode's history_turns: %d", cm.MaxRounds)
	}
	if !answerMode.Config.MultiTurnEnabled {
		cm.MaxRounds = 0
		logger.Infof(ctx, "Multi-turn disabled by answer mode, clearing history")
	}

	// FAQ strategy settings
	cm.FAQPriorityEnabled = answerMode.Config.FAQPriorityEnabled
	cm.FAQDirectAnswerThreshold = answerMode.Config.FAQDirectAnswerThreshold
	cm.FAQScoreBoost = answerMode.Config.FAQScoreBoost
	if cm.FAQPriorityEnabled {
		logger.Infof(ctx, "FAQ priority enabled: threshold=%.2f, boost=%.2f",
			cm.FAQDirectAnswerThreshold, cm.FAQScoreBoost)
	}

	if len(answerMode.Config.IntentPrompts) > 0 {
		cm.IntentPromptOverrides = answerMode.Config.IntentPrompts
		logger.Infof(ctx, "Using answer mode's intent_prompts (%d overrides)", len(cm.IntentPromptOverrides))
	}
}
