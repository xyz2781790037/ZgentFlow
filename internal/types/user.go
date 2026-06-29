package types

import (
	"time"

	"gorm.io/gorm"
)

// User is the internal actor attached to the single local workspace.
// It remains a database model because business rows reference creator IDs.
type User struct {
	ID           string         `json:"id"          gorm:"type:varchar(36);primaryKey"`
	Username     string         `json:"username"    gorm:"type:varchar(100);uniqueIndex;not null"`
	Email        string         `json:"email"       gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash string         `json:"-"           gorm:"type:varchar(255);not null"`
	Avatar       string         `json:"avatar"      gorm:"type:varchar(500)"`
	TenantID     uint64         `json:"tenant_id"   gorm:"index"`
	IsActive     bool           `json:"is_active"   gorm:"default:true"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at" gorm:"index"`
	Tenant       *Tenant        `json:"tenant,omitempty" gorm:"foreignKey:TenantID"`
}
