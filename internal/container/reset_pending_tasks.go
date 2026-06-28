package container

import (
	"context"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/application/repository"
	"github.com/xyz2781790037/ZealRAG/internal/logger"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/gorm"
)

const resetPendingStaleWindow = 30 * time.Minute

// resetPendingTasks resets the state of knowledge items stuck in processing
// due to an unexpected application restart.
//
// The Redis-backed active queue survives restart, but the *currently executing*
// task on the dead instance
// is lost — Asynq won't reschedule it until at-least-once retry kicks in,
// which can take minutes or never (e.g. if the deadline has passed). To bound
// the worst case we mark only "long-stale" rows failed: anything that hasn't
// been touched for 30 minutes is well past any reasonable in-flight window.
// Newer rows are left alone so we don't race a peer instance that's mid-process.
func resetPendingTasks(db *gorm.DB) {
	ctx := context.Background()
	spanRepo := repository.NewKnowledgeSpanRepository(db)
	staleCutoff := time.Now().Add(-resetPendingStaleWindow)

	// Cancel orphaned trace spans for knowledge rows we are about to mark
	// failed. resetPendingTasks does not touch asynq queues; this only
	// prevents the UI from showing duplicate running postprocess.*
	// subspans when a later retry also opens fresh spans.
	var stuckKnowledge []types.Knowledge
	if err := stuckKnowledgeParseQuery(db, staleCutoff).
		Select("id").Find(&stuckKnowledge).Error; err != nil {
		logger.Warnf(ctx, "resetPendingTasks: list stuck knowledge failed: %v", err)
	} else {
		for _, k := range stuckKnowledge {
			attempt, err := spanRepo.LatestAttempt(ctx, k.ID)
			if err != nil || attempt <= 0 {
				continue
			}
			if n, err := spanRepo.CancelAllOpenSpans(ctx, k.ID, attempt,
				"SERVER_RESTART", "task interrupted due to application restart"); err != nil {
				logger.Warnf(ctx, "resetPendingTasks: cancel spans for %s failed: %v", k.ID, err)
			} else if n > 0 {
				logger.Infof(ctx, "resetPendingTasks: cancelled %d open span(s) for knowledge %s attempt %d",
					n, k.ID, attempt)
			}
		}
	}

	// 1. Reset knowledge parsing tasks (including finalizing rows whose
	// enrichment subtasks were lost with the process).
	// Fresh query — reusing the *gorm.DB chain after Find() makes GORM emit
	// UPDATE ... FROM knowledges which PostgreSQL rejects (SQLSTATE 42712).
	result := stuckKnowledgeParseQuery(db, staleCutoff).Updates(map[string]interface{}{
		"parse_status":           types.ParseStatusFailed,
		"error_message":          "Task interrupted due to application restart",
		"pending_subtasks_count": 0,
	})
	if result.Error != nil {
		logger.Warnf(context.Background(), "Failed to reset pending knowledge tasks: %v", result.Error)
	} else if result.RowsAffected > 0 {
		logger.Infof(context.Background(), "Reset %d stuck knowledge parsing tasks to failed state", result.RowsAffected)
	}

	// 2. Reset knowledge summary tasks
	resultSummary := stuckKnowledgeSummaryQuery(db, staleCutoff).Updates(map[string]interface{}{
		"summary_status": types.SummaryStatusFailed,
	})
	if resultSummary.Error != nil {
		logger.Warnf(context.Background(), "Failed to reset pending summary tasks: %v", resultSummary.Error)
	} else if resultSummary.RowsAffected > 0 {
		logger.Infof(context.Background(), "Reset %d stuck summary generation tasks to failed state", resultSummary.RowsAffected)
	}

}

func stuckKnowledgeParseQuery(db *gorm.DB, staleCutoff time.Time) *gorm.DB {
	return db.Model(&types.Knowledge{}).
		Where("parse_status IN ?", []string{
			types.ParseStatusPending,
			types.ParseStatusProcessing,
			types.ParseStatusFinalizing,
			types.ParseStatusDeleting,
		}).
		Where("updated_at < ?", staleCutoff)
}

func stuckKnowledgeSummaryQuery(db *gorm.DB, staleCutoff time.Time) *gorm.DB {
	return db.Model(&types.Knowledge{}).
		Where("summary_status IN ?", []string{types.SummaryStatusPending, types.SummaryStatusProcessing}).
		Where("updated_at < ?", staleCutoff)
}
