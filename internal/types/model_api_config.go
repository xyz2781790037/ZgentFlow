package types

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/xyz2781790037/ZealRAG/internal/utils"
	"gorm.io/gorm"
)

// ModelAPIKey is encrypted at rest and decrypted when loaded by GORM.
// It is never serialized by API response DTOs.
type ModelAPIKey string

func (k ModelAPIKey) Value() (driver.Value, error) {
	plain := string(k)
	if plain == "" {
		return "", nil
	}
	key := utils.GetAESKey()
	if key == nil {
		return nil, fmt.Errorf("SYSTEM_AES_KEY is not configured")
	}
	encrypted, err := utils.EncryptAESGCM(plain, key)
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

func (k *ModelAPIKey) Scan(value any) error {
	if value == nil {
		*k = ""
		return nil
	}
	var stored string
	switch typed := value.(type) {
	case string:
		stored = typed
	case []byte:
		stored = string(typed)
	default:
		return fmt.Errorf("unsupported model API key database value %T", value)
	}
	plain, ok := utils.DecryptStoredSecretLenient(stored)
	if !ok {
		*k = ""
		return nil
	}
	*k = ModelAPIKey(plain)
	return nil
}

// ModelAPIConfig is a reusable API key grouped by model provider.
// Base URLs remain model-specific and are deliberately not stored here.
type ModelAPIConfig struct {
	ID         string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID   uint64         `json:"tenant_id" gorm:"not null;index"`
	Name       string         `json:"name" gorm:"type:varchar(100);not null"`
	Provider   string         `json:"provider" gorm:"type:varchar(32);not null"`
	APIKey     ModelAPIKey    `json:"-" gorm:"column:api_key;type:text;not null"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `json:"-" gorm:"index"`
	ModelCount int64          `json:"-" gorm:"-"`
}

func (ModelAPIConfig) TableName() string {
	return "model_api_configs"
}

func (c *ModelAPIConfig) BeforeCreate(*gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
