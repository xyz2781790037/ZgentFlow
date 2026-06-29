package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRetrieverEngineMappingOnlyIncludesPostgresHybridCapabilities(t *testing.T) {
	mapping := GetRetrieverEngineMapping()

	assert.Len(t, mapping, 1)
	assert.Contains(t, mapping["postgres"], RetrieverEngineParams{
		RetrieverType:       KeywordsRetrieverType,
		RetrieverEngineType: PostgresRetrieverEngineType,
	})
	assert.Contains(t, mapping["postgres"], RetrieverEngineParams{
		RetrieverType:       VectorRetrieverType,
		RetrieverEngineType: PostgresRetrieverEngineType,
	})
}
