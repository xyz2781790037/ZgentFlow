package retriever

import (
	"fmt"
	"sync"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"github.com/xyz2781790037/ZealRAG/internal/types/interfaces"
)

// RetrieveEngineRegistry stores ZealRAG's built-in retrieval engine by type.
type RetrieveEngineRegistry struct {
	byEngineType map[types.RetrieverEngineType]interfaces.RetrieveEngineService
	mu           sync.RWMutex
}

// NewRetrieveEngineRegistry creates a new retrieval engine registry
func NewRetrieveEngineRegistry() interfaces.RetrieveEngineRegistry {
	return &RetrieveEngineRegistry{
		byEngineType: make(map[types.RetrieverEngineType]interfaces.RetrieveEngineService),
	}
}

// --- interfaces.RetrieveEngineRegistry methods (unchanged behavior) ---

// Register registers a retrieval engine service by engine type.
// Returns an error if the engine type is already registered.
func (r *RetrieveEngineRegistry) Register(repo interfaces.RetrieveEngineService) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if repo.EngineType() != types.PostgresRetrieverEngineType {
		return fmt.Errorf("ZealRAG only supports the postgres retrieval engine")
	}

	if _, exists := r.byEngineType[repo.EngineType()]; exists {
		return fmt.Errorf("repository type %s already registered", repo.EngineType())
	}

	r.byEngineType[repo.EngineType()] = repo
	return nil
}

// GetRetrieveEngineService retrieves a retrieval engine service by type.
// Only searches the byEngineType map (env stores).
func (r *RetrieveEngineRegistry) GetRetrieveEngineService(repoType types.RetrieverEngineType) (
	interfaces.RetrieveEngineService, error,
) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	repo, exists := r.byEngineType[repoType]
	if !exists {
		return nil, fmt.Errorf("repository of type %s not found", repoType)
	}

	return repo, nil
}

// GetAllRetrieveEngineServices retrieves all registered retrieval engine services.
// Only returns byEngineType entries (env stores) for backward compatibility.
func (r *RetrieveEngineRegistry) GetAllRetrieveEngineServices() []interfaces.RetrieveEngineService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]interfaces.RetrieveEngineService, 0, len(r.byEngineType))
	for _, v := range r.byEngineType {
		result = append(result, v)
	}

	return result
}

// Compile-time assertion for the public registry contract.
var _ interfaces.RetrieveEngineRegistry = (*RetrieveEngineRegistry)(nil)
