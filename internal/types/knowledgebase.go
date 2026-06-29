package types

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"gorm.io/gorm"
)

// KnowledgeBaseType represents the type of the knowledge base
const (
	// KnowledgeBaseTypeDocument represents the document knowledge base type
	KnowledgeBaseTypeDocument = "document"
	KnowledgeBaseTypeFAQ      = "faq"
	// DefaultKnowledgeBaseMaxFileSizeMB preserves the original upload limit
	// for existing and newly created knowledge bases unless users change it.
	DefaultKnowledgeBaseMaxFileSizeMB int64 = 50
)

// FAQIndexMode represents the FAQ index mode: only index questions or index questions and answers
type FAQIndexMode string

const (
	// FAQIndexModeQuestionOnly only index questions and similar questions
	FAQIndexModeQuestionOnly FAQIndexMode = "question_only"
	// FAQIndexModeQuestionAnswer index questions and answers together
	FAQIndexModeQuestionAnswer FAQIndexMode = "question_answer"
)

// FAQQuestionIndexMode represents the FAQ question index mode: index together or index separately
type FAQQuestionIndexMode string

const (
	// FAQQuestionIndexModeCombined index questions and similar questions together
	FAQQuestionIndexModeCombined FAQQuestionIndexMode = "combined"
	// FAQQuestionIndexModeSeparate index questions and similar questions separately
	FAQQuestionIndexModeSeparate FAQQuestionIndexMode = "separate"
)

// KnowledgeBase represents a knowledge base entity
type KnowledgeBase struct {
	// Unique identifier of the knowledge base
	ID string `yaml:"id"                      json:"id"                      gorm:"type:varchar(36);primaryKey"`
	// Name of the knowledge base
	Name string `yaml:"name"                    json:"name"`
	// Type of the knowledge base (document, faq, etc.)
	Type string `yaml:"type"                    json:"type"                    gorm:"type:varchar(32);default:'document'"`
	// Whether this knowledge base is temporary (ephemeral) and should be hidden from UI
	IsTemporary bool `yaml:"is_temporary"            json:"is_temporary"            gorm:"default:false"`
	// Description of the knowledge base
	Description string `yaml:"description"             json:"description"`
	// MaxFileSizeMB limits the size of each newly uploaded file in this knowledge base.
	MaxFileSizeMB int64 `yaml:"max_file_size_mb"        json:"max_file_size_mb"        gorm:"column:max_file_size_mb;not null;default:50"`
	// Tenant ID
	TenantID uint64 `yaml:"tenant_id"               json:"tenant_id"`
	// OwnerUserID is the mutable authorization owner. TenantID remains the
	// immutable storage partition used by documents, chunks and vectors.
	OwnerUserID string `yaml:"owner_user_id" json:"owner_user_id" gorm:"type:varchar(36);index"`
	// SharingEnabled controls whether the current invite code accepts new requests.
	SharingEnabled bool `yaml:"sharing_enabled" json:"sharing_enabled" gorm:"not null;default:false"`
	// InviteCode is returned only to the owner by sharing-management endpoints.
	InviteCode string `yaml:"-" json:"-" gorm:"type:varchar(32);uniqueIndex"`
	// Chunking configuration
	ChunkingConfig ChunkingConfig `yaml:"chunking_config"         json:"chunking_config"         gorm:"type:json"`
	// Image processing configuration
	ImageProcessingConfig ImageProcessingConfig `yaml:"image_processing_config" json:"image_processing_config" gorm:"type:json"`
	// ID of the embedding model
	EmbeddingModelID string `yaml:"embedding_model_id"      json:"embedding_model_id"`
	// Summary model ID
	SummaryModelID string `yaml:"summary_model_id"        json:"summary_model_id"`
	// VLM config
	VLMConfig VLMConfig `yaml:"vlm_config"              json:"vlm_config"              gorm:"type:json"`
	// VectorStoreID is a database-only compatibility field for pre-ZealRAG rows.
	VectorStoreID *string `yaml:"-" json:"-" swaggerignore:"true" gorm:"column:vector_store_id;type:varchar(36);<-:create"`
	// FAQConfig stores FAQ specific configuration such as indexing strategy
	FAQConfig *FAQConfig `yaml:"faq_config"              json:"faq_config"              gorm:"column:faq_config;type:json"`
	// QuestionGenerationConfig stores question generation configuration for document knowledge bases
	QuestionGenerationConfig *QuestionGenerationConfig `yaml:"question_generation_config" json:"question_generation_config" gorm:"column:question_generation_config;type:json"`
	// IndexingStrategy controls which indexing pipelines are active for this knowledge base.
	// Pipelines: vector search and keyword search.
	IndexingStrategy IndexingStrategy `yaml:"indexing_strategy"       json:"indexing_strategy"       gorm:"column:indexing_strategy;type:json"`
	// ActiveGeneration is the only fully published vector build.
	ActiveGeneration   int64      `yaml:"active_generation"    json:"active_generation"    gorm:"column:active_generation;->"`
	BuildingGeneration *int64     `yaml:"building_generation"  json:"building_generation,omitempty" gorm:"column:building_generation;->"`
	RebuildStageID     string     `yaml:"-"                    json:"-"                    gorm:"column:rebuild_stage_id;->"`
	RebuildStatus      string     `yaml:"rebuild_status"       json:"rebuild_status"       gorm:"column:rebuild_status;->"`
	RebuildError       string     `yaml:"rebuild_error"        json:"rebuild_error"        gorm:"column:rebuild_error;->"`
	RebuildStartedAt   *time.Time `yaml:"rebuild_started_at"   json:"rebuild_started_at,omitempty" gorm:"column:rebuild_started_at;->"`
	RebuildCompletedAt *time.Time `yaml:"rebuild_completed_at" json:"rebuild_completed_at,omitempty" gorm:"column:rebuild_completed_at;->"`
	// IsPinned and PinnedAt are computed per-caller from user_kb_pins
	// (see migration 000050). They used to be stored on the row itself,
	// which made pinning a tenant-wide ordering decision gated behind
	// the kb-edit RBAC guard. The columns are still present in legacy
	// schemas for rollback safety but are no longer read or written by
	// the application — both fields are tagged `gorm:"-"` so GORM
	// ignores them on every CRUD call and the list handler stamps them
	// after enriching with the caller's pin set.
	IsPinned bool `yaml:"is_pinned"               json:"is_pinned"               gorm:"-"`
	// PinnedAt records when the current caller pinned this knowledge
	// base; nil when they have not.
	PinnedAt *time.Time `yaml:"pinned_at"               json:"pinned_at"               gorm:"-"`
	// Creation time of the knowledge base
	CreatedAt time.Time `yaml:"created_at"              json:"created_at"`
	// Last updated time of the knowledge base
	UpdatedAt time.Time `yaml:"updated_at"              json:"updated_at"`
	// Deletion time of the knowledge base
	DeletedAt gorm.DeletedAt `yaml:"deleted_at"              json:"deleted_at"              gorm:"index"`
	// Knowledge count (not stored in database, calculated on query)
	KnowledgeCount int64 `yaml:"knowledge_count"         json:"knowledge_count"         gorm:"-"`
	// Chunk count (not stored in database, calculated on query)
	ChunkCount int64 `yaml:"chunk_count"             json:"chunk_count"             gorm:"-"`
	// IsProcessing indicates if there is a processing import task (for FAQ type knowledge bases)
	IsProcessing bool `yaml:"is_processing"           json:"is_processing"           gorm:"-"`
	// ProcessingCount indicates the number of knowledge items being processed (for document type knowledge bases)
	ProcessingCount int64 `yaml:"processing_count"        json:"processing_count"        gorm:"-"`
	// Caller-specific sharing fields, populated at the API boundary.
	AccessRole    string `yaml:"access_role" json:"access_role" gorm:"-"`
	IsShared      bool   `yaml:"is_shared" json:"is_shared" gorm:"-"`
	OwnerUsername string `yaml:"owner_username" json:"owner_username" gorm:"-"`
}

