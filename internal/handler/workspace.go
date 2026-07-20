package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

// WorkspaceHandler exposes configuration for the local ZealRAG workspace.
type WorkspaceHandler struct {
	service interfaces.TenantService
}

func NewWorkspaceHandler(
	service interfaces.TenantService,
) *WorkspaceHandler {
	return &WorkspaceHandler{service: service}
}

func (h *WorkspaceHandler) GetConfig(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "parser-engine-config":
		h.getParserEngineConfig(c)
	case "retrieval-config":
		h.getRetrievalConfig(c)
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
	}
}

func (h *WorkspaceHandler) UpdateConfig(c *gin.Context) {
	ctx := c.Request.Context()
	key := secutils.SanitizeForLog(c.Param("key"))

	switch key {
	case "parser-engine-config":
		h.updateParserEngineConfig(c)
	case "retrieval-config":
		h.updateRetrievalConfig(c)
	default:
		logger.Info(ctx, "KV key not supported", "key", key)
		c.Error(errors.NewBadRequestError("unsupported key"))
	}
}

func currentTenant(c *gin.Context) (*types.Tenant, bool) {
	ctx := c.Request.Context()
	tenant, _ := types.TenantInfoFromContext(ctx)
	if tenant == nil {
		logger.Error(ctx, "Tenant is empty")
		c.Error(errors.NewBadRequestError("Tenant is empty"))
		return nil, false
	}
	return tenant, true
}

func handleTenantUpdateError(c *gin.Context, message string, err error) {
	ctx := c.Request.Context()
	if appErr, ok := errors.IsAppError(err); ok {
		c.Error(appErr)
		return
	}
	logger.ErrorWithFields(ctx, err, nil)
	c.Error(errors.NewInternalServerError(message).WithDetails(err.Error()))
}

func (h *WorkspaceHandler) getParserEngineConfig(c *gin.Context) {
	tenant, ok := currentTenant(c)
	if !ok {
		return
	}

	data := tenant.ParserEngineConfig
	if data == nil {
		data = &types.ParserEngineConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *WorkspaceHandler) updateParserEngineConfig(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.ParserEngineConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	tenant, ok := currentTenant(c)
	if !ok {
		return
	}
	tenant.ParserEngineConfig = &cfg

	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		handleTenantUpdateError(c, "Failed to update tenant parser engine config", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.ParserEngineConfig,
		"message": "Parser engine configuration updated successfully",
	})
}

func (h *WorkspaceHandler) getRetrievalConfig(c *gin.Context) {
	tenant, ok := currentTenant(c)
	if !ok {
		return
	}

	data := tenant.RetrievalConfig
	if data == nil {
		data = &types.RetrievalConfig{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *WorkspaceHandler) updateRetrievalConfig(c *gin.Context) {
	ctx := c.Request.Context()
	var cfg types.RetrievalConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		logger.Error(ctx, "Failed to parse request parameters", err)
		c.Error(errors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}

	if cfg.VectorThreshold < 0 || cfg.VectorThreshold > 1 {
		c.Error(errors.NewBadRequestError("vector_threshold must be between 0 and 1"))
		return
	}
	if cfg.KeywordThreshold < 0 || cfg.KeywordThreshold > 1 {
		c.Error(errors.NewBadRequestError("keyword_threshold must be between 0 and 1"))
		return
	}
	if cfg.RerankThreshold < -10 || cfg.RerankThreshold > 10 {
		c.Error(errors.NewBadRequestError("rerank_threshold must be between -10 and 10"))
		return
	}
	if cfg.EmbeddingTopK < 0 || cfg.EmbeddingTopK > 200 {
		c.Error(errors.NewBadRequestError("embedding_top_k must be between 0 and 200"))
		return
	}
	if cfg.RerankTopK < 0 || cfg.RerankTopK > 200 {
		c.Error(errors.NewBadRequestError("rerank_top_k must be between 0 and 200"))
		return
	}

	tenant, ok := currentTenant(c)
	if !ok {
		return
	}
	tenant.RetrievalConfig = &cfg

	updatedTenant, err := h.service.UpdateTenant(ctx, tenant)
	if err != nil {
		handleTenantUpdateError(c, "Failed to update retrieval config", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    updatedTenant.RetrievalConfig,
		"message": "Retrieval configuration updated successfully",
	})
}
