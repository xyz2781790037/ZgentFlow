package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	goerrors "errors"

	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/application/repository"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/tracing/langfuse"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"github.com/xyz2781790037/ZealRAG/internal/utils"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// KnowledgeHandler processes HTTP requests related to knowledge resources
type KnowledgeHandler struct {
	kgService   interfaces.KnowledgeService
	kbService   interfaces.KnowledgeBaseService
	asynqClient interfaces.TaskEnqueuer
	spanRepo    repository.KnowledgeSpanRepository
}

// NewKnowledgeHandler creates a new knowledge handler instance
func NewKnowledgeHandler(
	kgService interfaces.KnowledgeService,
	kbService interfaces.KnowledgeBaseService,
	asynqClient interfaces.TaskEnqueuer,
	spanRepo repository.KnowledgeSpanRepository,
) *KnowledgeHandler {
	return &KnowledgeHandler{
		kgService:   kgService,
		kbService:   kbService,
		asynqClient: asynqClient,
		spanRepo:    spanRepo,
	}
}

// validateKnowledgeBaseAccess validates access permissions to a knowledge base
// using the ":id" URL path parameter. It delegates to validateKnowledgeBaseAccessWithKBID.
func (h *KnowledgeHandler) validateKnowledgeBaseAccess(c *gin.Context) (*types.KnowledgeBase, string, uint64, error) {
	kbID := secutils.SanitizeForLog(c.Param("id"))
	return h.validateKnowledgeBaseAccessWithKBID(c, kbID)
}

// validateKnowledgeBaseAccessWithKBID validates access to the given knowledge base ID (e.g. from query or body).
// Returns the knowledge base, kbID, workspace partition ID, and error.
func (h *KnowledgeHandler) validateKnowledgeBaseAccessWithKBID(c *gin.Context, kbID string) (*types.KnowledgeBase, string, uint64, error) {
	ctx := c.Request.Context()
	tenantID, _ := types.TenantIDFromContext(ctx)
	if tenantID == 0 {
		tenantID = c.GetUint64(types.TenantIDContextKey.String())
	}
	if tenantID == 0 {
		logger.Error(ctx, "Failed to get tenant ID")
		return nil, "", 0, errors.NewUnauthorizedError("Unauthorized")
	}
	kbID = secutils.SanitizeForLog(kbID)
	if kbID == "" {
		return nil, "", 0, errors.NewBadRequestError("Knowledge base ID cannot be empty")
	}
	kb, err := h.kbService.GetKnowledgeBaseByID(ctx, kbID)
	if err != nil {
		// Same not-found-vs-real-error split as knowledgebase.go's
		// validateAndGetKnowledgeBase: ErrKnowledgeBaseNotFound is the
		// expected outcome for a probed/stale kb id and must surface as
		// 404, not the generic 500 the original code produced.
		if goerrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, kbID, 0, errors.NewNotFoundError("knowledge base not found")
		}
		logger.ErrorWithFields(ctx, err, nil)
		return nil, kbID, 0, errors.NewInternalServerError(err.Error())
	}
	if kb.TenantID != tenantID {
		logger.Warnf(ctx, "Permission denied to access KB %s, tenant ID: %d, KB tenant: %d", kbID, tenantID, kb.TenantID)
		return nil, kbID, 0, errors.NewForbiddenError("Permission denied to access this knowledge base")
	}
	return kb, kbID, tenantID, nil
}

// resolveKnowledgeAndValidateKBAccess resolves knowledge by ID in the current
// local workspace.
func (h *KnowledgeHandler) resolveKnowledgeAndValidateKBAccess(c *gin.Context, knowledgeID string) (*types.Knowledge, context.Context, error) {
	ctx := c.Request.Context()
	tenantID, _ := types.TenantIDFromContext(ctx)
	if tenantID == 0 {
		tenantID = c.GetUint64(types.TenantIDContextKey.String())
	}
	if tenantID == 0 {
		return nil, ctx, errors.NewUnauthorizedError("Unauthorized")
	}

	knowledge, err := h.kgService.GetKnowledgeByID(ctx, knowledgeID)
	if err != nil {
		return nil, ctx, errors.NewNotFoundError("Knowledge not found")
	}

	if knowledge.TenantID != tenantID {
		return nil, ctx, errors.NewForbiddenError("Permission denied to access this knowledge")
	}
	return knowledge, context.WithValue(ctx, types.TenantIDContextKey, tenantID), nil
}

// handleDuplicateKnowledgeError handles cases where duplicate knowledge is detected
// Returns true if the error was a duplicate error and was handled, false otherwise
func (h *KnowledgeHandler) handleDuplicateKnowledgeError(c *gin.Context,
	err error, knowledge *types.Knowledge, duplicateType string,
) bool {
	if dupErr, ok := err.(*types.DuplicateKnowledgeError); ok {
		ctx := c.Request.Context()
		logger.Warnf(ctx, "Detected duplicate %s: %s", duplicateType, secutils.SanitizeForLog(dupErr.Error()))
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": dupErr.Error(),
			"data":    knowledge, // knowledge contains the existing document
			"code":    fmt.Sprintf("duplicate_%s", duplicateType),
		})
		return true
	}
	return false
}

// enqueueKnowledgeListDelete enqueues an async batch-delete task for the
// given knowledge IDs and returns the asynq task ID.
func (h *KnowledgeHandler) enqueueKnowledgeListDelete(
	ctx context.Context, tenantID uint64, ids []string,
) (string, error) {
	payload := types.KnowledgeListDeletePayload{
		TenantID:     tenantID,
		KnowledgeIDs: ids,
	}
	langfuse.InjectTracing(ctx, &payload)
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(types.TypeKnowledgeListDelete, payloadBytes)
	info, err := h.asynqClient.Enqueue(task, asynq.Queue("low"), asynq.MaxRetry(3))
	if err != nil {
		return "", fmt.Errorf("enqueue task: %w", err)
	}
	return info.ID, nil
}

