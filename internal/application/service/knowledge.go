package service

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/xyz2781790037/ZealRAG/internal/application/service/retriever"
	"github.com/xyz2781790037/ZealRAG/internal/config"
	werrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// Error definitions for knowledge service operations
var (
	// ErrInvalidFileType is returned when an unsupported file type is provided
	ErrInvalidFileType = errors.New("unsupported file type")
	// ErrChunkNotFound is returned when a requested chunk cannot be found
	ErrChunkNotFound = errors.New("chunk not found")
	// ErrDuplicateFile is returned when trying to add a file that already exists
	ErrDuplicateFile = errors.New("file already exists")
)

// knowledgeService implements the knowledge service interface
// service 实现知识服务接口
type knowledgeService struct {
	config          *config.Config
	retrieveEngine  interfaces.RetrieveEngineRegistry
	ownership       retriever.TenantStoreOwnership
	repo            interfaces.KnowledgeRepository
	kbService       interfaces.KnowledgeBaseService
	tenantRepo      interfaces.TenantRepository
	tenantService   interfaces.TenantService
	documentReader  interfaces.DocumentReader
	chunkService    interfaces.ChunkService
	chunkRepo       interfaces.ChunkRepository
	fileSvc         interfaces.FileService
	modelService    interfaces.ModelService
	task            interfaces.TaskEnqueuer
	taskInspector   interfaces.TaskInspector
	redisClient     *redis.Client
	taskPendingRepo interfaces.TaskPendingOpsRepository

	// spanTracker records the per-attempt span tree for the parsing
	// pipeline. Best-effort: a nil tracker (test harness) is safely
	// handled because the public surface is the SpanTracker interface,
	// which has a no-op fallback. See knowledge_span_tracker.go.
	spanTracker SpanTracker
}

const (
	faqImportBatchSize = 50 // 每批处理的FAQ条目数
)

// NewKnowledgeService creates a new knowledge service instance
func NewKnowledgeService(
	config *config.Config,
	repo interfaces.KnowledgeRepository,
	documentReader interfaces.DocumentReader,
	kbService interfaces.KnowledgeBaseService,
	tenantRepo interfaces.TenantRepository,
	tenantService interfaces.TenantService,
	chunkService interfaces.ChunkService,
	chunkRepo interfaces.ChunkRepository,
	fileSvc interfaces.FileService,
	modelService interfaces.ModelService,
	task interfaces.TaskEnqueuer,
	taskInspector interfaces.TaskInspector,
	retrieveEngine interfaces.RetrieveEngineRegistry,
	ownership retriever.TenantStoreOwnership,
	redisClient *redis.Client,
	taskPendingRepo interfaces.TaskPendingOpsRepository,
	spanTracker SpanTracker,
) (interfaces.KnowledgeService, error) {
	return &knowledgeService{
		config:          config,
		repo:            repo,
		kbService:       kbService,
		tenantRepo:      tenantRepo,
		tenantService:   tenantService,
		documentReader:  documentReader,
		chunkService:    chunkService,
		chunkRepo:       chunkRepo,
		fileSvc:         fileSvc,
		modelService:    modelService,
		task:            task,
		taskInspector:   taskInspector,
		retrieveEngine:  retrieveEngine,
		ownership:       ownership,
		redisClient:     redisClient,
		taskPendingRepo: taskPendingRepo,
		spanTracker:     spanTracker,
	}, nil
}

// tracker returns a usable SpanTracker — falls back to a no-op when the
// service was constructed without one, such as in a test harness.
// All pipeline call sites go through this so they never need a nil check.
func (s *knowledgeService) tracker() SpanTracker {
	if s.spanTracker == nil {
		return noopSpanTracker{}
	}
	return s.spanTracker
}

// attemptCtxKey scopes the per-task attempt number to a single execution.
// Set once at the start of ProcessDocument / ProcessManualUpdate /
// KnowledgePostProcess so every nested tracker call within the same task
// can locate the right attempt without threading it through signatures.
type attemptCtxKey struct{}

// withAttempt returns a child ctx tagged with the given attempt number.
// Pass through every call site that may invoke the tracker.
func withAttempt(ctx context.Context, attempt int) context.Context {
	if attempt <= 0 {
		return ctx
	}
	return context.WithValue(ctx, attemptCtxKey{}, attempt)
}

// attemptFromCtx extracts the attempt number stored by withAttempt;
// returns 0 when missing (legacy paths or tests). Tracker call sites
// treat 0 as "skip recording" since we have no attempt to anchor under.
func attemptFromCtx(ctx context.Context) int {
	if v, ok := ctx.Value(attemptCtxKey{}).(int); ok {
		return v
	}
	return 0
}

