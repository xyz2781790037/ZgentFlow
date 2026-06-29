package types

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// KnowledgeTypeFAQ represents the FAQ knowledge type
	KnowledgeTypeFAQ = "faq"
)

// Knowledge parse status constants
const (
	// ParseStatusPending indicates the knowledge is waiting to be processed
	ParseStatusPending = "pending"
	// ParseStatusProcessing indicates the knowledge is being processed
	// (DocReader / chunking / embedding stage).
	ParseStatusProcessing = "processing"
	// ParseStatusFinalizing indicates the primary parse has finished but
	// enrichment subtasks (summary, question generation, Wiki generation)
	// are still in flight. The user-facing intuition behind this state is
	// "the document is queryable for vector search but is still spending
	// resources" — cancel-parse can interrupt enrichment from here.
	// pending_subtasks_count holds the outstanding subtask count; the
	// last subtask to finish atomically promotes the row to completed.
	ParseStatusFinalizing = "finalizing"
	// ParseStatusCompleted indicates the knowledge has been processed
	// successfully AND every enrichment subtask has reached a terminal
	// state. No further resources will be spent on the document until
	// the user explicitly re-parses it.
	ParseStatusCompleted = "completed"
	// ParseStatusFailed indicates the knowledge processing failed
	ParseStatusFailed = "failed"
	// ParseStatusDeleting indicates the knowledge is being deleted (used to prevent async task conflicts)
	ParseStatusDeleting = "deleting"
	// ParseStatusCancelled indicates parsing was cancelled by the user.
	// Same short-circuit semantics as ParseStatusDeleting for in-flight and
	// queued downstream tasks, but the knowledge row and any already-written
	// chunks/index are kept so the user can re-trigger parsing via reparse.
	ParseStatusCancelled = "cancelled"
)

// Summary status constants for async summary generation
const (
	// SummaryStatusNone indicates no summary task is needed
	SummaryStatusNone = "none"
	// SummaryStatusPending indicates the summary task is waiting to be processed
	SummaryStatusPending = "pending"
	// SummaryStatusProcessing indicates the summary is being generated
	SummaryStatusProcessing = "processing"
	// SummaryStatusCompleted indicates the summary has been generated successfully
	SummaryStatusCompleted = "completed"
	// SummaryStatusFailed indicates the summary generation failed
	SummaryStatusFailed = "failed"
)

// KnowledgeListFilter aggregates optional filters for listing knowledge entries
// under a knowledge base. Empty / zero fields mean "no filter on that dimension".
type KnowledgeListFilter struct {
	// Keyword performs a LIKE match on file_name / title when non-empty.
	Keyword string
	// FileType filters by file_type.
	FileType string
	// ParseStatus filters by parse_status when non-empty (e.g. pending, processing, completed, failed).
	ParseStatus string
	// UpdatedFrom, when non-zero, keeps rows with updated_at >= UpdatedFrom.
	UpdatedFrom time.Time
	// UpdatedTo, when non-zero, keeps rows with updated_at <= UpdatedTo.
	UpdatedTo time.Time
}

// Knowledge represents a knowledge entity in the system.
// It contains metadata about the knowledge source, its processing status,
// and references to the physical file if applicable.
type Knowledge struct {
	// Unique identifier of the knowledge
	ID string `json:"id"                 gorm:"type:varchar(36);primaryKey"`
	// Tenant ID
	TenantID uint64 `json:"tenant_id"`
	// ID of the knowledge base
	KnowledgeBaseID string `json:"knowledge_base_id"`
	// UploadedByUserID records who added the document to a shared KB.
	UploadedByUserID string `json:"uploaded_by_user_id" gorm:"type:varchar(36);index"`
	// Type of the knowledge
	Type string `json:"type"`
	// Title of the knowledge
	Title string `json:"title"`
	// Description of the knowledge
	Description string `json:"description"`
	// Source identifies the original uploaded file.
	Source string `json:"source"             gorm:"type:varchar(2048)"`
	// Parse status of the knowledge
	ParseStatus string `json:"parse_status"`
	// PendingSubtasksCount is the outstanding enrichment subtask count
	// (summary + question generation). Only meaningful while
	// ParseStatus == "finalizing"; defaults to 0 in any terminal state.
	PendingSubtasksCount int `json:"pending_subtasks_count" gorm:"type:int;not null;default:0"`
	// Summary status for async summary generation
	SummaryStatus string `json:"summary_status"     gorm:"type:varchar(32);default:none"`
	// Enable status of the knowledge
	EnableStatus string `json:"enable_status"`
	// ID of the embedding model
	EmbeddingModelID string `json:"embedding_model_id"`
	// File name of the knowledge
	FileName string `json:"file_name"`
	// File type of the knowledge
	FileType string `json:"file_type"`
	// File size of the knowledge
	FileSize int64 `json:"file_size"`
	// File hash of the knowledge
	FileHash string `json:"file_hash"`
	// File path of the knowledge
	FilePath string `json:"file_path"`
	// Storage size of the knowledge
	StorageSize int64 `json:"storage_size"`
	// Metadata of the knowledge
	Metadata JSON `json:"metadata"           gorm:"type:json"`
	// Last FAQ import result (for FAQ type knowledge only)
	LastFAQImportResult JSON `json:"last_faq_import_result" gorm:"type:json"`
	// Creation time of the knowledge
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the knowledge
	UpdatedAt time.Time `json:"updated_at"`
	// Processed time of the knowledge
	ProcessedAt *time.Time `json:"processed_at"`
	// Error message of the knowledge
	ErrorMessage string `json:"error_message"`
	// Deletion time of the knowledge
	DeletedAt gorm.DeletedAt `json:"deleted_at"         gorm:"index"`
	// Knowledge base name (not stored in database, populated on query)
	KnowledgeBaseName string `json:"knowledge_base_name" gorm:"-"`
}

