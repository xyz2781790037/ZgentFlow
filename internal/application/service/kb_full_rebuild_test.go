package service

import (
	"context"
	"testing"
	"time"

	"github.com/xyz2781790037/ZealRAG/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRebuildPublishTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`CREATE TABLE knowledge_bases (
			id TEXT PRIMARY KEY, tenant_id INTEGER NOT NULL,
			embedding_model_id TEXT, summary_model_id TEXT,
			active_generation INTEGER NOT NULL, building_generation INTEGER,
			rebuild_stage_id TEXT, rebuild_status TEXT, rebuild_error TEXT,
			rebuild_completed_at DATETIME, deleted_at DATETIME
		)`,
		`CREATE TABLE embeddings (id INTEGER PRIMARY KEY, knowledge_base_id TEXT, source_id TEXT)`,
		`CREATE TABLE chunks (
			id TEXT PRIMARY KEY, tenant_id INTEGER, knowledge_base_id TEXT,
			knowledge_id TEXT, content TEXT, status INTEGER, is_enabled BOOLEAN,
			chunk_type TEXT, chunk_index INTEGER DEFAULT 0, updated_at DATETIME, deleted_at DATETIME
		)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create test schema: %v", err)
		}
	}
	return db
}

func seedRebuildPublishTest(t *testing.T, db *gorm.DB) chunkSnapshot {
	t.Helper()
	now := time.Unix(1_700_000_000, 0).UTC()
	statements := []struct {
		sql  string
		args []interface{}
	}{
		{`INSERT INTO knowledge_bases
			(id, tenant_id, embedding_model_id, summary_model_id, active_generation, building_generation, rebuild_stage_id, rebuild_status, rebuild_error)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, '')`, []interface{}{"kb-live", 1, "embed-old", "chat-old", 1, 2, "kb-stage", types.KBRebuildStatusRunning}},
		{`INSERT INTO knowledge_bases
			(id, tenant_id, active_generation, rebuild_status, rebuild_error)
			VALUES (?, ?, ?, ?, '')`, []interface{}{"kb-stage", 1, 1, types.KBRebuildStatusIdle}},
		{`INSERT INTO chunks
			(id, tenant_id, knowledge_base_id, knowledge_id, content, status, is_enabled, chunk_type, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, []interface{}{"chunk-source", 1, "kb-live", "knowledge-1", "source", 0, true, types.ChunkTypeText, now}},
		{`INSERT INTO embeddings (id, knowledge_base_id, source_id) VALUES (1, 'kb-live', 'old-vector')`, nil},
		{`INSERT INTO embeddings (id, knowledge_base_id, source_id) VALUES (2, 'kb-stage', 'new-vector')`, nil},
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("seed test data: %v", err)
		}
	}

	svc := &kbFullRebuildService{db: db}
	chunks, err := svc.listSourceChunks(context.Background(), 1, "kb-live")
	if err != nil {
		t.Fatal(err)
	}
	return fingerprintSourceChunks(chunks)
}

func TestKBFullRebuildPublishSwitchesAllStagedData(t *testing.T) {
	db := newRebuildPublishTestDB(t)
	snapshot := seedRebuildPublishTest(t, db)
	svc := &kbFullRebuildService{db: db}
	payload := types.KBFullRebuildPayload{
		TenantID: 1, KnowledgeBaseID: "kb-live", Generation: 2,
		EmbeddingModelID: "embed-new", KnowledgeQAModelID: "chat-new",
	}

	if err := svc.publish(context.Background(), payload, "kb-stage", snapshot); err != nil {
		t.Fatalf("publish: %v", err)
	}

	var generation int64
	var status string
	var building *int64
	if err := db.Raw(`SELECT active_generation, building_generation, rebuild_status
		FROM knowledge_bases WHERE id = 'kb-live'`).Row().Scan(&generation, &building, &status); err != nil {
		t.Fatal(err)
	}
	if generation != 2 || building != nil || status != types.KBRebuildStatusSucceeded {
		t.Fatalf("unexpected published state: generation=%d building=%v status=%s", generation, building, status)
	}
	var embeddingModelID, chatModelID string
	if err := db.Raw(`SELECT embedding_model_id, summary_model_id
		FROM knowledge_bases WHERE id = 'kb-live'`).Row().Scan(&embeddingModelID, &chatModelID); err != nil {
		t.Fatal(err)
	}
	if embeddingModelID != "embed-new" || chatModelID != "chat-new" {
		t.Fatalf("unexpected published models: embedding=%s chat=%s", embeddingModelID, chatModelID)
	}

	assertCount := func(query string, want int64) {
		t.Helper()
		var got int64
		if err := db.Raw(query).Scan(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("query %q: got %d want %d", query, got, want)
		}
	}
	assertCount(`SELECT count(*) FROM embeddings WHERE knowledge_base_id = 'kb-live' AND source_id = 'new-vector'`, 1)
	assertCount(`SELECT count(*) FROM embeddings WHERE source_id = 'old-vector'`, 0)
	assertCount(`SELECT count(*) FROM knowledge_bases WHERE id = 'kb-stage'`, 0)
}

func TestKBFullRebuildPublishRejectsChangedSources(t *testing.T) {
	db := newRebuildPublishTestDB(t)
	snapshot := seedRebuildPublishTest(t, db)
	svc := &kbFullRebuildService{db: db}
	payload := types.KBFullRebuildPayload{TenantID: 1, KnowledgeBaseID: "kb-live", Generation: 2}
	if err := db.Exec(`UPDATE chunks SET content = 'changed' WHERE id = 'chunk-source'`).Error; err != nil {
		t.Fatal(err)
	}

	if err := svc.publish(context.Background(), payload, "kb-stage", snapshot); err == nil {
		t.Fatal("expected publish to fail when source chunks changed")
	}

	var generation int64
	var status string
	if err := db.Raw(`SELECT active_generation, rebuild_status FROM knowledge_bases WHERE id = 'kb-live'`).Row().Scan(&generation, &status); err != nil {
		t.Fatal(err)
	}
	if generation != 1 || status != types.KBRebuildStatusRunning {
		t.Fatalf("old state was modified: generation=%d status=%s", generation, status)
	}

	var oldVectorCount, stageVectorCount int64
	db.Raw(`SELECT count(*) FROM embeddings WHERE knowledge_base_id = 'kb-live' AND source_id = 'old-vector'`).Scan(&oldVectorCount)
	db.Raw(`SELECT count(*) FROM embeddings WHERE knowledge_base_id = 'kb-stage' AND source_id = 'new-vector'`).Scan(&stageVectorCount)
	if oldVectorCount != 1 || stageVectorCount != 1 {
		t.Fatalf("publish rollback was incomplete: old_vector=%d stage_vector=%d",
			oldVectorCount, stageVectorCount)
	}
}

func TestCopyReusableEmbeddingsCopiesCompatibleRowsIdempotently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE embeddings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at DATETIME, updated_at DATETIME,
		source_id TEXT NOT NULL, source_type INTEGER NOT NULL,
		chunk_id TEXT, knowledge_id TEXT, knowledge_base_id TEXT,
		content TEXT, dimension INTEGER NOT NULL, embedding BLOB,
		is_enabled BOOLEAN, tag_id TEXT,
		UNIQUE (knowledge_base_id, source_id, source_type)
	)`).Error; err != nil {
		t.Fatal(err)
	}
	statements := []string{
		`INSERT INTO embeddings
			(source_id, source_type, chunk_id, knowledge_id, knowledge_base_id, content, dimension, embedding, is_enabled)
		 VALUES ('chunk-reuse', 0, 'chunk-reuse', 'knowledge-1', 'kb-live', 'reusable', 3, X'010203', true)`,
		`INSERT INTO embeddings
			(source_id, source_type, chunk_id, knowledge_id, knowledge_base_id, content, dimension, embedding, is_enabled)
		 VALUES ('chunk-wrong-dim', 0, 'chunk-wrong-dim', 'knowledge-1', 'kb-live', 'wrong dim', 4, X'01020304', true)`,
		`INSERT INTO embeddings
			(source_id, source_type, chunk_id, knowledge_id, knowledge_base_id, content, dimension, embedding, is_enabled)
		 VALUES ('chunk-empty-vector', 0, 'chunk-empty-vector', 'knowledge-1', 'kb-live', 'empty', 3, NULL, true)`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}

	svc := &kbFullRebuildService{db: db}
	chunkIDs := []string{"chunk-reuse", "chunk-wrong-dim", "chunk-empty-vector", "chunk-missing"}
	for attempt := 0; attempt < 2; attempt++ {
		reused, err := svc.copyReusableEmbeddings(context.Background(), "kb-live", "kb-stage", chunkIDs, 3)
		if err != nil {
			t.Fatalf("copy reusable embeddings attempt %d: %v", attempt+1, err)
		}
		if len(reused) != 1 {
			t.Fatalf("attempt %d reused=%v, want only chunk-reuse", attempt+1, reused)
		}
		if _, ok := reused["chunk-reuse"]; !ok {
			t.Fatalf("attempt %d did not reuse chunk-reuse", attempt+1)
		}
	}

	var count int64
	if err := db.Table("embeddings").Where("knowledge_base_id = ?", "kb-stage").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("stage embedding rows=%d, want 1", count)
	}
}

func TestValidateStageRequiresExpectedVectors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`CREATE TABLE embeddings (id INTEGER PRIMARY KEY, knowledge_base_id TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO embeddings (id, knowledge_base_id) VALUES (1, 'kb-stage')`).Error; err != nil {
		t.Fatal(err)
	}

	svc := &kbFullRebuildService{db: db}
	if err := svc.validateStage(context.Background(), "kb-stage", 1); err != nil {
		t.Fatalf("validate vectors: %v", err)
	}
	if err := svc.validateStage(context.Background(), "kb-stage", 2); err == nil {
		t.Fatal("expected validation to reject incomplete vectors")
	}
}
