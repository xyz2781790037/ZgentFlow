package types

import "time"

const (
	KBRebuildStatusIdle      = "idle"
	KBRebuildStatusPending   = "pending"
	KBRebuildStatusRunning   = "running"
	KBRebuildStatusSucceeded = "succeeded"
	KBRebuildStatusFailed    = "failed"
)

// KBFullRebuildPayload identifies one immutable rebuild attempt.
type KBFullRebuildPayload struct {
	TracingContext
	TenantID           uint64 `json:"tenant_id"`
	KnowledgeBaseID    string `json:"knowledge_base_id"`
	Generation         int64  `json:"generation"`
	EmbeddingModelID   string `json:"embedding_model_id,omitempty"`
	KnowledgeQAModelID string `json:"knowledge_qa_model_id,omitempty"`
}

// KBRebuildState is returned by the start/status endpoints.
type KBRebuildState struct {
	KnowledgeBaseID    string     `json:"knowledge_base_id"`
	ActiveGeneration   int64      `json:"active_generation"`
	BuildingGeneration *int64     `json:"building_generation,omitempty"`
	Status             string     `json:"status"`
	Error              string     `json:"error,omitempty"`
	StartedAt          *time.Time `json:"started_at,omitempty"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
}
