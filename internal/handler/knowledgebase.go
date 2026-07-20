package handler

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/application/repository"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	"github.com/xyz2781790037/ZealRAG/internal/utils"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// KnowledgeBaseHandler defines the HTTP handler for knowledge base operations
type KnowledgeBaseHandler struct {
	service          interfaces.KnowledgeBaseService
	knowledgeService interfaces.KnowledgeService
	asynqClient      interfaces.TaskEnqueuer
	rebuildService   interfaces.KBFullRebuildService
	shareService     *service.KnowledgeBaseShareService
}

func validateKnowledgeBaseMaxFileSizeMB(value int64) error {
	max := utils.GetMaxFileSizeMB()
	if value < 1 || value > max {
		return apperrors.NewBadRequestError(
			fmt.Sprintf("单文件大小上限必须在 1 到 %d MB 之间", max),
		)
	}
	return nil
}

// NewKnowledgeBaseHandler creates a new knowledge base handler instance
func NewKnowledgeBaseHandler(
	service interfaces.KnowledgeBaseService,
	knowledgeService interfaces.KnowledgeService,
	asynqClient interfaces.TaskEnqueuer,
	rebuildService interfaces.KBFullRebuildService,
	shareService *service.KnowledgeBaseShareService,
) *KnowledgeBaseHandler {
	return &KnowledgeBaseHandler{
		service:          service,
		knowledgeService: knowledgeService,
		asynqClient:      asynqClient,
		rebuildService:   rebuildService,
		shareService:     shareService,
	}
}

// buildKBResponse merges caller-supplied fields into a knowledge-base response.
// Legacy vector-store bindings are intentionally hidden because ZealRAG always
// uses its built-in PostgreSQL/pgvector backend.
func buildKBResponse(
	kb *types.KnowledgeBase,
	extras map[string]interface{},
) interface{} {
	b, err := json.Marshal(kb)
	if err != nil {
		return kb
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil || m == nil {
		return kb
	}
	delete(m, "vector_store_id")
	delete(m, "tenant_id")
	delete(m, "owner_user_id")
	for k, v := range extras {
		m[k] = v
	}
	return m
}

// buildKBListResponse applies the same compatibility projection to a list.
func (h *KnowledgeBaseHandler) buildKBListResponse(
	_ context.Context, kbs []*types.KnowledgeBase, _ uint64,
) []interface{} {
	out := make([]interface{}, 0, len(kbs))
	for _, kb := range kbs {
		out = append(out, buildKBResponse(kb, nil))
	}
	return out
}

// HybridSearch godoc
// @Summary      混合搜索
// @Description  在知识库中执行向量和关键词混合搜索
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string             true  "知识库ID"
// @Param        request  body      types.SearchParams true  "搜索参数"
// @Success      200      {object}  map[string]interface{}  "搜索结果"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge-bases/{id}/hybrid-search [get]
func (h *KnowledgeBaseHandler) HybridSearch(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start hybrid search")

	// Validate and check permission for knowledge base access
	_, id, effectiveTenantID, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Parse request body
	var req types.SearchParams
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}

	logger.Infof(ctx, "Executing hybrid search, knowledge base ID: %s, query: %s, effectiveTenantID: %d",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.QueryText), effectiveTenantID)

	// Execute hybrid search with default search parameters
	// Note: For shared KBs, the service uses effectiveTenantID internally via context
	results, err := h.service.HybridSearch(ctx, id, req)
	if err != nil {
		// Preserve typed validation and authorization errors at the HTTP boundary.
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Hybrid search completed, knowledge base ID: %s, result count: %d",
		secutils.SanitizeForLog(id), len(results))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    results,
	})
}

// CreateKnowledgeBase godoc
// @Summary      创建知识库
// @Description  创建新的知识库
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        request  body      types.KnowledgeBase  true  "知识库信息"
// @Success      201      {object}  map[string]interface{}  "创建的知识库"
// @Failure      400      {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge-bases [post]
func (h *KnowledgeBaseHandler) CreateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Info(ctx, "Start creating knowledge base")

	// Parse request body
	var req types.KnowledgeBase
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	req.EnsureDefaults()
	if err := validateKnowledgeBaseMaxFileSizeMB(req.MaxFileSizeMB); err != nil {
		c.Error(err)
		return
	}
	logger.Infof(ctx, "Creating knowledge base, name: %s", secutils.SanitizeForLog(req.Name))
	// Create knowledge base using the service
	kb, err := h.service.CreateKnowledgeBase(ctx, &req)
	if err != nil {
		if appErr, ok := apperrors.IsAppError(err); ok {
			c.Error(appErr)
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base created successfully, ID: %s, name: %s",
		secutils.SanitizeForLog(kb.ID), secutils.SanitizeForLog(kb.Name))
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, nil),
	})
}