// CreateKnowledgeFromFile godoc
// @Summary      从文件创建知识
// @Description  上传文件并创建知识条目
// @Tags         知识管理
// @Accept       multipart/form-data
// @Produce      json
// @Param        id                path      string  true   "知识库ID"
// @Param        file              formData  file    true   "上传的文件"
// @Param        fileName          formData  string  false  "自定义文件名"
// @Param        metadata          formData  string  false  "元数据JSON"
// @Param        enable_multimodel formData  bool    false  "启用多模态处理"
// @Param        process_config    formData  string  false  "处理配置JSON（KnowledgeProcessOverrides）"
// @Success      200               {object}  map[string]interface{}  "创建的知识"
// @Failure      400               {object}  errors.AppError         "请求参数错误"
// @Failure      409               {object}  map[string]interface{}  "文件重复"
// @Router       /knowledge-bases/{id}/knowledge/file [post]
func (h *KnowledgeHandler) CreateKnowledgeFromFile(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start creating knowledge from file")

	// Validate access to the knowledge base (only owner or admin/editor can create)
	kb, kbID, effectiveTenantID, err := h.validateKnowledgeBaseAccess(c)
	if err != nil {
		c.Error(err)
		return
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, effectiveTenantID)

	// Get the uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		logger.Error(ctx, "File upload failed", err)
		c.Error(errors.NewBadRequestError("File upload failed").WithDetails(err.Error()))
		return
	}

	// Enforce the knowledge-base limit without exceeding the deployment hard cap.
	maxSizeMB := utils.GetMaxFileSizeMB()
	if kb.MaxFileSizeMB > 0 && kb.MaxFileSizeMB < maxSizeMB {
		maxSizeMB = kb.MaxFileSizeMB
	}
	maxSize := maxSizeMB * 1024 * 1024
	if file.Size > maxSize {
		logger.Error(ctx, "File size too large")
		c.Error(errors.NewBadRequestError(fmt.Sprintf("文件大小不能超过%dMB", maxSizeMB)))
		return
	}

	// Get custom filename if provided (for folder uploads with path)
	customFileName := c.PostForm("fileName")
	customFileName = secutils.SanitizeForLog(customFileName)
	displayFileName := file.Filename
	displayFileName = secutils.SanitizeForLog(displayFileName)
	if customFileName != "" {
		displayFileName = customFileName
		logger.Infof(ctx, "Using custom filename: %s (original: %s)", customFileName, displayFileName)
	}

	logger.Infof(ctx, "File upload successful, filename: %s, size: %.2f KB", displayFileName, float64(file.Size)/1024)
	logger.Infof(ctx, "Creating knowledge, knowledge base ID: %s, filename: %s", kbID, displayFileName)

	// Parse metadata if provided
	var metadata map[string]string
	metadataStr := c.PostForm("metadata")
	if metadataStr != "" {
		if err := json.Unmarshal([]byte(metadataStr), &metadata); err != nil {
			logger.Error(ctx, "Failed to parse metadata", err)
			c.Error(errors.NewBadRequestError("Invalid metadata format").WithDetails(err.Error()))
			return
		}
		logger.Infof(ctx, "Received file metadata: %s", secutils.SanitizeForLog(fmt.Sprintf("%v", metadata)))
	}

	enableMultimodelForm := c.PostForm("enable_multimodel")
	var enableMultimodel *bool
	if enableMultimodelForm != "" {
		parseBool, err := strconv.ParseBool(enableMultimodelForm)
		if err != nil {
			logger.Error(ctx, "Failed to parse enable_multimodel", err)
			c.Error(errors.NewBadRequestError("Invalid enable_multimodel format").WithDetails(err.Error()))
			return
		}
		enableMultimodel = &parseBool
	}

	var processOverrides *types.KnowledgeProcessOverrides
	if raw := c.PostForm("process_config"); raw != "" {
		processOverrides = &types.KnowledgeProcessOverrides{}
		if err := json.Unmarshal([]byte(raw), processOverrides); err != nil {
			logger.Error(ctx, "Failed to parse process_config", err)
			c.Error(errors.NewBadRequestError("Invalid process_config format").WithDetails(err.Error()))
			return
		}
	}
	if enableMultimodel != nil && (processOverrides == nil || processOverrides.EnableMultimodel == nil) {
		if processOverrides == nil {
			processOverrides = &types.KnowledgeProcessOverrides{EnableMultimodel: enableMultimodel}
		} else {
			processOverrides.EnableMultimodel = enableMultimodel
		}
	}
	// Shared members upload with the owner's KB configuration. Only the owner
	// may supply per-document parser/enrichment overrides.
	if role, _ := ctx.Value(types.KnowledgeBaseAccessRoleContextKey).(string); role != "" && role != types.KBRoleOwner {
		enableMultimodel = nil
		processOverrides = nil
	}

	// Create knowledge entry from the file
	knowledge, err := h.kgService.CreateKnowledgeFromFile(ctx, kbID, file, metadata, enableMultimodel, customFileName, processOverrides)
	// Check for duplicate knowledge error
	if err != nil {
		if h.handleDuplicateKnowledgeError(c, err, knowledge, "file") {
			return
		}
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(
		ctx,
		"Knowledge created successfully, ID: %s, title: %s",
		secutils.SanitizeForLog(knowledge.ID),
		secutils.SanitizeForLog(knowledge.Title),
	)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    knowledge,
	})
}

// GetKnowledge godoc
// @Summary      获取知识详情
// @Description  根据ID获取知识条目详情
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识ID"
// @Success      200  {object}  map[string]interface{}  "知识详情"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "知识不存在"
// @Router       /knowledge/{id} [get]
func (h *KnowledgeHandler) GetKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start retrieving knowledge")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	// Resolve knowledge and validate KB access (at least viewer)
	knowledge, _, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	logger.Infof(ctx, "Knowledge retrieved successfully, ID: %s, title: %s",
		secutils.SanitizeForLog(knowledge.ID), secutils.SanitizeForLog(knowledge.Title))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    knowledge,
	})
}

