package types

// Builtin answer mode IDs.
const (
	// BuiltinQuickAnswerID is the ID for the built-in quick RAG answer mode.
	BuiltinQuickAnswerID = "builtin-quick-answer"
)

// AgentMode constants for agent running mode
const (
	// AgentModeQuickAnswer is the RAG mode for quick Q&A
	AgentModeQuickAnswer = "quick-answer"
)

// AnswerMode is the runtime definition of one built-in answer mode.
type AnswerMode struct {
	ID          string           `yaml:"id" json:"id"`
	Name        string           `yaml:"name" json:"name"`
	Description string           `yaml:"description" json:"description"`
	Config      AnswerModeConfig `yaml:"config" json:"config"`
}

// AnswerModeConfig represents the configuration of a answer mode
type AnswerModeConfig struct {
	// ===== Basic Settings =====
	// Agent mode: "quick-answer" for RAG mode, "smart-reasoning" for ReAct agent mode
	AgentMode string `yaml:"agent_mode" json:"agent_mode"`
	// System prompt for the agent (unified prompt, uses web_search_status placeholder for dynamic behavior)
	SystemPrompt string `yaml:"system_prompt" json:"system_prompt"`
	// SystemPromptID references a template ID in prompt_templates/ YAML files.
	// If set and SystemPrompt is empty, the template content will be resolved at startup.
	SystemPromptID string `yaml:"system_prompt_id" json:"system_prompt_id,omitempty"`
	// Context template for normal mode (how to format retrieved chunks)
	ContextTemplate string `yaml:"context_template" json:"context_template"`
	// ContextTemplateID references a template ID in prompt_templates/ YAML files.
	// If set and ContextTemplate is empty, the template content will be resolved at startup.
	ContextTemplateID string `yaml:"context_template_id" json:"context_template_id,omitempty"`

	// ===== Model Settings =====
	// Model ID to use for conversations
	ModelID string `yaml:"model_id" json:"model_id"`
	// ReRank model ID for retrieval
	RerankModelID string `yaml:"rerank_model_id" json:"rerank_model_id"`
	// Temperature for LLM (0-1)
	Temperature float64 `yaml:"temperature" json:"temperature"`
	// Maximum completion tokens (only for normal mode)
	MaxCompletionTokens int `yaml:"max_completion_tokens" json:"max_completion_tokens"`
	// Whether to enable thinking mode (for models that support extended thinking)
	Thinking *bool `yaml:"thinking" json:"thinking"`

	// ===== Agent Mode Settings =====
	// Maximum iterations for ReAct loop (only for agent type)
	MaxIterations int `yaml:"max_iterations" json:"max_iterations"`
	// Timeout for a single LLM call in seconds (0 = use global default)
	LLMCallTimeout int `yaml:"llm_call_timeout" json:"llm_call_timeout,omitempty"`
	// Allowed tools (only for agent type)
	AllowedTools []string `yaml:"allowed_tools" json:"allowed_tools"`
	// ===== Knowledge Base Settings =====
	// Knowledge base selection mode: "all" = all KBs, "selected" = specific KBs, "none" = no KB
	KBSelectionMode string `yaml:"kb_selection_mode" json:"kb_selection_mode"`
	// Associated knowledge base IDs (only used when KBSelectionMode is "selected")
	KnowledgeBases []string `yaml:"knowledge_bases" json:"knowledge_bases"`
	// Whether to retrieve knowledge base only when explicitly mentioned with @ (default: false)
	// When true, knowledge base retrieval only happens if user explicitly mentions KB/files with @
	// When false, knowledge base retrieval happens according to KBSelectionMode
	RetrieveKBOnlyWhenMentioned bool `yaml:"retrieve_kb_only_when_mentioned" json:"retrieve_kb_only_when_mentioned"`

	// Whether to retain retrieval history across turns
	RetainRetrievalHistory bool `yaml:"retain_retrieval_history" json:"retain_retrieval_history"`

	// ===== Image Upload / Multimodal Settings =====
	// Whether image upload is enabled for this agent (default: false)
	ImageUploadEnabled bool `yaml:"image_upload_enabled" json:"image_upload_enabled"`
	// VLM model ID for image analysis (optional, falls back to workspace default)
	VLMModelID string `yaml:"vlm_model_id" json:"vlm_model_id"`
	// ===== FAQ Strategy Settings =====
	// Whether FAQ priority strategy is enabled (FAQ answers prioritized over document chunks)
	FAQPriorityEnabled bool `yaml:"faq_priority_enabled" json:"faq_priority_enabled"`
	// FAQ direct answer threshold - if similarity > this value, use FAQ answer directly
	FAQDirectAnswerThreshold float64 `yaml:"faq_direct_answer_threshold" json:"faq_direct_answer_threshold"`
	// FAQ score boost multiplier - FAQ results score multiplied by this factor
	FAQScoreBoost float64 `yaml:"faq_score_boost" json:"faq_score_boost"`

	// ===== Web Search Settings =====
	// Whether web search is enabled
	WebSearchEnabled bool `yaml:"web_search_enabled" json:"web_search_enabled"`
	// Maximum web search results
	WebSearchMaxResults int `yaml:"web_search_max_results" json:"web_search_max_results"`
	// WebSearchProviderID references a specific WebSearchProviderEntity.
	// If empty, the workspace default provider (is_default=true) is used.
	WebSearchProviderID string `yaml:"web_search_provider_id" json:"web_search_provider_id,omitempty"`
	// Whether to auto-fetch full page content for reranked web search results
	WebFetchEnabled bool `yaml:"web_fetch_enabled" json:"web_fetch_enabled"`
	// Max number of pages to fetch after rerank (default: 3)
	WebFetchTopN int `yaml:"web_fetch_top_n" json:"web_fetch_top_n,omitempty"`

	// ===== Multi-turn Conversation Settings =====
	// Whether multi-turn conversation is enabled
	MultiTurnEnabled bool `yaml:"multi_turn_enabled" json:"multi_turn_enabled"`
	// Number of history turns to keep in context
	HistoryTurns int `yaml:"history_turns" json:"history_turns"`

	// ===== Retrieval Strategy Settings (for both modes) =====
	// Embedding/Vector retrieval top K
	EmbeddingTopK int `yaml:"embedding_top_k" json:"embedding_top_k"`
	// Keyword retrieval threshold
	KeywordThreshold float64 `yaml:"keyword_threshold" json:"keyword_threshold"`
	// Vector retrieval threshold
	VectorThreshold float64 `yaml:"vector_threshold" json:"vector_threshold"`
	// Rerank top K
	RerankTopK int `yaml:"rerank_top_k" json:"rerank_top_k"`
	// Rerank threshold
	RerankThreshold float64 `yaml:"rerank_threshold" json:"rerank_threshold"`

	// ===== Advanced Settings (mainly for normal mode) =====
	// Whether to enable query expansion
	EnableQueryExpansion bool `yaml:"enable_query_expansion" json:"enable_query_expansion"`
	// Whether to enable query rewrite for multi-turn conversations
	EnableRewrite bool `yaml:"enable_rewrite" json:"enable_rewrite"`
	// Rewrite prompt system message
	RewritePromptSystem string `yaml:"rewrite_prompt_system" json:"rewrite_prompt_system"`
	// Rewrite prompt user message template
	RewritePromptUser string `yaml:"rewrite_prompt_user" json:"rewrite_prompt_user"`
	// Dedicated chat model ID for the query-understanding (rewrite + intent) step.
	// When empty, the main conversation ModelID is used as a fallback.
	QueryUnderstandModelID string `yaml:"query_understand_model_id" json:"query_understand_model_id,omitempty"`
	// Fallback strategy: "fixed" for fixed response, "model" for model generation
	FallbackStrategy string `yaml:"fallback_strategy" json:"fallback_strategy"`
	// Fixed fallback response (when FallbackStrategy is "fixed")
	FallbackResponse string `yaml:"fallback_response" json:"fallback_response"`
	// Fallback prompt (when FallbackStrategy is "model")
	FallbackPrompt string `yaml:"fallback_prompt" json:"fallback_prompt"`
	// IntentPrompts holds per-intent system prompt overrides for non-retrieval
	// intents (greeting, chitchat, etc.). Empty values fall back to templates
	// under config/prompt_templates/intent_prompts.yaml.
	IntentPrompts map[string]string `yaml:"intent_prompts" json:"intent_prompts,omitempty"`
}

// EnsureDefaults sets default values for the quick answer mode.
func (a *AnswerMode) EnsureDefaults() {
	if a == nil {
		return
	}
	if a.Config.Temperature < 0 {
		a.Config.Temperature = 0.7
	}
	if a.Config.WebSearchMaxResults == 0 {
		a.Config.WebSearchMaxResults = 5
	}
	if a.Config.HistoryTurns == 0 {
		a.Config.HistoryTurns = 5
	}
	// Retrieval strategy defaults
	if a.Config.EmbeddingTopK == 0 {
		a.Config.EmbeddingTopK = 10
	}
	if a.Config.KeywordThreshold == 0 {
		a.Config.KeywordThreshold = 0.3
	}
	if a.Config.VectorThreshold == 0 {
		a.Config.VectorThreshold = 0.5
	}
	if a.Config.RerankTopK == 0 {
		a.Config.RerankTopK = 5
	}
	// Advanced settings defaults
	if a.Config.FallbackStrategy == "" {
		a.Config.FallbackStrategy = "model"
	}
	if a.Config.MaxCompletionTokens == 0 {
		a.Config.MaxCompletionTokens = 2048
	}
}
