package container

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const resetPendingKnowledgeDDL = `
CREATE TABLE IF NOT EXISTS knowledges (
    id              VARCHAR(64) PRIMARY KEY,
    parse_status    VARCHAR(32) NOT NULL DEFAULT 'pending',
    summary_status  VARCHAR(32) NOT NULL DEFAULT 'none',
    pending_subtasks_count INTEGER NOT NULL DEFAULT 0,
    error_message   TEXT,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at      DATETIME
);
`

func setupResetPendingDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec(resetPendingKnowledgeDDL).Error)
	return db
}

func TestResetPendingTasks_KnowledgeFindThenUpdate(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, parse_status, updated_at) VALUES (?, ?, ?)`,
		"k-stuck", types.ParseStatusProcessing, stale,
	).Error)

	resetPendingTasks(db)

	var status, errMsg string
	require.NoError(t, db.Raw(
		`SELECT parse_status, error_message FROM knowledges WHERE id = ?`, "k-stuck",
	).Row().Scan(&status, &errMsg))
	assert.Equal(t, types.ParseStatusFailed, status)
	assert.Contains(t, errMsg, "application restart")
}

func TestResetPendingTasks_KnowledgeFreshIsNotReset(t *testing.T) {
	db := setupResetPendingDB(t)
	fresh := time.Now().Add(-5 * time.Minute)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, parse_status, updated_at) VALUES (?, ?, ?)`,
		"k-fresh", types.ParseStatusProcessing, fresh,
	).Error)

	resetPendingTasks(db)

	var status string
	require.NoError(t, db.Raw(
		`SELECT parse_status FROM knowledges WHERE id = ?`, "k-fresh",
	).Row().Scan(&status))
	assert.Equal(t, types.ParseStatusProcessing, status)
}

func TestStuckKnowledgeParseQuery_ReuseAfterFindDoesNotBreakUpdate(t *testing.T) {
	db := setupResetPendingDB(t)
	stale := time.Now().Add(-2 * time.Hour)
	require.NoError(t, db.Exec(
		`INSERT INTO knowledges (id, parse_status, updated_at) VALUES (?, ?, ?)`,
		"k-reuse", types.ParseStatusProcessing, stale,
	).Error)

	staleCutoff := time.Now().Add(-resetPendingStaleWindow)

	var rows []types.Knowledge
	q := stuckKnowledgeParseQuery(db, staleCutoff)
	require.NoError(t, q.Select("id").Find(&rows).Error)
	require.Len(t, rows, 1)

	result := stuckKnowledgeParseQuery(db, staleCutoff).Updates(map[string]interface{}{
		"parse_status": types.ParseStatusFailed,
	})
	require.NoError(t, result.Error)
	assert.Equal(t, int64(1), result.RowsAffected)
}