// GetKnowledgeSpans godoc
// @Summary      获取知识文档解析的 Span 树（含历史尝试）
// @Description  返回该知识在解析流水线的 trace tree（root → stage → subspan）：每段状态、耗时、input/output、错误码、langfuse_trace_id。支持 ?attempt=N 查看历史尝试；不传则返回最新尝试。前端用于渲染时间线 + 多模态/embedding 子节点 + 一键跳转 Langfuse。
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id        path   string  true   "知识ID"
// @Param        attempt   query  int     false  "指定尝试号；省略=最新"
// @Success      200       {object}  map[string]interface{}
// @Router       /api/v1/knowledge/{id}/spans [get]
//
// Always returns the canonical 5-stage timeline; missing stage rows are
// synthesized as "pending" so the frontend timeline always renders five
// segments. Subspans (multimodal.image[i], generation.*) ride along under
// each stage as children when present.
func (h *KnowledgeHandler) GetKnowledgeSpans(c *gin.Context) {
	ctx := c.Request.Context()

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	knowledge, _, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	// Pick attempt: explicit ?attempt=N wins; otherwise pull the
	// latest attempt from the spans table. Lite-mode / fresh installs
	// with zero rows fall through to attempt=0, in which case we
	// return a placeholder tree (5 pending stages, no root, no
	// children) so the UI still renders.
	requestedAttempt := 0
	if v := strings.TrimSpace(c.Query("attempt")); v != "" {
		if n, perr := strconv.Atoi(v); perr == nil && n > 0 {
			requestedAttempt = n
		}
	}

	rows := []types.KnowledgeProcessingSpan{}
	currentAttempt := 0
	if h.spanRepo != nil {
		if requestedAttempt == 0 {
			latest, lerr := h.spanRepo.LatestAttempt(ctx, knowledge.ID)
			if lerr != nil {
				logger.Warnf(ctx, "spans LatestAttempt failed for %s: %v", knowledge.ID, lerr)
			} else {
				currentAttempt = latest
			}
		} else {
			currentAttempt = requestedAttempt
		}
		if currentAttempt > 0 {
			rows, err = h.spanRepo.ListByAttempt(ctx, knowledge.ID, currentAttempt)
			if err != nil {
				logger.Warnf(ctx, "spans ListByAttempt failed kid=%s attempt=%d: %v",
					knowledge.ID, currentAttempt, err)
				rows = nil
			}
		}
	}

	// Build tree: index by SpanID, then attach to parents. Stages
	// missing from the DB are synthesized as "pending" placeholders
	// under a synthetic (or real, if present) root so the timeline
	// always renders five segments. parse_status threads through so
	// pre-tracker historical knowledge (no rows but parse_status is
	// already terminal) renders as done/failed instead of pending —
	// otherwise legacy completed documents would forever look like
	// they're still waiting in the queue.
	tree, currentStageName, lastErr := buildSpanTree(knowledge.ID, currentAttempt, rows, knowledge.ParseStatus)

	resp := gin.H{
		"knowledge_id":    knowledge.ID,
		"parse_status":    knowledge.ParseStatus,
		"current_attempt": currentAttempt,
		"current_stage":   currentStageName,
		"trace":           tree,
	}
	if lastErr != nil {
		resp["last_error"] = gin.H{
			"stage":       lastErr.Name,
			"code":        lastErr.ErrorCode,
			"message":     lastErr.ErrorMessage,
			"finished_at": lastErr.FinishedAt,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    resp,
	})
}

// buildSpanTree assembles a flat list of span rows into a parent-child
// tree rooted at the (knowledge, attempt)'s root span. Missing canonical
// stages are filled in with pending placeholders so the UI always renders
// the five timeline segments. Returns the root, the current_stage name
// (the running stage if any), and the most recent failed span if one
// exists.
//
// parseStatus is the knowledge.parse_status string. When the spans table
// has zero rows for this attempt (legacy data parsed before tracking, or
// a fresh knowledge before the pipeline starts), the placeholder status
// is inferred from parseStatus: completed → done, failed → failed,
// otherwise pending. Without this, every historical knowledge would
// render as "all 5 stages pending" forever despite having actually
// completed parsing.
func buildSpanTree(knowledgeID string, attempt int, rows []types.KnowledgeProcessingSpan, parseStatus string) (
	root *types.SpanTreeNode, currentStage string, lastFailure *types.KnowledgeProcessingSpan,
) {
	now := time.Now()
	// Build node lookup, identify root.
	nodes := make(map[string]*types.SpanTreeNode, len(rows))
	var rootRow *types.KnowledgeProcessingSpan
	stageRowByName := map[string]*types.KnowledgeProcessingSpan{}
	for i := range rows {
		r := rows[i]
		nodes[r.SpanID] = &types.SpanTreeNode{KnowledgeProcessingSpan: r}
		if r.Kind == types.SpanKindRoot && rootRow == nil {
			cp := r
			rootRow = &cp
		}
		if r.Kind == types.SpanKindStage {
			cp := r
			stageRowByName[r.Name] = &cp
		}
		if r.Status == types.SpanStatusRunning && r.Kind == types.SpanKindStage && currentStage == "" {
			currentStage = r.Name
		}
		if r.Status == types.SpanStatusFailed {
			cp := r
			lastFailure = &cp
		}
	}

	// Pick the synthesized stage status from parse_status. Without this,
	// historical knowledge that completed before span tracking was wired
	// would render as "5 pending stages" forever — the rows simply
	// weren't recorded, but parse_status correctly reads "completed".
	// The synthesized stages don't carry duration/timing data; they
	// just communicate the inferred terminal state.
	syntheticStatus := types.SpanStatusPending
	switch parseStatus {
	case types.ParseStatusCompleted:
		syntheticStatus = types.SpanStatusDone
	case types.ParseStatusFailed:
		syntheticStatus = types.SpanStatusFailed
	}

	// Synthesize root if no rows came back so the API contract stays
	// stable (frontend always expects a `trace` object).
	if rootRow == nil {
		root = &types.SpanTreeNode{KnowledgeProcessingSpan: types.KnowledgeProcessingSpan{
			KnowledgeID: knowledgeID,
			Attempt:     attempt,
			SpanID:      "",
			Name:        "knowledge_processing",
			Kind:        types.SpanKindRoot,
			Status:      syntheticStatus,
			CreatedAt:   now,
			UpdatedAt:   now,
		}}
	} else {
		root = nodes[rootRow.SpanID]
	}

	// Link real children to their parents. Walk `rows` (not the
	// `nodes` map!) so the append order matches the repo's stable
	// `ORDER BY id ASC`. Iterating the map directly would give
	// callers a different child ordering on every request — Go map
	// iteration is intentionally randomised — and the UI would
	// flicker subspans into a different order on each refresh.
	for i := range rows {
		r := &rows[i]
		n := nodes[r.SpanID]
		if n == nil || n == root {
			continue
		}
		if r.ParentSpanID == "" {
			// Real top-level row with no parent and not the root
			// itself — attach to root so it doesn't dangle.
			root.Children = append(root.Children, n)
			continue
		}
		parent, ok := nodes[r.ParentSpanID]
		if !ok {
			// Unknown parent (orphan); attach to root.
			root.Children = append(root.Children, n)
			continue
		}
		parent.Children = append(parent.Children, n)
	}

	// Synthesize missing stage rows as children of root so the timeline
	// always shows 5 segments. Status mirrors the synthesized root —
	// pending while the pipeline is still running, done/failed for
	// historical knowledge whose terminal state we know but whose
	// per-stage timing was never recorded. Appended in AllStages order
	// so the canonical stage layout is deterministic regardless of
	// which rows are missing.
	for _, name := range types.AllStages {
		if _, ok := stageRowByName[name]; ok {
			continue
		}
		placeholder := types.KnowledgeProcessingSpan{
			KnowledgeID: knowledgeID,
			Attempt:     attempt,
			Name:        name,
			Kind:        types.SpanKindStage,
			Status:      syntheticStatus,
			CreatedAt:   now,
			UpdatedAt:   now,
		}
		root.Children = append(root.Children, &types.SpanTreeNode{KnowledgeProcessingSpan: placeholder})
	}

	return root, currentStage, lastFailure
}

