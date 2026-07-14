package service

import (
	"context"
	"testing"

	"github.com/xyz2781790037/ZealRAG/internal/types"
)

func TestShouldReuseDocumentChunksWithParentChild(t *testing.T) {
	options := ProcessChunksOptions{
		ReuseUnchangedChunks: true,
		ParentChunks: []types.ParsedParentChunk{
			{Content: "parent", Seq: 0},
		},
	}
	if !shouldReuseDocumentChunks(options) {
		t.Fatal("parent-child input must not disable unchanged text reuse")
	}
}

func TestChunkReuseKeepsTextAndLeavesParentChunksStale(t *testing.T) {
	knowledge := &types.Knowledge{
		ID:               "knowledge-1",
		Title:            "Document",
		EmbeddingModelID: "embedding-1",
	}
	fingerprint := documentChunkFingerprintForKnowledge(knowledge, "stable child", "section")
	oldText := &types.Chunk{
		ID:          "text-child",
		KnowledgeID: knowledge.ID,
		Content:     "stable child",
		ContentHash: fingerprint,
		ChunkType:   types.ChunkTypeText,
	}
	oldParent := &types.Chunk{
		ID:          "parent-old",
		KnowledgeID: knowledge.ID,
		Content:     "parent",
		ChunkType:   types.ChunkTypeParentText,
	}

	state := newChunkReuseState(context.Background(), knowledge, []*types.Chunk{oldText, oldParent}, false)
	reused, ok := state.takeTextChunk(fingerprint)
	if !ok || reused.ID != oldText.ID {
		t.Fatalf("reused=%v ok=%v, want text-child", reused, ok)
	}

	stale := state.staleChunkIDs()
	if len(stale) != 1 || stale[0] != oldParent.ID {
		t.Fatalf("stale=%v, want old parent chunk", stale)
	}
}
