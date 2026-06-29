package interfaces

import (
	"context"

	"github.com/hibiken/asynq"
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// KBFullRebuildService atomically rebuilds vectors and Wiki.
type KBFullRebuildService interface {
	Start(ctx context.Context, kbID string) (*types.KBRebuildState, error)
	GetState(ctx context.Context, kbID string) (*types.KBRebuildState, error)
	Handle(ctx context.Context, task *asynq.Task) error
}