// GetMetadata returns the metadata as a map[string]string.
func (k *Knowledge) GetMetadata() map[string]string {
	metadata := make(map[string]string)
	if len(k.Metadata) == 0 {
		return metadata
	}
	metadataMap, err := k.Metadata.Map()
	if err != nil {
		return nil
	}
	for k, v := range metadataMap {
		metadata[k] = fmt.Sprintf("%v", v)
	}
	return metadata
}

// BeforeCreate hook generates a UUID for new Knowledge entities before they are created.
func (k *Knowledge) BeforeCreate(tx *gorm.DB) (err error) {
	if k.ID == "" {
		k.ID = uuid.New().String()
	}
	return nil
}

// KnowledgeSearchScope defines a workspace knowledge-base search scope.
type KnowledgeSearchScope struct {
	TenantID uint64
	KBID     string
}

// SetLastFAQImportResult sets FAQ import result to the dedicated field.
func (k *Knowledge) SetLastFAQImportResult(result *FAQImportResult) error {
	if result == nil {
		k.LastFAQImportResult = nil
		return nil
	}
	jsonValue, err := result.ToJSON()
	if err != nil {
		return err
	}
	k.LastFAQImportResult = jsonValue
	return nil
}

// GetLastFAQImportResult parses and returns FAQ import result from the dedicated field.
func (k *Knowledge) GetLastFAQImportResult() (*FAQImportResult, error) {
	if len(k.LastFAQImportResult) == 0 {
		return nil, nil
	}
	var result FAQImportResult
	if err := json.Unmarshal(k.LastFAQImportResult, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

const metadataKeyProcessOverrides = "process_overrides"

// ProcessOverrides parses process config overrides from knowledge metadata.
func (k *Knowledge) ProcessOverrides() (*KnowledgeProcessOverrides, error) {
	if k == nil || len(k.Metadata) == 0 {
		return nil, nil
	}
	metadataMap, err := k.Metadata.Map()
	if err != nil {
		return nil, err
	}
	raw, ok := metadataMap[metadataKeyProcessOverrides]
	if !ok || raw == nil {
		return nil, nil
	}
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var overrides KnowledgeProcessOverrides
	if err := json.Unmarshal(bytes, &overrides); err != nil {
		return nil, err
	}
	return &overrides, nil
}

// SetProcessOverrides merges process config overrides into knowledge metadata.
func (k *Knowledge) SetProcessOverrides(o *KnowledgeProcessOverrides) error {
	if k == nil {
		return nil
	}
	metadataMap, err := k.Metadata.Map()
	if err != nil {
		return err
	}
	if o == nil {
		delete(metadataMap, metadataKeyProcessOverrides)
	} else {
		bytes, err := json.Marshal(o)
		if err != nil {
			return err
		}
		var value interface{}
		if err := json.Unmarshal(bytes, &value); err != nil {
			return err
		}
		metadataMap[metadataKeyProcessOverrides] = value
	}
	bytes, err := json.Marshal(metadataMap)
	if err != nil {
		return err
	}
	k.Metadata = JSON(bytes)
	return nil
}

// KnowledgeCheckParams defines parameters used to check if knowledge already exists.
type KnowledgeCheckParams struct {
	// File parameters
	FileName string
	FileSize int64
	FileHash string
	// URL parameters
	URL string
	// Text passage parameters
	Passages []string
	// Knowledge type
	Type string
}
