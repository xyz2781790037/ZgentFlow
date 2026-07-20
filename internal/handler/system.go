package handler

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/infrastructure/docparser"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
	secutils "github.com/xyz2781790037/ZealRAG/internal/utils"
)

type SystemHandler struct {
	documentReader interfaces.DocumentReader
}

func NewSystemHandler(
	documentReader interfaces.DocumentReader,
) *SystemHandler {
	return &SystemHandler{
		documentReader: documentReader,
	}
}

func (h *SystemHandler) getDocReaderConnInfo() (addr, transport string) {
	addr = strings.TrimSpace(os.Getenv("DOCREADER_ADDR"))
	transport = strings.TrimSpace(os.Getenv("DOCREADER_TRANSPORT"))
	if transport == "" {
		transport = "grpc"
	}
	return addr, strings.ToLower(transport)
}

func (h *SystemHandler) ListParserEngines(c *gin.Context) {
	var overrides map[string]string
	if value, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := value.(*types.Tenant); ok && tenant != nil && tenant.ParserEngineConfig != nil {
			overrides = tenant.ParserEngineConfig.ToOverridesMap()
		}
	}

	reader, docreaderAddr, docreaderTransport := h.resolveDocReader(c.Request.Context(), overrides)
	connected := reader != nil && reader.IsConnected()
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), reader, overrides)
	engines := docparser.ListAllEngines(connected, overrides, remoteEngines)
	c.JSON(200, gin.H{
		"code":                0,
		"msg":                 "success",
		"data":                engines,
		"docreader_addr":      docreaderAddr,
		"docreader_transport": docreaderTransport,
		"connected":           connected,
	})
}

func (h *SystemHandler) ReconnectDocReader(c *gin.Context) {
	var req struct {
		Addr string `json:"addr" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "请提供 addr 参数"})
		return
	}
	addr := strings.TrimSpace(req.Addr)
	if addr == "" {
		c.JSON(400, gin.H{"code": 1, "msg": "addr 不能为空"})
		return
	}
	if err := secutils.ValidateURLForSSRF(addr); err != nil {
		logger.Warnf(c.Request.Context(), "SSRF validation failed for docreader addr: %v", err)
		c.JSON(400, gin.H{"code": 1, "msg": secutils.FormatSSRFError("DocReader 地址", addr, err)})
		return
	}
	if h.documentReader == nil {
		c.JSON(500, gin.H{"code": 1, "msg": "document converter not initialized"})
		return
	}
	if err := h.documentReader.Reconnect(addr); err != nil {
		logger.Errorf(c.Request.Context(), "Failed to reconnect docreader to %s: %v", addr, err)
		c.JSON(200, gin.H{"code": 1, "msg": fmt.Sprintf("连接失败: %v", err)})
		return
	}

	var overrides map[string]string
	if value, exists := c.Get(types.TenantInfoContextKey.String()); exists {
		if tenant, ok := value.(*types.Tenant); ok && tenant != nil && tenant.ParserEngineConfig != nil {
			overrides = tenant.ParserEngineConfig.ToOverridesMap()
		}
	}
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), h.documentReader, overrides)
	engines := docparser.ListAllEngines(true, overrides, remoteEngines)
	_, transport := h.getDocReaderConnInfo()
	c.JSON(200, gin.H{
		"code":                0,
		"msg":                 "连接成功",
		"data":                engines,
		"docreader_addr":      addr,
		"docreader_transport": transport,
		"connected":           true,
	})
}

func (h *SystemHandler) CheckParserEngines(c *gin.Context) {
	var body types.ParserEngineConfig
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"code": 1, "msg": "请求体格式错误"})
		return
	}
	overrides := body.ToOverridesMap()
	reader, docreaderAddr, docreaderTransport := h.resolveDocReader(c.Request.Context(), overrides)
	connected := reader != nil && reader.IsConnected()
	remoteEngines := h.fetchRemoteEngines(c.Request.Context(), reader, overrides)
	engines := docparser.ListAllEngines(connected, overrides, remoteEngines)
	c.JSON(200, gin.H{
		"code":                0,
		"msg":                 "success",
		"data":                engines,
		"docreader_addr":      docreaderAddr,
		"docreader_transport": docreaderTransport,
		"connected":           connected,
	})
}

func (h *SystemHandler) resolveDocReader(_ context.Context, _ map[string]string) (interfaces.DocumentReader, string, string) {
	addr, transport := h.getDocReaderConnInfo()
	return h.documentReader, addr, transport
}

func (h *SystemHandler) fetchRemoteEngines(
	ctx context.Context,
	reader interfaces.DocumentReader,
	overrides map[string]string,
) []types.ParserEngineInfo {
	if reader == nil || !reader.IsConnected() {
		return nil
	}
	engines, err := reader.ListEngines(ctx, overrides)
	if err != nil {
		logger.Warnf(ctx, "Failed to fetch remote engines from docreader: %v", err)
		return nil
	}
	return engines
}

func (h *SystemHandler) ResolveDocumentReader(_ context.Context, addr string) interfaces.DocumentReader {
	if addr == "" {
		return h.documentReader
	}
	reader, err := docparser.NewHTTPDocumentReader(addr)
	if err != nil || reader == nil {
		return reader
	}
	return reader
}