// KnowledgeBaseConfig represents the knowledge base configuration
type KnowledgeBaseConfig struct {
	// MaxFileSizeMB is optional so older update clients preserve the current value.
	MaxFileSizeMB *int64 `yaml:"max_file_size_mb,omitempty" json:"max_file_size_mb,omitempty"`
	// Chunking configuration
	ChunkingConfig ChunkingConfig `yaml:"chunking_config"         json:"chunking_config"`
	// Image processing configuration
	ImageProcessingConfig ImageProcessingConfig `yaml:"image_processing_config" json:"image_processing_config"`
	// FAQ configuration (only for FAQ type knowledge bases)
	FAQConfig *FAQConfig `yaml:"faq_config"              json:"faq_config"`
	// IndexingStrategy controls which indexing pipelines are active.
	// nil means "no change" when updating (preserves existing strategy).
	IndexingStrategy *IndexingStrategy `yaml:"indexing_strategy"       json:"indexing_strategy"`
}

// ParserEngineRule maps a set of file types to a specific parser engine.
type ParserEngineRule struct {
	FileTypes []string `yaml:"file_types" json:"file_types"`
	Engine    string   `yaml:"engine"     json:"engine"`
}

// ChunkingConfig represents the document splitting configuration
type ChunkingConfig struct {
	// Chunk size
	ChunkSize int `yaml:"chunk_size"    json:"chunk_size"`
	// Chunk overlap
	ChunkOverlap int `yaml:"chunk_overlap" json:"chunk_overlap"`
	// Separators
	Separators []string `yaml:"separators"    json:"separators"`
	// ParserEngineRules configures which parser engine to use for each file type.
	// When empty, the builtin engine is used for all types.
	ParserEngineRules []ParserEngineRule `yaml:"parser_engine_rules,omitempty" json:"parser_engine_rules,omitempty"`
	// EnableParentChild enables two-level parent-child chunking strategy.
	// When enabled, large parent chunks provide context while small child chunks
	// are used for vector matching. Retrieval matches on child but returns parent content.
	EnableParentChild bool `yaml:"enable_parent_child,omitempty" json:"enable_parent_child,omitempty"`
	// ParentChunkSize is the size of parent chunks (default: 4096).
	// Only used when EnableParentChild is true.
	ParentChunkSize int `yaml:"parent_chunk_size,omitempty" json:"parent_chunk_size,omitempty"`
	// ChildChunkSize is the size of child chunks used for embedding (default: 384).
	// Only used when EnableParentChild is true.
	ChildChunkSize int `yaml:"child_chunk_size,omitempty" json:"child_chunk_size,omitempty"`
	// Strategy selects the adaptive chunking tier. Empty / "legacy" preserves
	// the historical recursive splitter; "auto" lets a profiler pick between
	// heading-aware, heuristic and recursive tiers; "heading" / "heuristic" /
	// "recursive" pin the tier explicitly.
	Strategy string `yaml:"strategy,omitempty" json:"strategy,omitempty"`
	// TokenLimit caps chunk size in approximate tokens. 0 = use ChunkSize
	// as a character count.
	TokenLimit int `yaml:"token_limit,omitempty" json:"token_limit,omitempty"`
	// Languages hints the heuristic patterns. Empty = auto-detect from content.
	// Examples: ["de"], ["en", "zh"].
	Languages []string `yaml:"languages,omitempty" json:"languages,omitempty"`
}

