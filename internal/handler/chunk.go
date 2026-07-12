package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/application/service"
	"github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// ChunkHandler defines HTTP handlers for chunk operations.
type ChunkHandler struct {
	service interfaces.ChunkService
}

// NewChunkHandler creates a new chunk handler.
func NewChunkHandler(service interfaces.ChunkService) *ChunkHandler {
	return &ChunkHandler{service: service}
}

// GetChunkByIDOnly godoc
// @Summary      通过ID获取分块
// @Description  仅通过分块ID获取分块详情（不需要knowledge_id）
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        id   path      string  true  "分块ID"
// @Success      200  {object}  map[string]interface{}  "分块详情"
// @Failure      400  {object}  errors.AppError         "请求参数错误"
// @Failure      404  {object}  errors.AppError         "分块不存在"
// @Router       /chunks/by-id/{id} [get]
func (h *ChunkHandler) GetChunkByIDOnly(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start retrieving chunk by ID only")

	chunkID := secutils.SanitizeForLog(c.Param("id"))
	if chunkID == "" {
		logger.Error(ctx, "Chunk ID is empty")
		c.Error(errors.NewBadRequestError("Chunk ID cannot be empty"))
		return
	}

	// Get chunk by ID without tenant filter (chunk may belong to shared
	// KB; the route-level KB-access guard already verified read
	// permission against the parent KB before we got here).
	chunk, err := h.service.GetChunkByIDOnly(ctx, chunkID)
	if err != nil {
		if err == service.ErrChunkNotFound {
			logger.Warnf(ctx, "Chunk not found, chunk ID: %s", chunkID)
			c.Error(errors.NewNotFoundError("Chunk not found"))
			return
		}
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// 对 chunk 内容进行安全清理
	if chunk.Content != "" {
		chunk.Content = secutils.SanitizeForDisplay(chunk.Content)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    chunk,
	})
}

// ListKnowledgeChunks godoc
// @Summary      获取知识分块列表
// @Description  获取指定知识下的所有分块列表，支持分页
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        knowledge_id  path      string  true   "知识ID"
// @Param        page          query     int     false  "页码"  default(1)
// @Param        page_size     query     int     false  "每页数量"  default(10)
// @Success      200           {object}  map[string]interface{}  "分块列表"
// @Failure      400           {object}  errors.AppError         "请求参数错误"
// @Router       /chunks/{knowledge_id} [get]
func (h *ChunkHandler) ListKnowledgeChunks(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start retrieving knowledge chunks list")

	knowledgeID := secutils.SanitizeForLog(c.Param("knowledge_id"))
	if knowledgeID == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	// Parse pagination parameters
	var pagination types.Pagination
	if err := c.ShouldBindQuery(&pagination); err != nil {
		logger.Errorf(ctx, "Failed to parse pagination parameters: %s", secutils.SanitizeForLog(err.Error()))
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}
	if pagination.Page < 1 {
		pagination.Page = 1
	}
	if pagination.PageSize < 1 {
		pagination.PageSize = 10
	}
	if pagination.PageSize > 100 {
		pagination.PageSize = 100
	}

	// Default to text chunks; callers may override via ?chunk_type=image_caption etc.
	chunkType := []types.ChunkType{types.ChunkTypeText}
	if queryTypes := c.QueryArray("chunk_type"); len(queryTypes) > 0 {
		chunkType = make([]types.ChunkType, 0, len(queryTypes))
		for _, qt := range queryTypes {
			chunkType = append(chunkType, types.ChunkType(qt))
		}
	}

	// The route-level guard has rewritten the request's tenant context
	// to the effective tenant for shared KBs.
	result, err := h.service.ListPagedChunksByKnowledgeID(ctx, knowledgeID, &pagination, chunkType)
	if err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	// 对 chunk 内容进行安全清理
	for _, chunk := range result.Data.([]*types.Chunk) {
		if chunk.Content != "" {
			chunk.Content = secutils.SanitizeForDisplay(chunk.Content)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      result.Data,
		"total":     result.Total,
		"page":      result.Page,
		"page_size": result.PageSize,
	})
}

// UpdateChunkRequest defines the request structure for updating a chunk
type UpdateChunkRequest struct {
	Content    string    `json:"content"`
	Embedding  []float32 `json:"embedding"`
	ChunkIndex int       `json:"chunk_index"`
	IsEnabled  bool      `json:"is_enabled"`
	StartAt    int       `json:"start_at"`
	EndAt      int       `json:"end_at"`
	ImageInfo  string    `json:"image_info"`
}

// fetchChunkAndVerifyOwnership fetches a chunk by ID and verifies it
// belongs to the URL :knowledge_id (defence in depth: the route-level
// KB-access guard already ensured the caller has write access to the
// KB; this check stops a same-tenant attacker from passing one
// knowledge_id while addressing a chunk owned by a different
// knowledge in the same KB).
func (h *ChunkHandler) fetchChunkAndVerifyOwnership(c *gin.Context) (*types.Chunk, string, error) {
	ctx := c.Request.Context()

	knowledgeID := secutils.SanitizeForLog(c.Param("knowledge_id"))
	if knowledgeID == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		return nil, "", errors.NewBadRequestError("Knowledge ID cannot be empty")
	}
	id := secutils.SanitizeForLog(c.Param("id"))
	if id == "" {
		logger.Error(ctx, "Chunk ID is empty")
		return nil, knowledgeID, errors.NewBadRequestError("Chunk ID cannot be empty")
	}
	chunk, err := h.service.GetChunkByID(ctx, id)
	if err != nil {
		if err == service.ErrChunkNotFound {
			logger.Warnf(ctx, "Chunk not found, knowledge ID: %s, chunk ID: %s", knowledgeID, id)
			return nil, knowledgeID, errors.NewNotFoundError("Chunk not found")
		}
		logger.ErrorWithFields(ctx, err, nil)
		return nil, knowledgeID, errors.NewInternalServerError(err.Error())
	}
	if chunk.KnowledgeID != knowledgeID {
		logger.Warnf(ctx, "Chunk does not belong to knowledge, knowledge ID: %s, chunk ID: %s", knowledgeID, id)
		return nil, knowledgeID, errors.NewForbiddenError("No permission to access this chunk")
	}
	return chunk, knowledgeID, nil
}