// attemptSuperseded reports whether a newer parse attempt has started for the
// knowledge since this enrichment subtask was enqueued. Stale subtasks from a
// previous upload/edit/reparse that is still draining must NOT touch the new
// attempt's chunks or decrement its pending_subtasks_count — doing so would
// race-promote the row to completed before the new attempt finishes. An attempt
// of 0 predates attempt tracking (or tracking is disabled) and is never treated
// as superseded.
func attemptSuperseded(ctx context.Context, tracker SpanTracker, knowledgeID string, attempt int) bool {
	if attempt <= 0 || knowledgeID == "" {
		return false
	}
	return tracker.LatestAttempt(ctx, knowledgeID) > attempt
}

// finalizeSubtaskDetachedTimeout bounds the detached decrement so a wedged DB
// connection can't hang a worker goroutine forever in its terminal defer.
const finalizeSubtaskDetachedTimeout = 10 * time.Second

// finalizeSubtaskDetached evaluates the drain decision for a subtask's
// terminal exit and — when the subtask should drain — decrements
// pending_subtasks_count using a context DETACHED from the caller's
// cancellation.
//
// Decision: a subtask drains exactly once, on its terminal exit, UNLESS a newer
// attempt superseded it. "Terminal" means either the handler succeeded
// (retErr == nil) or it's the final asynq attempt (final). A non-final failure
// returns without draining because asynq will retry.
//
// Why detach: the decrement runs after the handler body, often as the very
// last thing a worker does. If it rode the task ctx, a cancelled ctx (graceful
// shutdown, a worker being preempted, or the task being interrupted under
// load) would make the DB UPDATE fail. That failure is only logged and
// swallowed, and because enrichment handlers frequently still return success
// (per-chunk LLM errors are tolerated, not propagated), asynq never retries —
// so the slot is never drained and the parent knowledge is stranded in
// "finalizing" forever with a non-zero counter. Detaching keeps the counter
// correct across cancellation; a bounded timeout guards against a wedged DB.
//
// source is a free-form tag (e.g. "question_batch[3]", "summary", "wiki")
// used to attribute a decrement failure to a specific subtask in logs.
func finalizeSubtaskDetached(
	ctx context.Context,
	repo interfaces.KnowledgeRepository,
	knowledgeID, source string,
	retErr error,
	superseded, final bool,
) {
	willDrain := repo != nil && knowledgeID != "" && !superseded && (retErr == nil || final)
	if !willDrain {
		return
	}
	dctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), finalizeSubtaskDetachedTimeout)
	defer cancel()
	if _, _, err := repo.FinalizeSubtask(dctx, knowledgeID); err != nil {
		logger.Warnf(ctx, "finalize subtask decrement failed source=%s knowledge=%s err=%v",
			source, knowledgeID, err)
	}
}

// beginStage / endStage / failStage / skipStage are the by-name shims
// the pipeline uses so call sites don't have to thread *Span values
// through the existing function signatures. Each helper looks up the
// stage from (kid, attempt-from-ctx, stageName) at write time — costs
// one extra DB read per terminal transition (≤ a dozen per knowledge),
// which is dwarfed by the work the stages themselves do.
func (s *knowledgeService) beginStage(ctx context.Context, kid, name string, input types.JSONMap) {
	a := attemptFromCtx(ctx)
	if a <= 0 {
		return
	}
	s.tracker().BeginStage(ctx, kid, a, name, input)
}

func (s *knowledgeService) endStage(ctx context.Context, kid, name string, output types.JSONMap) {
	a := attemptFromCtx(ctx)
	if a <= 0 {
		return
	}
	span := s.tracker().LookupStage(ctx, kid, a, name)
	if span == nil {
		return
	}
	s.tracker().EndSpan(ctx, span, output)
}

func (s *knowledgeService) failStage(ctx context.Context, kid, name, code, msg string, err error) {
	a := attemptFromCtx(ctx)
	if a <= 0 {
		return
	}
	span := s.tracker().LookupStage(ctx, kid, a, name)
	if span == nil {
		return
	}
	s.tracker().FailSpan(ctx, span, code, msg, err)
}

func (s *knowledgeService) skipStage(ctx context.Context, kid, name, reason string) {
	a := attemptFromCtx(ctx)
	if a <= 0 {
		return
	}
	span := s.tracker().LookupStage(ctx, kid, a, name)
	if span == nil {
		// No begin recorded — synthesize a span row for skipped state.
		// Use BeginStage with no input then SkipSpan to keep schema
		// invariants (started_at / kind set).
		span = s.tracker().BeginStage(ctx, kid, a, name, nil)
	}
	s.tracker().SkipSpan(ctx, span, reason)
}