// ResolveParserEngine returns the engine name for the given file type
// based on the configured rules. Returns empty string (builtin) when
// no rule matches.
func (c ChunkingConfig) ResolveParserEngine(fileType string) string {
	for _, rule := range c.ParserEngineRules {
		for _, ft := range rule.FileTypes {
			if ft == fileType {
				return rule.Engine
			}
		}
	}
	return ""
}

// ImageProcessingConfig represents the image processing configuration
type ImageProcessingConfig struct {
	// Model ID
	ModelID string `yaml:"model_id" json:"model_id"`
}

// Value implements the driver.Valuer interface, used to convert ChunkingConfig to database value
func (c ChunkingConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface, used to convert database value to ChunkingConfig
func (c *ChunkingConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// Value implements the driver.Valuer interface, used to convert ImageProcessingConfig to database value
func (c ImageProcessingConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface, used to convert database value to ImageProcessingConfig
func (c *ImageProcessingConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// VLMConfig represents the VLM configuration
type VLMConfig struct {
	Enabled bool   `yaml:"enabled"  json:"enabled"`
	ModelID string `yaml:"model_id" json:"model_id"`

	// 兼容老版本
	// Model Name
	ModelName string `yaml:"model_name" json:"model_name"`
	// Base URL
	BaseURL string `yaml:"base_url" json:"base_url"`
	// API Key
	APIKey string `yaml:"api_key" json:"api_key"`
	// Interface Type: "ollama" or "openai"
	InterfaceType string `yaml:"interface_type" json:"interface_type"`
}

// IsEnabled 判断多模态是否启用（兼容新老版本）
// 新版本：Enabled && ModelID != ""
// 老版本：ModelName != "" && BaseURL != ""
func (c VLMConfig) IsEnabled() bool {
	// 新版本配置
	if c.Enabled && c.ModelID != "" {
		return true
	}
	// 兼容老版本配置
	if c.ModelName != "" && c.BaseURL != "" {
		return true
	}
	return false
}

// QuestionGenerationConfig represents the question generation configuration for document knowledge bases
// When enabled, the system will use LLM to generate questions for each chunk during document parsing
// These generated questions will be indexed separately to improve recall
type QuestionGenerationConfig struct {
	Enabled bool `yaml:"enabled"  json:"enabled"`
	// Number of questions to generate per chunk (default: 3, max: 10)
	QuestionCount int `yaml:"question_count" json:"question_count"`
}

// Value implements the driver.Valuer interface
func (c QuestionGenerationConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface
func (c *QuestionGenerationConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// Value implements the driver.Valuer interface, used to convert VLMConfig to database value
func (c VLMConfig) Value() (driver.Value, error) {
	return json.Marshal(c)
}

// Scan implements the sql.Scanner interface, used to convert database value to VLMConfig
func (c *VLMConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, c)
}

// FAQConfig 存储 FAQ 知识库的特有配置
type FAQConfig struct {
	IndexMode         FAQIndexMode         `yaml:"index_mode"          json:"index_mode"`
	QuestionIndexMode FAQQuestionIndexMode `yaml:"question_index_mode" json:"question_index_mode"`
}

// Value implements driver.Valuer
func (f FAQConfig) Value() (driver.Value, error) {
	return json.Marshal(f)
}

// Scan implements sql.Scanner
func (f *FAQConfig) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(b, f)
}

// EnsureDefaults 确保类型与配置具备默认值
func (kb *KnowledgeBase) EnsureDefaults() {
	if kb == nil {
		return
	}
	if kb.Type == "" {
		kb.Type = KnowledgeBaseTypeDocument
	}
	if kb.MaxFileSizeMB <= 0 {
		kb.MaxFileSizeMB = DefaultKnowledgeBaseMaxFileSizeMB
	}
	// Clear type-specific configs that don't belong
	if kb.Type != KnowledgeBaseTypeFAQ {
		kb.FAQConfig = nil
	}
	// Set defaults for FAQ
	if kb.Type == KnowledgeBaseTypeFAQ {
		if kb.FAQConfig == nil {
			kb.FAQConfig = &FAQConfig{
				IndexMode:         FAQIndexModeQuestionAnswer,
				QuestionIndexMode: FAQQuestionIndexModeCombined,
			}
			return
		}
		if kb.FAQConfig.IndexMode == "" {
			kb.FAQConfig.IndexMode = FAQIndexModeQuestionAnswer
		}
		if kb.FAQConfig.QuestionIndexMode == "" {
			kb.FAQConfig.QuestionIndexMode = FAQQuestionIndexModeCombined
		}
	}

	// Ensure IndexingStrategy has defaults.
	// For existing rows where indexing_strategy is NULL, GORM Scan() returns
	// DefaultIndexingStrategy() (vector+keyword=true). This block handles the
	// case where a fresh struct was created in-memory without touching DB.
	if kb.IndexingStrategy.IsZero() {
		kb.IndexingStrategy = DefaultIndexingStrategy()
	}
}

// KBCapabilities describes the functional features a knowledge base exposes.
// It is computed from the KB's configuration and type.
// and surfaced in the JSON representation of a KnowledgeBase so that the frontend
// can filter / enable / disable KB options based on what the selected agent type needs.
type KBCapabilities struct {
	// Vector means semantic (embedding) search is indexed.
	Vector bool `json:"vector"`
	// Keyword means BM25 / sparse keyword search is indexed.
	Keyword bool `json:"keyword"`
	// FAQ means the KB is a FAQ-type KB (Q/A pairs).
	FAQ bool `json:"faq"`
}

// Capabilities returns the computed capability flags for this KB.
// Safe to call on a nil KB (returns zero value).
func (kb *KnowledgeBase) Capabilities() KBCapabilities {
	if kb == nil {
		return KBCapabilities{}
	}
	return KBCapabilities{
		Vector:  kb.IsVectorEnabled(),
		Keyword: kb.IsKeywordEnabled(),
		FAQ:     kb.Type == KnowledgeBaseTypeFAQ,
	}
}

// MarshalJSON augments the default JSON encoding of KnowledgeBase with a computed
// `capabilities` field so clients (agent editor) can filter KBs by feature.
// It preserves all existing fields verbatim.
func (kb *KnowledgeBase) MarshalJSON() ([]byte, error) {
	type alias KnowledgeBase
	aux := struct {
		*alias
		Capabilities KBCapabilities `json:"capabilities"`
	}{
		alias:        (*alias)(kb),
		Capabilities: kb.Capabilities(),
	}
	return json.Marshal(aux)
}

// IsVectorEnabled checks if vector (semantic) search is enabled.
func (kb *KnowledgeBase) IsVectorEnabled() bool {
	return kb != nil && kb.IndexingStrategy.VectorEnabled
}

// IsKeywordEnabled checks if keyword (BM25) search is enabled.
func (kb *KnowledgeBase) IsKeywordEnabled() bool {
	return kb != nil && kb.IndexingStrategy.KeywordEnabled
}

// NeedsEmbeddingModel returns true if any enabled pipeline requires an embedding model.
// Currently only vector and keyword search need embeddings.
func (kb *KnowledgeBase) NeedsEmbeddingModel() bool {
	return kb != nil && kb.IndexingStrategy.NeedsEmbedding()
}

// IsMultimodalEnabled 判断多模态是否启用，由 VLMConfig.IsEnabled() 决定。
func (kb *KnowledgeBase) IsMultimodalEnabled() bool {
	if kb == nil {
		return false
	}
	return kb.VLMConfig.IsEnabled()
}

// Normalize clears obsolete vector-store bindings.
func (kb *KnowledgeBase) Normalize() {
	if kb == nil {
		return
	}
	kb.VectorStoreID = nil
}
