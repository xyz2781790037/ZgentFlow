package service

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// retrieveFromStores keeps the established call shape while executing the
// single PostgreSQL/pgvector retrieval unit used by ZealRAG.
func (s *knowledgeBaseService) retrieveFromStores(
	ctx context.Context,
	groups []*storeGroup,
) ([]*types.RetrieveResult, error) {
	if len(groups) == 0 || groups[0] == nil || groups[0].Engine == nil {
		return nil, nil
	}
	return groups[0].Engine.Retrieve(ctx, paramsWithTopK(groups[0]))
}

func paramsWithTopK(group *storeGroup) []types.RetrieveParams {
	params := make([]types.RetrieveParams, len(group.BaseParams))
	for i, param := range group.BaseParams {
		param.TopK = group.TopK
		params[i] = param
	}
	return params
}