// ListKnowledge godoc
// @Summary      获取知识列表
// @Description  获取知识库下的知识列表，支持分页和筛选
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id         path      string  true   "知识库ID"
// @Param        page       query     int     false  "页码"
// @Param        page_size  query     int     false  "每页数量"
// @Param        keyword       query     string  false  "关键词搜索"
// @Param        file_type     query     string  false  "文件类型筛选"
// @Param        parse_status  query     string  false  "解析状态筛选 (pending/processing/completed/failed)"
// @Param        source        query     string  false  "来源/渠道筛选 (web/api/feishu/notion/yuque/wechat/...，或 manual/url 按 type 过滤)"
// @Param        start_time    query     string  false  "更新时间起点，RFC3339 格式"
// @Param        end_time      query     string  false  "更新时间终点，RFC3339 格式"
// @Success      200        {object}  map[string]interface{}  "知识列表"
// @Failure      400        {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge-bases/{id}/knowledge [get]
func (h *KnowledgeHandler) ListKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start retrieving knowledge list")

	// Validate access to the knowledge base (read access - any permission level)
	_, kbID, effectiveTenantID, err := h.validateKnowledgeBaseAccess(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Update context with effective tenant ID for shared KB access
	ctx = context.WithValue(ctx, types.TenantIDContextKey, effectiveTenantID)

	// Parse pagination parameters from query string
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		logger.Error(ctx, "Failed to parse pagination parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	filter := types.KnowledgeListFilter{
		Keyword:     c.Query("keyword"),
		FileType:    c.Query("file_type"),
		ParseStatus: c.Query("parse_status"),
	}
	if raw := c.Query("start_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid start_time: " + err.Error()))
			return
		}
		filter.UpdatedFrom = t
	}
	if raw := c.Query("end_time"); raw != "" {
		t, err := parseFilterTime(raw)
		if err != nil {
			c.Error(errors.NewBadRequestError("invalid end_time: " + err.Error()))
			return
		}
		filter.UpdatedTo = t
	}

	logger.Infof(
		ctx,
		"Retrieving knowledge list under knowledge base, kb_id=%s keyword=%s file_type=%s parse_status=%s start_time=%s end_time=%s page=%d page_size=%d effectiveTenantID=%d",
		secutils.SanitizeForLog(kbID),
		secutils.SanitizeForLog(filter.Keyword),
		secutils.SanitizeForLog(filter.FileType),
		secutils.SanitizeForLog(filter.ParseStatus),
		secutils.SanitizeForLog(c.Query("start_time")),
		secutils.SanitizeForLog(c.Query("end_time")),
		pagination.Page,
		pagination.PageSize,
		effectiveTenantID,
	)

	// Retrieve paginated knowledge entries
	result, err := h.kgService.ListPagedKnowledgeByKnowledgeBaseID(ctx, kbID, &pagination, filter)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(
		ctx,
		"Knowledge list retrieved successfully, knowledge base ID: %s, total: %d",
		secutils.SanitizeForLog(kbID),
		result.Total,
	)
	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result.Data,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// DeleteKnowledge godoc
// @Summary      删除知识
// @Description  根据ID异步删除知识条目。请求会被入队到与批量删除相同的异步管道（asynq）；
// @Description  接口返回 200 仅表示任务已提交（响应 data.task_id 为任务 ID），实际删除由后台 worker 完成。
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识ID"
// @Success      200  {object}  map[string]interface{}  "任务已提交，返回 task_id"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge/{id} [delete]
func (h *KnowledgeHandler) DeleteKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start deleting knowledge")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	if err := h.kgService.TrashKnowledge(effCtx, id); err != nil {
		logger.Errorf(ctx, "Failed to move knowledge to recycle bin: %v", err)
		c.Error(errors.NewInternalServerError("Failed to move document to recycle bin"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Document moved to recycle bin",
	})
}

// BatchDeleteKnowledgeRequest is the body schema for POST /knowledge/batch-delete.
type BatchDeleteKnowledgeRequest struct {
	KBID string   `json:"kb_id" binding:"required"`
	IDs  []string `json:"ids"  binding:"required"`
}

