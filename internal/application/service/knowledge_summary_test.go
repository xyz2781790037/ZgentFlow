package service

import (
	"errors"
	"testing"
)

// TestCheckSufficientSummaryContent verifies the pure-text gate that prevents
// empty or too-short documents from calling the summary model.
func TestCheckSufficientSummaryContent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantError bool
	}{
		{
			name:      "empty content rejected",
			content:   "",
			wantError: true,
		},
		{
			name:      "only whitespace rejected",
			content:   "   \n\n\t  ",
			wantError: true,
		},
		{
			name:      "below threshold rejected",
			content:   "hi",
			wantError: true,
		},
		{
			name:      "short legitimate note above threshold accepted",
			content:   "Meeting at 3pm tomorrow.",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkSufficientSummaryContent(tt.content)
			if tt.wantError {
				if err == nil {
					t.Errorf("expected errInsufficientSummaryContent, got nil")
					return
				}
				if !errors.Is(err, errInsufficientSummaryContent) {
					t.Errorf("expected errInsufficientSummaryContent sentinel, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}
			}
		})
	}
}

// TestCheckSufficientSummaryContent_ThresholdOverride verifies that the
// threshold remains configurable without a rebuild.
func TestCheckSufficientSummaryContent_ThresholdOverride(t *testing.T) {
	content := "Meeting at 3pm." // 15 runes

	originalThreshold := minTextContentRunes
	t.Cleanup(func() { minTextContentRunes = originalThreshold })

	// With default threshold (10), this content passes.
	if err := checkSufficientSummaryContent(content); err != nil {
		t.Fatalf("default threshold: expected pass, got %v", err)
	}

	// With a tighter threshold (50), the same content is rejected.
	minTextContentRunes = 50
	err := checkSufficientSummaryContent(content)
	if !errors.Is(err, errInsufficientSummaryContent) {
		t.Fatalf("tightened threshold: expected errInsufficientSummaryContent, got %v", err)
	}
}
