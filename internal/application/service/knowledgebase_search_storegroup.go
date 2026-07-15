package service

import (
	"context"
	"fmt"

	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// storeGroup carries the fixed pgvector engine and immutable retrieval
// parameters reused by FAQ retries.
type storeGroup struct {
	Engine     *retriever.CompositeRetrieveEngine
	BaseParams []types.RetrieveParams
	TopK       int
}

// resolveStoreGroups builds one retrieval unit for every selected knowledge
// base. The plural return shape is retained for queued-task compatibility.
func (s *knowledgeBaseService) resolveStoreGroups(
	ctx context.Context,
	primary *types.KnowledgeBase,
	kbs []*types.KnowledgeBase,
	params types.SearchParams,
	matchCount int,
) ([]*storeGroup, error) {
	engine, err := retriever.CreateRetrieveEngineForKB(
		ctx, s.retrieveEngine, s.ownership, types.MustTenantIDFromContext(ctx), nil)
	if err != nil {
		return nil, err
	}
	baseParams, err := s.buildRetrievalParams(ctx, engine, primary, kbs, params, matchCount)
	if err != nil {
		return nil, fmt.Errorf("build pgvector retrieval params: %w", err)
	}
	return []*storeGroup{{Engine: engine, BaseParams: baseParams, TopK: matchCount}}, nil
}

// authorizeKBAccess rejects searches whose scope includes a knowledge base
// not owned by the current tenant.
//
// Returning NotFound rather than Forbidden avoids leaking the existence
// of unauthorized KB IDs that the caller could not otherwise observe.
// Structured logs record the rejection with the offending kb_id (always
// safe — KB IDs are UUIDs without sensitive content) and the requesting
// tenant for audit.
func (s *knowledgeBaseService) authorizeKBAccess(
	ctx context.Context,
	kbs []*types.KnowledgeBase,
	requestTenantID uint64,
) error {
	if len(kbs) == 0 {
		return nil
	}

	for _, kb := range kbs {
		if kb.TenantID == requestTenantID {
			continue
		}
		logger.WarnWithFields(ctx, logger.Fields{
			"caller_tenant_id": requestTenantID,
			"kb_tenant_id":     kb.TenantID,
			"kb_id":            kb.ID,
			"reason":           "knowledge base belongs to another tenant",
		}, "search scope rejected: foreign-tenant knowledge base")
		return apperrors.NewNotFoundError("knowledge base not found")
	}
	return nil
}

// validateSameEmbeddingModel rejects multi-KB searches that span more than
// one resolved embedding-model identity key. Single-KB calls no-op.
//
// Wiki-only KBs (empty resolved key) are tolerated: if every
// KB lacks an embedding model, validation passes and HybridSearch returns
// an empty result set via the allBaseParamsEmpty fast path.
//
// Log fields are sanitized via secutils.SanitizeForLog because resolved
// keys are derived from model.Parameters.BaseURL, which is tenant-
// configured and can contain CR/LF or other control characters.
func (s *knowledgeBaseService) validateSameEmbeddingModel(
	ctx context.Context,
	kbs []*types.KnowledgeBase,
) error {
	if len(kbs) <= 1 {
		return nil
	}
	keys := s.ResolveEmbeddingModelKeys(ctx, kbs)
	var seen string
	for _, kb := range kbs {
		k, ok := keys[kb.ID]
		if !ok || k == "" {
			// Wiki-only carve-out: KB has no embedding model.
			continue
		}
		if seen == "" {
			seen = k
			continue
		}
		if k != seen {
			logger.WarnWithFields(ctx, logger.Fields{
				"primary_key": secutils.SanitizeForLog(seen),
				"diverging":   secutils.SanitizeForLog(k),
				"kb_id":       kb.ID,
				"kb_count":    len(kbs),
			}, "multi-KB search rejected: embedding models differ")
			return apperrors.NewBadRequestError(
				"selected knowledge bases use different embedding models; " +
					"multi-KB search requires every knowledge base to share a single embedding model")
		}
	}
	return nil
}
