package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	appservice "github.com/xyz2781790037/ZealRAG/internal/application/service"
	apperrors "github.com/xyz2781790037/ZealRAG/internal/errors"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

type ModelAPIConfigHandler struct {
	service interfaces.ModelAPIConfigService
}

func NewModelAPIConfigHandler(service interfaces.ModelAPIConfigService) *ModelAPIConfigHandler {
	return &ModelAPIConfigHandler{service: service}
}

type modelAPIConfigRequest struct {
	Name     string  `json:"name" binding:"required"`
	Provider string  `json:"provider" binding:"required"`
	APIKey   *string `json:"api_key"`
}

type modelAPIConfigResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Provider   string `json:"provider"`
	Configured bool   `json:"configured"`
	ModelCount int64  `json:"model_count"`
}

func newModelAPIConfigResponse(config *types.ModelAPIConfig) modelAPIConfigResponse {
	return modelAPIConfigResponse{
		ID:         config.ID,
		Name:       config.Name,
		Provider:   config.Provider,
		Configured: config.APIKey != "",
		ModelCount: config.ModelCount,
	}
}

func writeModelAPIConfigError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, appservice.ErrModelAPIConfigNotFound):
		c.Error(apperrors.NewNotFoundError("API 配置不存在"))
	case errors.Is(err, appservice.ErrModelAPIConfigNameUsed):
		c.Error(apperrors.NewBadRequestError("同一服务商下的 API 配置名称不能重复"))
	default:
		c.Error(apperrors.NewBadRequestError(err.Error()))
	}
}

func (h *ModelAPIConfigHandler) List(c *gin.Context) {
	configs, err := h.service.List(c.Request.Context())
	if err != nil {
		c.Error(apperrors.NewInternalServerError(err.Error()))
		return
	}
	data := make([]modelAPIConfigResponse, 0, len(configs))
	for _, config := range configs {
		data = append(data, newModelAPIConfigResponse(config))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func (h *ModelAPIConfigHandler) Create(c *gin.Context) {
	var req modelAPIConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.APIKey == nil {
		c.Error(apperrors.NewBadRequestError("名称、服务商和 API Key 均不能为空"))
		return
	}
	config, err := h.service.Create(c.Request.Context(), req.Name, req.Provider, *req.APIKey)
	if err != nil {
		writeModelAPIConfigError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": newModelAPIConfigResponse(config)})
}

func (h *ModelAPIConfigHandler) Update(c *gin.Context) {
	var req modelAPIConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewBadRequestError("名称和服务商不能为空"))
		return
	}
	config, err := h.service.Update(c.Request.Context(), c.Param("id"), req.Name, req.Provider, req.APIKey)
	if err != nil {
		writeModelAPIConfigError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newModelAPIConfigResponse(config)})
}

func (h *ModelAPIConfigHandler) Delete(c *gin.Context) {
	count, err := h.service.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeModelAPIConfigError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    gin.H{"affected_model_count": count},
	})
}
