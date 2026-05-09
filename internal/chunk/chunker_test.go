package chunk

import (
	"strings"
	"testing"
)

func TestChunker_Overlap_manyChunks(t *testing.T) {
	s := strings.Repeat("あ", 2000)
	parts := Chunk(s)
	if len(parts) < 2 {
		t.Fatalf("len=%d", len(parts))
	}
}

func TestChunker_Overlap_sharedRunes(t *testing.T) {
	s := strings.Repeat("a", ChunkSizeRunes) + strings.Repeat("b", ChunkOverlapRunes+10)
	parts := Chunk(s)
	if len(parts) < 2 {
		t.Fatalf("want >=2 chunks, got %d", len(parts))
	}
	// Tail of first chunk (overlap region) must match start of second for overlap runes.
	r0 := []rune(parts[0])
	r1 := []rune(parts[1])
	tail := string(r0[len(r0)-ChunkOverlapRunes:])
	head := string(r1[:ChunkOverlapRunes])
	if tail != head {
		t.Fatalf("overlap mismatch tail=%q head=%q", tail, head)
	}
}

func TestChunker_shortSingleChunk(t *testing.T) {
	s := strings.Repeat("z", 100)
	parts := Chunk(s)
	if len(parts) != 1 || parts[0] != s {
		t.Fatalf("got %#v", parts)
	}
}

func TestChunker_empty(t *testing.T) {
	if Chunk("") != nil {
		t.Fatalf("expected nil slice for empty input")
	}
}

func TestChunker_exactlyOneWindow(t *testing.T) {
	s := strings.Repeat("c", ChunkSizeRunes)
	parts := Chunk(s)
	if len(parts) != 1 || parts[0] != s {
		t.Fatalf("got %#v", parts)
	}
}