// beginPostprocessSubspan opens a subspan beneath the postprocess stage
// span for (kid, attempt). Async post-pipeline tasks (summary, question,
// graph, wiki) call this on entry so their actual processing time shows
// up in the trace tree under postprocess instead of the stage looking
// like an instant ~10ms enqueue.
//
// Returns nil when:
//   - attempt <= 0 (legacy in-flight task without span tracking)
//   - the postprocess stage span is missing (parse predates tracker, or
//     the upstream BeginStage call failed)
//
// Callers must tolerate nil — pair every begin with a deferred
// endPostprocessSubspan / failPostprocessSubspan that no-ops on nil.
func (s *knowledgeService) beginPostprocessSubspan(
	ctx context.Context, knowledgeID string, attempt int, name string, input types.JSONMap,
) *Span {
	if attempt <= 0 || knowledgeID == "" || name == "" {
		return nil
	}
	parent := s.tracker().LookupStage(ctx, knowledgeID, attempt, types.StagePostProcess)
	if parent == nil {
		return nil
	}
	return s.tracker().BeginSubSpan(ctx, parent, name, types.SpanKindSubSpan, input)
}

// beginQuestionBatchSubspan opens a per-batch question subspan under the
// "postprocess.question" grouping span created by the orchestrator, falling
// back to the postprocess stage when the group span isn't found (legacy
// in-flight tasks or a tracker that skipped it). Mirrors beginPostprocessSubspan
// but resolves the grouping parent first.
func (s *knowledgeService) beginQuestionBatchSubspan(
	ctx context.Context, knowledgeID string, attempt int, name string, input types.JSONMap,
) *Span {
	if attempt <= 0 || knowledgeID == "" || name == "" {
		return nil
	}
	parent := s.tracker().LookupSpanByName(ctx, knowledgeID, attempt, postprocessQuestionGroupSpanName)
	if parent == nil {
		parent = s.tracker().LookupStage(ctx, knowledgeID, attempt, types.StagePostProcess)
	}
	if parent == nil {
		return nil
	}
	return s.tracker().BeginSubSpan(ctx, parent, name, types.SpanKindSubSpan, input)
}

func (s *knowledgeService) endPostprocessSubspan(ctx context.Context, span *Span, output types.JSONMap) {
	if span == nil {
		return
	}
	s.tracker().EndSpan(ctx, span, output)
}

func (s *knowledgeService) failPostprocessSubspan(
	ctx context.Context, span *Span, code, msg string, err error,
) {
	if span == nil {
		return
	}
	s.tracker().FailSpan(ctx, span, code, msg, err)
}

// getParserEngineOverridesFromContext returns parser engine overrides from tenant in context (e.g. MinerU endpoint, API key).
// Used when building document ReadRequest so UI-configured values take precedence over env.
func (s *knowledgeService) getParserEngineOverridesFromContext(ctx context.Context) map[string]string {
	if v := ctx.Value(types.TenantInfoContextKey); v != nil {
		if tenant, ok := v.(*types.Tenant); ok && tenant != nil {
			return tenant.ParserEngineConfig.ToOverridesMap()
		}
	}
	return nil
}

// GetRepository gets the knowledge repository
// Parameters:
//   - ctx: Context with authentication and request information
//
// Returns:
//   - interfaces.KnowledgeRepository: Knowledge repository
func (s *knowledgeService) GetRepository() interfaces.KnowledgeRepository {
	return s.repo
}

// isKnowledgeDeleting checks if a knowledge entry is being deleted.
// This is used to prevent async tasks from conflicting with deletion operations.
func (s *knowledgeService) isKnowledgeDeleting(ctx context.Context, tenantID uint64, knowledgeID string) bool {
	knowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledgeID)
	if err != nil {
		// If we can't find the knowledge, assume it's deleted
		logger.Warnf(ctx, "Failed to check knowledge deletion status (assuming deleted): %v", err)
		return true
	}
	if knowledge == nil {
		return true
	}
	return knowledge.ParseStatus == types.ParseStatusDeleting
}

// isKnowledgeAborted returns (true, status) when the knowledge has been
// marked as deleting OR cancelled so async pipeline workers should bail
// out. Status is returned so callers can branch on cleanup behavior:
// deleting → existing cleanup of partial chunks/index applies;
// cancelled → keep partially written data per user expectation.
//
// When the row is missing or unreadable we conservatively return
// (true, ParseStatusDeleting): the existing deleting branch already
// handles cleanup-or-no-op semantics safely.
func (s *knowledgeService) isKnowledgeAborted(
	ctx context.Context, tenantID uint64, knowledgeID string,
) (bool, string) {
	knowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, knowledgeID)
	if err != nil {
		logger.Warnf(ctx, "Failed to check knowledge abort status (assuming deleted): %v", err)
		return true, types.ParseStatusDeleting
	}
	if knowledge == nil {
		return true, types.ParseStatusDeleting
	}
	switch knowledge.ParseStatus {
	case types.ParseStatusDeleting, types.ParseStatusCancelled:
		return true, knowledge.ParseStatus
	}
	return false, knowledge.ParseStatus
}