// validateAndGetKnowledgeBase validates the path and loads a knowledge base
// from the current local workspace.
func (h *KnowledgeBaseHandler) validateAndGetKnowledgeBase(c *gin.Context) (*types.KnowledgeBase, string, uint64, error) {
	ctx := c.Request.Context()
	tenantID, exists := types.TenantIDFromContext(ctx)
	if !exists || tenantID == 0 {
		tenantID = c.GetUint64(types.TenantIDContextKey.String())
		exists = tenantID != 0
	}
	if !exists || tenantID == 0 {
		logger.Error(ctx, "Failed to get tenant ID")
		return nil, "", 0, apperrors.NewUnauthorizedError("Unauthorized")
	}

	// Get knowledge base ID from URL parameter
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Knowledge base ID is empty")
		return nil, "", 0, apperrors.NewBadRequestError("Knowledge base ID cannot be empty")
	}

	kb, err := h.service.GetKnowledgeBaseByID(ctx, id)
	if err != nil {
		// repo.GetKnowledgeBaseByID surfaces ErrKnowledgeBaseNotFound for
		// missing or cross-tenant rows. Map it to 404 here so the four
		// callers (Get / Update / Delete / TogglePin / Copy / Hybrid-search
		// path) don't have to wrap NewInternalServerError into a 500 for
		// every probe of a non-existent id.
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			return nil, id, 0, apperrors.NewNotFoundError("knowledge base not found")
		}
		logger.ErrorWithFields(ctx, err, nil)
		return nil, id, 0, apperrors.NewInternalServerError(err.Error())
	}

	if kb.TenantID != tenantID {
		logger.Warnf(ctx, "Knowledge base %s belongs to tenant %d, request tenant is %d", id, kb.TenantID, tenantID)
		return nil, id, 0, apperrors.NewForbiddenError("No permission to operate")
	}
	return kb, id, tenantID, nil
}

// GetKnowledgeBase godoc
// @Summary      获取知识库详情
// @Description  根据ID获取知识库详情。
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id         path      string  true   "知识库ID"
// @Param        agent_id   query     string  false  "共享智能体 ID（用于校验智能体是否有权访问该知识库）"
// @Success      200  {object}  map[string]interface{}  "知识库详情"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "知识库不存在"
// @Router       /knowledge-bases/{id} [get]
func (h *KnowledgeBaseHandler) GetKnowledgeBase(c *gin.Context) {
	// Validate and get the knowledge base
	kb, _, _, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}
	// Fill counts (knowledge_count, chunk_count, is_processing) so hover/detail shows correct numbers
	if fillErr := h.service.FillKnowledgeBaseCounts(c.Request.Context(), kb); fillErr != nil {
		logger.Warnf(c.Request.Context(), "Failed to fill KB counts for %s: %v", kb.ID, fillErr)
	}
	if _, access, accessErr := h.shareService.ResolveAccess(c.Request.Context(), kb.ID, types.KBRoleReader); accessErr == nil {
		kb.AccessRole = access.Role
		kb.IsShared = access.Shared
		kb.OwnerUsername = access.OwnerUsername
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": buildKBResponse(kb, nil)})
}

// ListKnowledgeBases godoc
// @Summary      获取知识库列表
// @Description  获取当前工作区的所有知识库。
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Success      200  {object}  map[string]interface{}  "知识库列表"
// @Failure      500  {object}  errors.AppError         "服务器错误"
// @Router       /knowledge-bases [get]
func (h *KnowledgeBaseHandler) ListKnowledgeBases(c *gin.Context) {
	ctx := c.Request.Context()

	// Include owned KBs plus explicitly shared KB memberships.
	kbs, err := h.shareService.ListAccessibleKnowledgeBases(ctx)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	for _, kb := range kbs {
		if fillErr := h.service.FillKnowledgeBaseCounts(ctx, kb); fillErr != nil {
			logger.Warnf(ctx, "Failed to fill KB counts for %s: %v", kb.ID, fillErr)
		}
	}

	callerTenantID := c.GetUint64(types.TenantIDContextKey.String())
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.buildKBListResponse(ctx, kbs, callerTenantID),
	})
}

// TogglePinKnowledgeBase godoc
// @Summary      置顶/取消置顶知识库
// @Description  切换知识库的置顶状态
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id  path      string  true  "知识库ID"
// @Success      200  {object}  map[string]interface{}  "更新后的知识库"
// @Failure      404  {object}  errors.AppError         "知识库不存在"
// @Router       /knowledge-bases/{id}/pin [put]
func (h *KnowledgeBaseHandler) TogglePinKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	if id == "" {
		c.Error(apperrors.NewBadRequestError("knowledge base ID is required"))
		return
	}

	kb, err := h.service.TogglePinKnowledgeBase(ctx, id)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(apperrors.NewNotFoundError("knowledge base not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, nil),
	})
}

// UpdateKnowledgeBaseRequest defines the request body structure for updating a knowledge base
type UpdateKnowledgeBaseRequest struct {
	Name        string                     `json:"name"        binding:"required"`
	Description string                     `json:"description"`
	Config      *types.KnowledgeBaseConfig `json:"config"`
}