// UpdateChunk godoc
// @Summary      更新分块
// @Description  更新指定分块的内容和属性
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        knowledge_id  path      string              true  "知识ID"
// @Param        id            path      string              true  "分块ID"
// @Param        request       body      UpdateChunkRequest  true  "更新请求"
// @Success      200           {object}  map[string]interface{}  "更新后的分块"
// @Failure      400           {object}  errors.AppError         "请求参数错误"
// @Failure      404           {object}  errors.AppError         "分块不存在"
// @Router       /chunks/{knowledge_id}/{id} [put]
func (h *ChunkHandler) UpdateChunk(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start updating knowledge chunk")

	chunk, knowledgeID, err := h.fetchChunkAndVerifyOwnership(c)
	if err != nil {
		c.Error(err)
		return
	}
	var req UpdateChunkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf(ctx, "Failed to parse request parameters: %s", secutils.SanitizeForLog(err.Error()))
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	if req.Content != "" {
		chunk.Content = req.Content
	}

	chunk.IsEnabled = req.IsEnabled

	if err := h.service.UpdateChunk(ctx, chunk); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	logger.Infof(ctx, "Knowledge chunk updated successfully, knowledge ID: %s, chunk ID: %s",
		secutils.SanitizeForLog(knowledgeID), secutils.SanitizeForLog(chunk.ID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    chunk,
	})
}

// DeleteChunk godoc
// @Summary      删除分块
// @Description  删除指定的分块
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        knowledge_id  path      string  true  "知识ID"
// @Param        id            path      string  true  "分块ID"
// @Success      200           {object}  map[string]interface{}  "删除成功"
// @Failure      400           {object}  errors.AppError         "请求参数错误"
// @Failure      404           {object}  errors.AppError         "分块不存在"
// @Router       /chunks/{knowledge_id}/{id} [delete]
func (h *ChunkHandler) DeleteChunk(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting knowledge chunk")

	chunk, _, err := h.fetchChunkAndVerifyOwnership(c)
	if err != nil {
		c.Error(err)
		return
	}

	if err := h.service.DeleteChunk(ctx, chunk.ID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Chunk deleted",
	})
}

// DeleteChunksByKnowledgeID godoc
// @Summary      删除知识下所有分块
// @Description  删除指定知识下的所有分块
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        knowledge_id  path      string  true  "知识ID"
// @Success      200           {object}  map[string]interface{}  "删除成功"
// @Failure      400           {object}  errors.AppError         "请求参数错误"
// @Router       /chunks/{knowledge_id} [delete]
func (h *ChunkHandler) DeleteChunksByKnowledgeID(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting all chunks under knowledge")

	knowledgeID := secutils.SanitizeForLog(c.Param("knowledge_id"))
	if knowledgeID == "" {
		logger.Error(ctx, "Knowledge ID is empty")
		c.Error(errors.NewBadRequestError("Knowledge ID cannot be empty"))
		return
	}

	if err := h.service.DeleteChunksByKnowledgeID(ctx, knowledgeID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewInternalServerError(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All chunks under knowledge deleted",
	})
}

// DeleteGeneratedQuestion godoc
// @Summary      删除生成的问题
// @Description  删除分块中生成的问题
// @Tags         分块管理
// @Accept       json
// @Produce      json
// @Param        id       path      string                       true  "分块ID"
// @Param        request  body      object{question_id=string}   true  "问题ID"
// @Success      200      {object}  map[string]interface{}       "删除成功"
// @Failure      400      {object}  errors.AppError              "请求参数错误"
// @Failure      404      {object}  errors.AppError              "分块不存在"
// @Router       /chunks/by-id/{id}/questions [delete]
func (h *ChunkHandler) DeleteGeneratedQuestion(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Info(ctx, "Start deleting generated question from chunk")

	chunkID := secutils.SanitizeForLog(c.Param("id"))
	if chunkID == "" {
		logger.Error(ctx, "Chunk ID is empty")
		c.Error(errors.NewBadRequestError("Chunk ID cannot be empty"))
		return
	}

	var req struct {
		QuestionID string `json:"question_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Errorf(ctx, "Failed to parse request parameters: %s", secutils.SanitizeForLog(err.Error()))
		c.Error(errors.NewBadRequestError("Question ID is required"))
		return
	}

	if err := h.service.DeleteGeneratedQuestion(ctx, chunkID, req.QuestionID); err != nil {
		logger.ErrorWithFields(ctx, err, nil)
		c.Error(errors.NewBadRequestError(err.Error()))
		return
	}

	logger.Infof(ctx, "Generated question deleted successfully, chunk ID: %s, question ID: %s",
		secutils.SanitizeForLog(chunkID), secutils.SanitizeForLog(req.QuestionID))
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Generated question deleted",
	})
}
