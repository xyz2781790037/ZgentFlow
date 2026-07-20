package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/xyz2781790037/ZealRAG/internal/middleware"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

type stubKBCreateService struct {
	interfaces.KnowledgeBaseService
	createErr error
}

func (s *stubKBCreateService) CreateKnowledgeBase(_ context.Context, kb *types.KnowledgeBase) (*types.KnowledgeBase, error) {
	if s.createErr != nil {
		return nil, s.createErr
	}
	kb.ID = "kb-new"
	kb.TenantID = 1
	return kb, nil
}

func newCreateKBRouter(svc interfaces.KnowledgeBaseService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.Use(func(c *gin.Context) {
		c.Set(types.TenantIDContextKey.String(), uint64(1))
		c.Set(types.UserIDContextKey.String(), "u-test")
		c.Next()
	})
	handler := &KnowledgeBaseHandler{service: svc}
	router.POST("/knowledge-bases", handler.CreateKnowledgeBase)
	return router
}

func TestCreateKBRawServiceErrorReturns500(t *testing.T) {
	response := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost, "/knowledge-bases", strings.NewReader(`{"name":"kb"}`))
	request.Header.Set("Content-Type", "application/json")
	newCreateKBRouter(&stubKBCreateService{createErr: testError("database unavailable")}).ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", response.Code, response.Body.String())
	}
}

func TestBuildKBResponseHidesLegacyVectorStoreFields(t *testing.T) {
	legacyStoreID := "aaaa-bbbb-cccc-dddd"
	kb := &types.KnowledgeBase{
		ID:            "kb-1",
		Name:          "knowledge",
		TenantID:      1,
		VectorStoreID: &legacyStoreID,
	}
	got := buildKBResponse(kb, map[string]interface{}{"my_permission": "viewer"})
	row, ok := got.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", got)
	}
	if _, exists := row["vector_store_id"]; exists {
		t.Fatalf("response must not expose legacy vector_store_id")
	}
	if row["my_permission"] != "viewer" {
		t.Fatalf("expected merged response field, got %v", row["my_permission"])
	}
	serialized, _ := json.Marshal(row)
	if strings.Contains(string(serialized), legacyStoreID) {
		t.Fatalf("response leaked legacy store ID: %s", serialized)
	}
}

type testError string

func (e testError) Error() string { return string(e) }