// BatchDeleteKnowledge godoc
// @Summary      批量删除知识
// @Description  按 ID 列表批量删除单个知识库下的多个知识条目
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        request  body      BatchDeleteKnowledgeRequest  true  "批量删除请求"
// @Success      200      {object}  map[string]interface{}       "删除成功"
// @Failure      400      {object}  errors.AppError              "请求参数错误"
// @Failure      403      {object}  errors.AppError              "权限不足"
// @Router       /knowledge/batch-delete [post]
func (h *KnowledgeHandler) BatchDeleteKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	var req BatchDeleteKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(errors.NewBadRequestError("Invalid request parameters: " + err.Error()))
		return
	}

	// Deduplicate and drop empty IDs.
	seen := make(map[string]struct{}, len(req.IDs))
	ids := make([]string, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id := strings.TrimSpace(raw)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		c.Error(errors.NewBadRequestError("ids cannot be empty"))
		return
	}
	const maxBatch = 200
	if len(ids) > maxBatch {
		c.Error(errors.NewBadRequestError(fmt.Sprintf("too many ids (max %d per batch)", maxBatch)))
		return
	}

	// Validate KB access (editor or admin) using the kb_id from body.
	_, kbID, effectiveTenantID, err := h.validateKnowledgeBaseAccessWithKBID(c, req.KBID)
	if err != nil {
		c.Error(err)
		return
	}
	ctx = context.WithValue(ctx, types.TenantIDContextKey, effectiveTenantID)

	// Single batch fetch to validate that every id exists and belongs to the
	// requested KB. The service-layer DeleteKnowledgeList only enforces tenant
	// scope, not KB scope, so the handler must guard against cross-KB deletion.
	knowledgeList, err := h.kgService.GetKnowledgeBatch(ctx, effectiveTenantID, ids)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if len(knowledgeList) != len(ids) {
		c.Error(errors.NewBadRequestError("One or more knowledge entries not found"))
		return
	}
	for _, k := range knowledgeList {
		if k.KnowledgeBaseID != kbID {
			c.Error(errors.NewBadRequestError(
				fmt.Sprintf("Knowledge %s does not belong to knowledge base %s",
					secutils.SanitizeForLog(k.ID), secutils.SanitizeForLog(kbID))))
			return
		}
	}

	if err := h.kgService.TrashKnowledgeList(ctx, ids); err != nil {
		logger.Errorf(ctx, "Failed to move documents to recycle bin: %v", err)
		c.Error(errors.NewInternalServerError("Failed to move documents to recycle bin"))
		return
	}

	logger.Infof(ctx, "Moved documents to recycle bin: kb_id=%s count=%d",
		secutils.SanitizeForLog(kbID), len(ids))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Documents moved to recycle bin",
		"data": gin.H{
			"deleted_count": len(ids),
		},
	})
}

// ClearKnowledgeBaseContents godoc
// @Summary      清空知识库内容
// @Description  删除知识库下的所有知识条目（异步任务）。知识库本身保留，仅清空其中的内容
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识库ID"
// @Success      200  {object}  map[string]interface{}  "清空任务已提交"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      403  {object}  errors.AppError         "权限不足"
// @Router       /knowledge-bases/{id}/knowledge [delete]
func (h *KnowledgeHandler) ClearKnowledgeBaseContents(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start clearing knowledge base contents")

	_, kbID, effectiveTenantID, err := h.validateKnowledgeBaseAccess(c)
	if err != nil {
		c.Error(err)
		return
	}

	ctx = context.WithValue(ctx, types.TenantIDContextKey, effectiveTenantID)

	knowledgeList, err := h.kgService.ListKnowledgeByKnowledgeBaseID(ctx, kbID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to list knowledge entries").WithDetails(err.Error()))
		return
	}

	if len(knowledgeList) == 0 {
		logger.Infof(ctx, "Knowledge base %s is already empty", secutils.SanitizeForLog(kbID))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Knowledge base is already empty",
			"data":    gin.H{"deleted_count": 0},
		})
		return
	}

	knowledgeIDs := make([]string, 0, len(knowledgeList))
	for _, knowledge := range knowledgeList {
		knowledgeIDs = append(knowledgeIDs, knowledge.ID)
	}

	if err := h.kgService.TrashKnowledgeList(ctx, knowledgeIDs); err != nil {
		logger.Errorf(ctx, "Failed to move knowledge list to recycle bin: %v", err)
		c.Error(errors.NewInternalServerError("Failed to move documents to recycle bin"))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Documents moved to recycle bin",
		"data":    gin.H{"deleted_count": len(knowledgeIDs)},
	})
}