// UpdateKnowledgeBase godoc
// @Summary      更新知识库
// @Description  更新知识库的名称、描述和配置
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id       path      string                     true  "知识库ID"
// @Param        request  body      UpdateKnowledgeBaseRequest true  "更新请求"
// @Success      200      {object}  map[string]interface{}     "更新后的知识库"
// @Failure      400      {object}  errors.AppError            "请求参数错误"
// @Router       /knowledge-bases/{id} [put]
func (h *KnowledgeBaseHandler) UpdateKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start updating knowledge base")

	// Validate and get the knowledge base
	_, id, _, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	// Parse request body
	var req UpdateKnowledgeBaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(apperrors.NewBadRequestError("Invalid request parameters").WithDetails(err.Error()))
		return
	}
	if req.Config != nil && req.Config.MaxFileSizeMB != nil {
		if err := validateKnowledgeBaseMaxFileSizeMB(*req.Config.MaxFileSizeMB); err != nil {
			c.Error(err)
			return
		}
	}
	logger.Infof(ctx, "Updating knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(req.Name))

	// Update the knowledge base
	kb, err := h.service.UpdateKnowledgeBase(ctx, id, req.Name, req.Description, req.Config)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base updated successfully, ID: %s",
		secutils.SanitizeForLog(id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    buildKBResponse(kb, nil),
	})
}

// DeleteKnowledgeBase godoc
// @Summary      删除知识库
// @Description  删除指定的知识库及其所有内容
// @Tags         知识库
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "知识库ID"
// @Success      200  {object}  map[string]interface{}  "删除成功"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Router       /knowledge-bases/{id} [delete]
func (h *KnowledgeBaseHandler) DeleteKnowledgeBase(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting knowledge base")

	// Validate and get the knowledge base
	kb, id, _, err := h.validateAndGetKnowledgeBase(c)
	if err != nil {
		c.Error(err)
		return
	}

	logger.Infof(ctx, "Deleting knowledge base, ID: %s, name: %s",
		secutils.SanitizeForLog(id), secutils.SanitizeForLog(kb.Name))

	// Delete the knowledge base
	if err := h.service.TrashKnowledgeBase(ctx, id); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge base deleted successfully, ID: %s",
		secutils.SanitizeForLog(id))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Knowledge base moved to recycle bin",
	})
}

func (h *KnowledgeBaseHandler) ListTrashedKnowledgeBases(c *gin.Context) {
	rows, err := h.service.ListTrashedKnowledgeBases(c.Request.Context())
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": rows})
}

func (h *KnowledgeBaseHandler) RestoreTrashedKnowledgeBase(c *gin.Context) {
	if err := h.service.RestoreKnowledgeBase(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *KnowledgeBaseHandler) PurgeTrashedKnowledgeBase(c *gin.Context) {
	if err := h.service.PurgeTrashedKnowledgeBase(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(apperrors.NewBadRequestError(err.Error()))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Permanent deletion started"})
}

// ListMoveTargets returns knowledge bases eligible as move targets for the given source KB.
// Filters: same Type, same EmbeddingModelID, different ID, not temporary.
//
// ListMoveTargets godoc
// @Summary      获取可移动目标知识库列表
// @Description  返回与源知识库 Type 一致、EmbeddingModelID 一致、非临时且不是自身的目标知识库列表
// @Tags         知识库
// @Produce      json
// @Param        id   path      string                  true  "源知识库 ID"
// @Success      200  {object}  map[string]interface{}  "可移动目标列表"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "知识库不存在"
// @Router       /knowledge-bases/{id}/move-targets [get]
func (h *KnowledgeBaseHandler) ListMoveTargets(c *gin.Context) {
	ctx := c.Request.Context()

	sourceKBID := c.Param("id")
	if sourceKBID == "" {
		c.Error(apperrors.NewBadRequestError("Knowledge base ID is required"))
		return
	}

	tenantID, exists := c.Get(types.TenantIDContextKey.String())
	if !exists {
		c.Error(apperrors.NewUnauthorizedError("Unauthorized"))
		return
	}

	// Get source knowledge base
	sourceKB, err := h.service.GetKnowledgeBaseByID(ctx, sourceKBID)
	if err != nil {
		if stderrors.Is(err, repository.ErrKnowledgeBaseNotFound) {
			c.Error(errors.NewNotFoundError("Source knowledge base not found"))
			return
		}
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}
	if sourceKB.TenantID != tenantID.(uint64) {
		c.Error(errors.NewForbiddenError("No permission to access this knowledge base"))
		return
	}

	// Get all knowledge bases
	allKBs, err := h.service.ListKnowledgeBases(ctx)
	if err != nil {
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// Filter eligible targets
	targets := make([]*types.KnowledgeBase, 0)
	for _, kb := range allKBs {
		if kb.ID == sourceKBID {
			continue
		}
		if kb.IsTemporary {
			continue
		}
		if kb.Type != sourceKB.Type {
			continue
		}
		if kb.EmbeddingModelID != sourceKB.EmbeddingModelID {
			continue
		}
		targets = append(targets, kb)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    targets,
	})
}
