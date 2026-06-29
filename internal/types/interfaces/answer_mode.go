package interfaces

import (
	"context"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// AnswerModeService is the retained read-only catalog for the two built-in
// answer modes. User-created agents are not part of the product.
type AnswerModeService interface {
	GetAnswerModeByID(ctx context.Context, id string) (*types.AnswerMode, error)
	ListAnswerModes(ctx context.Context) ([]*types.AnswerMode, error)
}