func (h *KnowledgeHandler) ListTrashedKnowledge(c *gin.Context) {
	rows, err := h.kgService.ListTrashedKnowledge(c.Request.Context())
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *KnowledgeHandler) RestoreTrashedKnowledge(c *gin.Context) {
	if err := h.kgService.RestoreKnowledge(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeHandler) PurgeTrashedKnowledge(c *gin.Context) {
	if err := h.kgService.PurgeTrashedKnowledge(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DownloadKnowledgeFile godoc
// @Summary      下载知识文件
// @Description  下载知识条目关联的原始文件
// @Tags         知识管理
// @Accept       json
// @Produce      application/octet-stream
// @Param        id   path      string  true  "知识ID"
// @Success      200  {file}    file    "文件内容"
// @Failure      400  {object}  errors.AppError  "请求参数错误"
// @Router       /knowledge/{id}/download [get]
func (h *KnowledgeHandler) DownloadKnowledgeFile(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start downloading knowledge file")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}
	logger.Infof(ctx, "Retrieving knowledge file, ID: %s", secutils.SanitizeForLog(id))

	file, filename, err := h.kgService.GetKnowledgeFile(effCtx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to retrieve file").WithDetails(err.Error()))
		return
	}
	defer file.Close()

	logger.Infof(
		ctx,
		"Knowledge file retrieved successfully, ID: %s, filename: %s",
		secutils.SanitizeForLog(id),
		secutils.SanitizeForLog(filename),
	)

	// Set response headers for file download
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	cd := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	c.Header("Content-Disposition", cd)
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Expires", "0")
	c.Header("Cache-Control", "must-revalidate")
	c.Header("Pragma", "public")

	// Stream file content to response
	c.Stream(func(w io.Writer) bool {
		if _, err := io.Copy(w, file); err != nil {
			logger.Errorf(ctx, "Failed to send file: %v", err)
			return false
		}
		logger.Debug(ctx, "File sending completed")
		return false
	})
}

// mimeTypeByExt returns the MIME type for a given file extension.
func mimeTypeByExt(filename string) string {
	ext := strings.ToLower(filename)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	} else {
		ext = ""
	}
	m := map[string]string{
		".pdf":      "application/pdf",
		".docx":     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		".doc":      "application/msword",
		".pptx":     "application/vnd.openxmlformats-officedocument.presentationml.presentation",
		".ppt":      "application/vnd.ms-powerpoint",
		".xlsx":     "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		".xls":      "application/vnd.ms-excel",
		".csv":      "text/csv",
		".jpg":      "image/jpeg",
		".jpeg":     "image/jpeg",
		".png":      "image/png",
		".gif":      "image/gif",
		".bmp":      "image/bmp",
		".webp":     "image/webp",
		".svg":      "image/svg+xml",
		".tiff":     "image/tiff",
		".txt":      "text/plain; charset=utf-8",
		".md":       "text/markdown; charset=utf-8",
		".markdown": "text/markdown; charset=utf-8",
		".json":     "application/json; charset=utf-8",
		".xml":      "application/xml; charset=utf-8",
		".html":     "text/html; charset=utf-8",
		".css":      "text/css; charset=utf-8",
		".js":       "text/javascript; charset=utf-8",
		".ts":       "text/typescript; charset=utf-8",
		".py":       "text/x-python; charset=utf-8",
		".go":       "text/x-go; charset=utf-8",
		".java":     "text/x-java; charset=utf-8",
		".yaml":     "text/yaml; charset=utf-8",
		".yml":      "text/yaml; charset=utf-8",
		".sh":       "text/x-shellscript; charset=utf-8",
	}
	if ct, ok := m[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}

// PreviewKnowledgeFile godoc
// @Summary      预览知识文件
// @Description  返回知识条目关联的原始文件，Content-Type 根据文件类型设置，用于浏览器内嵌预览
// @Tags         知识管理
// @Accept       json
// @Produce      application/pdf,image/jpeg,image/png,text/plain
// @Param        id   path      string  true  "知识ID"
// @Success      200  {file}    file    "文件内容"
// @Failure      400  {object}  errors.AppError  "请求参数错误"
// @Router       /knowledge/{id}/preview [get]
func (h *KnowledgeHandler) PreviewKnowledgeFile(c *gin.Context) {
	ctx := c.Request.Context()

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	file, filename, err := h.kgService.GetKnowledgeFile(effCtx, id)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to retrieve file").WithDetails(err.Error()))
		return
	}
	defer file.Close()

	contentType := mimeTypeByExt(filename)
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	c.Header("Cache-Control", "private, max-age=3600")

	c.Stream(func(w io.Writer) bool {
		if _, err := io.Copy(w, file); err != nil {
			logger.Errorf(ctx, "Failed to stream preview: %v", err)
			return false
		}
		return false
	})
}

// GetKnowledgeBatchRequest defines parameters for batch knowledge retrieval
type GetKnowledgeBatchRequest struct {
	IDs  []string `form:"ids" binding:"required"` // List of knowledge IDs
	KBID string   `form:"kb_id"`                  // Optional: scope to this KB
}

// GetKnowledgeBatch godoc
// @Summary      批量获取知识
// @Description  根据ID列表批量获取知识条目。可选 kb_id 用于限定知识库范围。
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        ids       query     []string  true   "知识ID列表"
// @Param        kb_id     query     string   false  "可选，知识库ID"
// @Success      200       {object}  map[string]interface{}  "知识列表"
// @Failure      400       {object}  errors.AppError        "请求参数错误"
// @Router       /knowledge/batch [get]
func (h *KnowledgeHandler) GetKnowledgeBatch(c *gin.Context) {
	ctx := c.Request.Context()

	tenantID, ok := c.Get(types.TenantIDContextKey.String())
	if !ok {
		logger.Error(ctx, "Failed to get tenant ID")
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}
	effectiveTenantID := tenantID.(uint64)

	var req GetKnowledgeBatchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	var knowledges []*types.Knowledge
	var err error

	// scopeKBID tracks the single KB the results must belong to (set by explicit kb_id).
	var scopeKBID string

	// Optional kb_id: validate local workspace access and narrow the result set.
	if kbID := secutils.SanitizeForLog(req.KBID); kbID != "" {
		_, _, effID, err := h.validateKnowledgeBaseAccessWithKBID(c, kbID)
		if err != nil {
			c.Error(err)
			return
		}
		scopeKBID = kbID
		effectiveTenantID = effID
		ctx = context.WithValue(ctx, types.TenantIDContextKey, effectiveTenantID)

		logger.Infof(ctx, "Batch retrieving knowledge with kb_id, effective tenant ID: %d, IDs count: %d",
			effectiveTenantID, len(req.IDs))

		knowledges, err = h.kgService.GetKnowledgeBatch(ctx, effectiveTenantID, req.IDs)
	} else {
		logger.Infof(ctx, "Batch retrieving knowledge without kb_id, effective tenant ID: %d, IDs count: %d",
			effectiveTenantID, len(req.IDs))

		knowledges, err = h.kgService.GetKnowledgeBatch(ctx, effectiveTenantID, req.IDs)
	}

	// Scope an explicit kb_id to a single knowledge base.
	var allowedKBSet map[string]bool
	if scopeKBID != "" {
		allowedKBSet = map[string]bool{scopeKBID: true}
	}
	if allowedKBSet != nil && len(knowledges) > 0 {
		filtered := make([]*types.Knowledge, 0, len(knowledges))
		for _, k := range knowledges {
			if allowedKBSet[k.KnowledgeBaseID] {
				filtered = append(filtered, k)
			}
		}
		knowledges = filtered
	}

	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to retrieve knowledge list").WithDetails(err.Error()))
		return
	}

	logger.Infof(ctx, "Batch knowledge retrieval successful, requested count: %d, returned count: %d",
		len(req.IDs), len(knowledges))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    knowledges,
	})
}

// UpdateKnowledge godoc
// @Summary      更新知识
// @Description  更新知识条目信息
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id       path      string          true  "知识ID"
// @Param        request  body      types.Knowledge true  "知识信息"
// @Success      200      {object}  map[string]interface{}  "更新成功"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge/{id} [put]
func (h *KnowledgeHandler) UpdateKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	var knowledge types.Knowledge
	if err := c.ShouldBindJSON(&knowledge); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	knowledge.ID = id

	if err := h.kgService.UpdateKnowledge(effCtx, &knowledge); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge updated successfully, knowledge ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge chunk updated successfully",
	})
}

