package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

type fixedKBRepo struct {
	rows map[string]*types.KnowledgeBase
}

func newFixedKBRepo() *fixedKBRepo {
	return &fixedKBRepo{rows: make(map[string]*types.KnowledgeBase)}
}

func (r *fixedKBRepo) CreateKnowledgeBase(_ context.Context, kb *types.KnowledgeBase) error {
	r.rows[kb.ID] = kb
	return nil
}

func (r *fixedKBRepo) GetKnowledgeBaseByID(_ context.Context, id string) (*types.KnowledgeBase, error) {
	kb := r.rows[id]
	if kb == nil {
		return nil, errors.New("not found")
	}
	return kb, nil
}

func (r *fixedKBRepo) GetKnowledgeBaseByIDAndTenant(_ context.Context, id string, tenantID uint64) (*types.KnowledgeBase, error) {
	kb := r.rows[id]
	if kb == nil || kb.TenantID != tenantID {
		return nil, errors.New("not found")
	}
	return kb, nil
}

func (r *fixedKBRepo) GetKnowledgeBaseByIDs(_ context.Context, _ []string) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (r *fixedKBRepo) ListKnowledgeBases(_ context.Context) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (r *fixedKBRepo) ListKnowledgeBasesByTenantID(_ context.Context, _ uint64) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (r *fixedKBRepo) UpdateKnowledgeBase(_ context.Context, _ *types.KnowledgeBase) error {
	return nil
}

func (r *fixedKBRepo) DeleteKnowledgeBase(_ context.Context, _ string) error {
	return nil
}

func (r *fixedKBRepo) GetTrashedKnowledgeBase(_ context.Context, _ uint64, _ string) (*types.KnowledgeBase, error) {
	return nil, errors.New("not found")
}

func (r *fixedKBRepo) ListTrashedKnowledgeBases(_ context.Context, _ uint64) ([]*types.KnowledgeBase, error) {
	return nil, nil
}

func (r *fixedKBRepo) RestoreKnowledgeBase(_ context.Context, _ uint64, _ string) error {
	return nil
}

func (r *fixedKBRepo) PurgeKnowledgeBase(_ context.Context, _ uint64, _ string) error {
	return nil
}

func (r *fixedKBRepo) TogglePinKnowledgeBase(_ context.Context, _ string, _ uint64) (*types.KnowledgeBase, error) {
	return nil, nil
}

func (r *fixedKBRepo) SetUserKBPin(_ context.Context, _ uint64, _ string, _ string, _ bool) (*time.Time, error) {
	return nil, nil
}

func (r *fixedKBRepo) ListUserKBPinIDs(_ context.Context, _ uint64, _ string) (map[string]time.Time, error) {
	return map[string]time.Time{}, nil
}

var _ interfaces.KnowledgeBaseRepository = (*fixedKBRepo)(nil)

func fixedTenantContext() context.Context {
	ctx := context.WithValue(context.Background(), types.TenantIDContextKey, uint64(1))
	tenant := &types.Tenant{ID: 1}
	return context.WithValue(ctx, types.TenantInfoContextKey, tenant)
}

func TestCreateKnowledgeBaseIgnoresLegacyVectorStoreBinding(t *testing.T) {
	repo := newFixedKBRepo()
	svc := &knowledgeBaseService{repo: repo, modelService: &stubModelService{}}
	legacyStoreID := "0193b8a0-1111-7000-8000-000000000001"
	kb, err := svc.CreateKnowledgeBase(fixedTenantContext(), &types.KnowledgeBase{
		Name:          "kb",
		VectorStoreID: &legacyStoreID,
	})
	require.NoError(t, err)
	assert.Nil(t, kb.VectorStoreID)
	assert.Equal(t, "chat-model", kb.SummaryModelID)
	assert.Equal(t, "chat-model", kb.EmbeddingModelID)
	assert.Len(t, repo.rows, 1)
}
