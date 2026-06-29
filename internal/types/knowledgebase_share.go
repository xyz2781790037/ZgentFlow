package types

import "time"

const (
	KBRoleReader = "reader"
	KBRoleWriter = "writer"
	KBRoleAdmin  = "admin"
	KBRoleOwner  = "owner"

	KBJoinPending  = "pending"
	KBJoinApproved = "approved"
	KBJoinRejected = "rejected"
)

// KnowledgeBaseMember grants one user access to one knowledge base. The owner
// is stored on knowledge_bases.owner_user_id and is not duplicated here.
type KnowledgeBaseMember struct {
	ID              uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	KnowledgeBaseID string    `json:"knowledge_base_id" gorm:"type:varchar(36);not null;uniqueIndex:idx_kb_member"`
	UserID          string    `json:"user_id" gorm:"type:varchar(36);not null;uniqueIndex:idx_kb_member"`
	Role            string    `json:"role" gorm:"type:varchar(16);not null"`
	JoinedAt        time.Time `json:"joined_at"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (KnowledgeBaseMember) TableName() string { return "knowledge_base_members" }

type KnowledgeBaseJoinRequest struct {
	ID              string     `json:"id" gorm:"type:varchar(36);primaryKey"`
	KnowledgeBaseID string     `json:"knowledge_base_id" gorm:"type:varchar(36);not null;index"`
	UserID          string     `json:"user_id" gorm:"type:varchar(36);not null;index"`
	Status          string     `json:"status" gorm:"type:varchar(16);not null;index"`
	ReviewedBy      *string    `json:"reviewed_by,omitempty" gorm:"type:varchar(36)"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (KnowledgeBaseJoinRequest) TableName() string { return "knowledge_base_join_requests" }

type KnowledgeBaseAuditLog struct {
	ID               uint64    `json:"id" gorm:"primaryKey;autoIncrement"`
	KnowledgeBaseID  string    `json:"knowledge_base_id" gorm:"type:varchar(36);not null;index"`
	ActorUserID      string    `json:"actor_user_id" gorm:"type:varchar(36);not null;index"`
	Action           string    `json:"action" gorm:"type:varchar(64);not null;index"`
	TargetUserID     string    `json:"target_user_id,omitempty" gorm:"type:varchar(36)"`
	TargetResourceID string    `json:"target_resource_id,omitempty" gorm:"type:varchar(64)"`
	Details          JSON      `json:"details,omitempty" gorm:"type:jsonb"`
	CreatedAt        time.Time `json:"created_at"`
}

func (KnowledgeBaseAuditLog) TableName() string { return "knowledge_base_audit_logs" }

// KnowledgeBaseAccess is caller-specific metadata returned with KB responses.
type KnowledgeBaseAccess struct {
	Role          string `json:"role"`
	Shared        bool   `json:"shared"`
	OwnerUsername string `json:"owner_username"`
}

func KBRoleRank(role string) int {
	switch role {
	case KBRoleOwner:
		return 40
	case KBRoleAdmin:
		return 30
	case KBRoleWriter:
		return 20
	case KBRoleReader:
		return 10
	default:
		return 0
	}
}
