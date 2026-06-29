package interfaces

import (
	"github.com/xyz2781790037/ZealRAG/internal/types"
)

// Metrics defines interface for computing evaluation metrics
type Metrics interface {
	// Compute calculates metric score based on input data
	Compute(metricInput *types.MetricInput) float64
}