// ReparseKnowledge godoc
// @Summary      重新解析知识
// @Description  删除知识中现有的文档内容并重新解析，使用异步任务方式处理
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识ID"
// @Param        body body      object  false  "可选的处理配置覆盖：{\"process_config\": KnowledgeProcessOverrides}"
// @Success      200  {object}  map[string]interface{}  "重新解析任务已提交"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      403  {object}  errors.AppError         "权限不足"
// @Router       /knowledge/{id}/reparse [post]
func (h *KnowledgeHandler) ReparseKnowledge(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start re-parsing knowledge")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	// Validate KB access with editor permission (reparse requires write access)
	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	// Optional per-reparse parse config override. Empty body keeps the
	// overrides stored at upload time.
	var processOverrides *types.KnowledgeProcessOverrides
	if c.Request.ContentLength != 0 {
		var req struct {
			ProcessConfig *types.KnowledgeProcessOverrides `json:"process_config"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			logger.Error(ctx, "Failed to parse reparse request body", err)
			c.Error(errors.NewBadRequestError("Invalid reparse request body").WithDetails(err.Error()))
			return
		}
		processOverrides = req.ProcessConfig
	}
	if role, _ := ctx.Value(types.KnowledgeBaseAccessRoleContextKey).(string); role != "" && role != types.KBRoleOwner {
		processOverrides = nil
	}

	// Call service to reparse knowledge
	knowledge, err := h.kgService.ReparseKnowledge(effCtx, id, processOverrides)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_id": id,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge reparse task submitted successfully, knowledge ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge reparse task submitted",
		"data":    knowledge,
	})
}

// CancelKnowledgeParse godoc
// @Summary      取消知识解析
// @Description  取消进行中的知识解析任务。当前已写入的 chunk / 索引保留，可通过 reparse 接口重新触发解析。已完成 / 已失败 / 删除中的知识不支持取消。
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识ID"
// @Success      200  {object}  map[string]interface{}  "取消已提交"
// @Failure      400  {object}  errors.AppError         "状态不支持取消"
// @Failure      403  {object}  errors.AppError         "权限不足"
// @Failure      404  {object}  errors.AppError         "知识不存在"
// @Router       /knowledge/{id}/cancel-parse [post]
func (h *KnowledgeHandler) CancelKnowledgeParse(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start cancelling knowledge parse")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	// Editor permission — same gate as ReparseKnowledge / DeleteKnowledge.
	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	knowledge, err := h.kgService.CancelKnowledgeParse(effCtx, id)
	if err != nil {
		if appErr, ok := errors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, map[string]interface{}{
			"knowledge_id": id,
		})
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge parse cancelled successfully, knowledge ID: %s", id)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge parse cancelled",
		"data":    knowledge,
	})
}

// UpdateImageInfo godoc
// @Summary      更新图像信息
// @Description  更新知识分块的图像信息
// @Tags         知识管理
// @Accept       json
// @Produce      json
// @Param        id        path      string  true  "知识ID"
// @Param        chunk_id  path      string  true  "分块ID"
// @Param        request   body      object{image_info=string}  true  "图像信息"
// @Success      200       {object}  map[string]interface{}     "更新成功"
// @Failure      400       {object}  errors.AppError            "请求参数错误"
// @Router       /knowledge/image/{id}/{chunk_id} [put]
func (h *KnowledgeHandler) UpdateImageInfo(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start updating image info")

	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}
	chunkID := secutils.SanitizeForLog(c.Param("chunk_id"))
	if chunkID == "" {
		logger.Error(ctx, "Chunk ID is empty")
		c.Error(errors.NewBadRequestError("Chunk ID cannot be empty"))
		return
	}

	_, effCtx, err := h.resolveKnowledgeAndValidateKBAccess(c, id)
	if err != nil {
		c.Error(err)
		return
	}

	var request struct {
		ImageInfo string `json:"image_info"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	logger.Infof(ctx, "Updating knowledge chunk, knowledge ID: %s, chunk ID: %s", id, chunkID)
	err = h.kgService.UpdateImageInfo(effCtx, id, chunkID, secutils.SanitizeForLog(request.ImageInfo))
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge chunk updated successfully, knowledge ID: %s, chunk ID: %s", id, chunkID)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge chunk image updated successfully",
	})
}

// SearchKnowledge godoc
// @Summary      Search knowledge
// @Description  Search knowledge files by keyword in the current workspace.
// @Tags         Knowledge
// @Accept       json
// @Produce      json
// @Param        keyword    query     string  false "Keyword to search"
// @Param        offset     query     int     false "Offset for pagination"
// @Param        limit      query     int     false "Limit for pagination (default 20)"
// @Param        file_types query     string  false "Comma-separated file extensions to filter (e.g., csv,xlsx)"
// @Success      200         {object}  map[string]interface{}     "Search results"
// @Failure      400         {object}  errors.AppError            "Invalid request"
// @Router       /knowledge/search [get]
func (h *KnowledgeHandler) SearchKnowledge(c *gin.Context) {
	ctx := c.Request.Context()
	if userID, ok := c.Get(types.UserIDContextKey.String()); ok {
		ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	}
	// Accept both ?keyword= (legacy / upstream name) and ?query= (what most
	// agent integrations send). Falling silently back to an unsorted
	// listing when both are empty caused the "all queries return the same 5
	// newest cards" footgun — return 400 instead so the caller gets a clear
	// signal that the search wasn't query-driven.
	keyword := c.Query("keyword")
	if keyword == "" {
		keyword = c.Query("query")
	}
	if strings.TrimSpace(keyword) == "" {
		c.Error(errors.NewBadRequestError("missing search keyword: pass ?keyword=... or ?query=..."))
		return
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	var fileTypes []string
	if fileTypesStr := c.Query("file_types"); fileTypesStr != "" {
		for _, ft := range strings.Split(fileTypesStr, ",") {
			ft = strings.TrimSpace(ft)
			if ft != "" {
				fileTypes = append(fileTypes, ft)
			}
		}
	}

	knowledges, hasMore, err := h.kgService.SearchKnowledge(ctx, keyword, offset, limit, fileTypes)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError("Failed to search knowledge").WithDetails(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":  true,
		"data":     knowledges,
		"has_more": hasMore,
	})
}

// MoveKnowledgeRequest defines the request for moving knowledge items
type MoveKnowledgeRequest struct {
	KnowledgeIDs []string `json:"knowledge_ids" binding:"required,min=1"`
	SourceKBID   string   `json:"source_kb_id"  binding:"required"`
	TargetKBID   string   `json:"target_kb_id"  binding:"required"`
	Mode         string   `json:"mode"          binding:"required,oneof=reuse_vectors reparse"`
}

// MoveKnowledgeResponse defines the response for move knowledge
type MoveKnowledgeResponse struct {
	TaskID         string `json:"task_id"`
	SourceKBID     string `json:"source_kb_id"`
	TargetKBID     string `json:"target_kb_id"`
	KnowledgeCount int    `json:"knowledge_count"`
	Message        string `json:"message"`
}