// GetKnowledgeByID retrieves a knowledge entry by its ID
func (s *knowledgeService) GetKnowledgeByID(ctx context.Context, id string) (*types.Knowledge, error) {
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)

	knowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_id": id,
			"tenant_id":    tenantID,
		})
		return nil, err
	}

	logger.Infof(ctx, "Knowledge retrieved successfully, ID: %s, type: %s", knowledge.ID, knowledge.Type)
	return knowledge, nil
}

// ListKnowledgeByKnowledgeBaseID returns all knowledge entries in a knowledge base
func (s *knowledgeService) ListKnowledgeByKnowledgeBaseID(ctx context.Context,
	kbID string,
) ([]*types.Knowledge, error) {
	return s.repo.ListKnowledgeByKnowledgeBaseID(ctx, ctx.Value(types.TenantIDContextKey).(uint64), kbID)
}

// ListPagedKnowledgeByKnowledgeBaseID returns paginated knowledge entries in a knowledge base
func (s *knowledgeService) ListPagedKnowledgeByKnowledgeBaseID(ctx context.Context,
	kbID string, page *types.Pagination, filter types.KnowledgeListFilter,
) (*types.PageResult, error) {
	knowledges, total, err := s.repo.ListPagedKnowledgeByKnowledgeBaseID(ctx,
		ctx.Value(types.TenantIDContextKey).(uint64), kbID, page, filter)
	if err != nil {
		return nil, err
	}

	return types.NewPageResult(total, page, knowledges), nil
}

// GetKnowledgeFile retrieves the physical file associated with a knowledge entry
func (s *knowledgeService) GetKnowledgeFile(ctx context.Context, id string) (io.ReadCloser, string, error) {
	// Get knowledge record
	tenantID := ctx.Value(types.TenantIDContextKey).(uint64)
	knowledge, err := s.repo.GetKnowledgeByID(ctx, tenantID, id)
	if err != nil {
		return nil, "", err
	}

	// Resolve KB-level file service with FilePath fallback protection
	kb, _ := s.kbService.GetKnowledgeBaseByID(ctx, knowledge.KnowledgeBaseID)
	file, err := s.resolveFileServiceForPath(ctx, kb, knowledge.FilePath).GetFile(ctx, knowledge.FilePath)
	if err != nil {
		return nil, "", err
	}

	return file, knowledge.FileName, nil
}

func (s *knowledgeService) UpdateKnowledge(ctx context.Context, knowledge *types.Knowledge) error {
	record, err := s.repo.GetKnowledgeByID(ctx, ctx.Value(types.TenantIDContextKey).(uint64), knowledge.ID)
	if err != nil {
		logger.Errorf(ctx, "Failed to get knowledge record: %v", err)
		return err
	}
	// if need other fields update, please add here
	if knowledge.Title != "" {
		record.Title = knowledge.Title
	}
	if knowledge.Description != "" {
		record.Description = knowledge.Description
	}

	// Update knowledge record in the repository
	if err := s.repo.UpdateKnowledge(ctx, record); err != nil {
		logger.Errorf(ctx, "Failed to update knowledge: %v", err)
		return err
	}
	logger.Infof(ctx, "Knowledge updated successfully, ID: %s", knowledge.ID)
	return nil
}

// GetKnowledgeBatch retrieves multiple knowledge entries by their IDs
func (s *knowledgeService) GetKnowledgeBatch(ctx context.Context,
	tenantID uint64, ids []string,
) ([]*types.Knowledge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	return s.repo.GetKnowledgeBatch(ctx, tenantID, ids)
}

// SearchKnowledge searches document knowledge items in the current tenant.
// fileTypes: optional list of file extensions to filter by (e.g., ["csv", "xlsx"])
func (s *knowledgeService) SearchKnowledge(ctx context.Context, keyword string, offset, limit int, fileTypes []string) ([]*types.Knowledge, bool, error) {
	tenantID, ok := ctx.Value(types.TenantIDContextKey).(uint64)
	if !ok {
		return nil, false, werrors.NewUnauthorizedError("Tenant ID not found in context")
	}

	scopes := make([]types.KnowledgeSearchScope, 0)

	ownKBs, err := s.kbService.ListKnowledgeBases(ctx)
	if err == nil {
		for _, kb := range ownKBs {
			if kb != nil && kb.Type == types.KnowledgeBaseTypeDocument {
				scopes = append(scopes, types.KnowledgeSearchScope{TenantID: tenantID, KBID: kb.ID})
			}
		}
	}

	if len(scopes) == 0 {
		return nil, false, nil
	}
	return s.repo.SearchKnowledgeInScopes(ctx, scopes, keyword, offset, limit, fileTypes)
}
