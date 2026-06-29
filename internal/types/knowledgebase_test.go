package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKnowledgeBaseClearsLegacyVectorStoreBinding(t *testing.T) {
	legacyStoreID := "legacy-store"
	kb := KnowledgeBase{VectorStoreID: &legacyStoreID}
	kb.Normalize()
	assert.Nil(t, kb.VectorStoreID)
}