// MoveKnowledge moves knowledge items from one knowledge base to another (async task).
//
// MoveKnowledge godoc
// @Summary      移动知识到其他知识库
// @Description  将一条或多条知识从源知识库移动到目标知识库（异步），返回任务 ID 用于查询进度
// @Tags         知识
// @Accept       json
// @Produce      json
// @Param        request  body      handler.MoveKnowledgeRequest  true  "{source_kb_id, target_kb_id, knowledge_ids}"
// @Success      200      {object}  handler.MoveKnowledgeResponse  "任务信息"
// @Failure      400      {object}  errors.AppError                "请求参数错误"
// @Router       /knowledge/move [post]
func (h *KnowledgeHandler) MoveKnowledge(c *gin.Context) {
	ctx := c.Request.Context()

	var req MoveKnowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "MoveKnowledge: failed to parse request", err)
		c.Error(errors.NewBadRequestError("Invalid request parameters: " + err.Error()))
		return
	}

	// Validate source != target
	if req.SourceKBID == req.TargetKBID {
		c.Error(errors.NewBadRequestError("Source and target knowledge base cannot be the same"))
		return
	}

	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		c.Error(errors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Validate source KB
	sourceKB, err := h.kbService.GetKnowledgeBaseByID(ctx, req.SourceKBID)
	if err != nil {
		if goerrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != tenantID.(uint64) {
		c.Error(errors.NewForbiddenError("No permission to access source knowledge base"))
		return
	}

	// Validate target KB
	targetKB, err := h.kbService.GetKnowledgeBaseByID(ctx, req.TargetKBID)
	if err != nil {
		if goerrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Target knowledge base not found"))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if targetKB.TenantID != tenantID.(uint64) {
		c.Error(errors.NewForbiddenError("No permission to access target knowledge base"))
		return
	}

	// Validate type match
	if sourceKB.Type != targetKB.Type {
		c.Error(errors.NewBadRequestError("Source and target knowledge bases must be the same type"))
		return
	}

	// Validate embedding model match
	if sourceKB.EmbeddingModelID != targetKB.EmbeddingModelID {
		c.Error(errors.NewBadRequestError("Source and target must use the same embedding model"))
		return
	}

	// Validate all knowledge IDs belong to source KB and are in completed status
	for _, kID := range req.KnowledgeIDs {
		knowledge, err := h.kgService.GetKnowledgeByID(ctx, kID)
		if err != nil {
			c.Error(errors.NewBadRequestError(fmt.Sprintf("Knowledge item %s not found", kID)))
			return
		}
		if knowledge.KnowledgeBaseID != req.SourceKBID {
			c.Error(errors.NewBadRequestError(fmt.Sprintf("Knowledge item %s does not belong to the source knowledge base", kID)))
			return
		}
		if knowledge.ParseStatus != types.ParseStatusCompleted {
			c.Error(errors.NewBadRequestError(fmt.Sprintf("Knowledge item %s is not in completed status (current: %s)", kID, knowledge.ParseStatus)))
			return
		}
	}

	// Generate task ID
	taskID := utils.GenerateTaskID("kg_move", tenantID.(uint64), req.SourceKBID)

	// Create move payload
	payload := types.KnowledgeMovePayload{
		TenantID:     tenantID.(uint64),
		TaskID:       taskID,
		KnowledgeIDs: req.KnowledgeIDs,
		SourceKBID:   req.SourceKBID,
		TargetKBID:   req.TargetKBID,
		Mode:         req.Mode,
	}
	langfuse.InjectTracing(ctx, &payload)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		logger.Errorf(ctx, "MoveKnowledge: failed to marshal payload: %v", err)
		c.Error(errors.NewInternalServerError("Failed to create task"))
		return
	}

	// Enqueue move task
	task := asynq.NewTask(types.TypeKnowledgeMove, payloadBytes)
	info, err := h.asynqClient.Enqueue(task,
		asynq.TaskID(taskID), asynq.Queue("default"), asynq.MaxRetry(3))
	if err != nil {
		logger.Errorf(ctx, "MoveKnowledge: failed to enqueue task: %v", err)
		c.Error(errors.NewInternalServerError("Failed to enqueue task"))
		return
	}

	logger.Infof(ctx, "MoveKnowledge: task enqueued: %s, asynq_id: %s, source: %s, target: %s, count: %d",
		taskID, info.ID, secutils.SanitizeForLog(req.SourceKBID), secutils.SanitizeForLog(req.TargetKBID), len(req.KnowledgeIDs))

	// Save initial progress
	initialProgress := &types.KnowledgeMoveProgress{
		TaskID:     taskID,
		SourceKBID: req.SourceKBID,
		TargetKBID: req.TargetKBID,
		Status:     types.KnowledgeMoveStatusPending,
		Total:      len(req.KnowledgeIDs),
		Progress:   0,
		Message:    "Task queued, waiting to start...",
		CreatedAt:  time.Now().Unix(),
		UpdatedAt:  time.Now().Unix(),
	}
	if err := h.kgService.SaveKnowledgeMoveProgress(ctx, initialProgress); err != nil {
		logger.Warnf(ctx, "MoveKnowledge: failed to save initial progress: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": MoveKnowledgeResponse{
			TaskID:         taskID,
			SourceKBID:     req.SourceKBID,
			TargetKBID:     req.TargetKBID,
			KnowledgeCount: len(req.KnowledgeIDs),
			Message:        "Knowledge move task started",
		},
	})
}

// GetKnowledgeMoveProgress retrieves the progress of a knowledge move task.
//
// GetKnowledgeMoveProgress godoc
// @Summary      获取知识移动进度
// @Description  按任务 ID 查询移动进度
// @Tags         知识
// @Produce      json
// @Param        task_id  path      string                       true  "移动任务 ID"
// @Success      200      {object}  types.KnowledgeMoveProgress  "进度信息"
// @Failure      404      {object}  errors.AppError              "任务不存在"
// @Router       /knowledge/move/progress/{task_id} [get]
func (h *KnowledgeHandler) GetKnowledgeMoveProgress(c *gin.Context) {
	ctx := c.Request.Context()

	taskID := c.Param("task_id")
	if taskID == "" {
		c.Error(errors.NewBadRequestError("Task ID cannot be empty"))
		return
	}

	progress, err := h.kgService.GetKnowledgeMoveProgress(ctx, taskID)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    progress,
	})
}

// parseFilterTime parses a query-string timestamp accepted by knowledge list
// filters. It supports RFC3339, RFC3339 with milliseconds, and the date-only
// "2006-01-02" form (interpreted at start of day in the local timezone).
func parseFilterTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"}
	var lastErr error
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, time.Local); err == nil {
			return t, nil
		} else {
			lastErr = err
		}
	}
	return time.Time{}, lastErr
}
