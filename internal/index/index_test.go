package index

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
)

func TestOpenAndClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sub", "index.db")
	st, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer st.Close()

	for _, tbl := range []string{"documents", "chunks", "chunks_fts"} {
		var name string
		err := st.DB.QueryRow(`SELECT name FROM sqlite_master WHERE name = ?`, tbl).Scan(&name)
		if err != nil {
			t.Errorf("expected table %s to exist: %v", tbl, err)
		}
	}
}

func TestRebuildOrUpdateIncrementalAndDeletion(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := filepath.Join(tmp, "vault")
	if err := os.MkdirAll(filepath.Join(vaultDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	noteA := filepath.Join(vaultDir, "projects", "a.md")
	noteB := filepath.Join(vaultDir, "projects", "b.md")
	if err := os.WriteFile(noteA, []byte("# Note A\n\nAlpha content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(noteB, []byte("# Note B\n\nBeta content.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vaultDir, Kind: "personal"}

	st, err := Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	ctx := context.Background()
	s1, err := st.RebuildOrUpdate(ctx, cfg, false)
	if err != nil {
		t.Fatalf("initial RebuildOrUpdate failed: %v", err)
	}
	if s1.Vaults != 1 || s1.Indexed != 2 || s1.Skipped != 0 || s1.Removed != 0 || s1.Chunks < 2 {
		t.Fatalf("unexpected initial stats: %+v", s1)
	}

	// Second run with no changes -> both skipped by SHA256
	s2, err := st.RebuildOrUpdate(ctx, cfg, false)
	if err != nil {
		t.Fatalf("incremental RebuildOrUpdate failed: %v", err)
	}
	if s2.Indexed != 0 || s2.Skipped != 2 || s2.Removed != 0 {
		t.Fatalf("expected 2 skipped, got %+v", s2)
	}

	// Modify noteA and delete noteB -> 1 indexed, 1 removed
	if err := os.WriteFile(noteA, []byte("# Note A\n\nUpdated alpha.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(noteB); err != nil {
		t.Fatal(err)
	}

	s3, err := st.RebuildOrUpdate(ctx, cfg, false)
	if err != nil {
		t.Fatalf("update+delete RebuildOrUpdate failed: %v", err)
	}
	if s3.Indexed != 1 || s3.Skipped != 0 || s3.Removed != 1 {
		t.Fatalf("expected Indexed=1 Skipped=0 Removed=1, got %+v", s3)
	}

	var docCount, chunkCount int
	if err := st.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docCount); err != nil {
		t.Fatal(err)
	}
	if err := st.DB.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunkCount); err != nil {
		t.Fatal(err)
	}
	if docCount != 1 || chunkCount != 1 {
		t.Errorf("expected 1 document and 1 chunk after deletion, got docs=%d chunks=%d", docCount, chunkCount)
	}
}

func TestChunkMarkdown(t *testing.T) {
	body := strings.Join([]string{
		"Intro paragraph.",
		"## Setup",
		"Step 1",
		"### Details",
		strings.Repeat("word ", 10) + "\n" + strings.Repeat("more ", 10),
	}, "\n")

	chunks := ChunkMarkdown("DocTitle", body, 40)
	if len(chunks) < 4 {
		t.Fatalf("expected at least 4 chunks (intro, Setup, and split Details), got %d: %+v", len(chunks), chunks)
	}
	if chunks[0].Heading != "DocTitle" || chunks[0].Content != "Intro paragraph." {
		t.Errorf("unexpected first chunk: %+v", chunks[0])
	}
	if chunks[1].Heading != "Setup" || chunks[1].Content != "Step 1" {
		t.Errorf("unexpected second chunk: %+v", chunks[1])
	}
	for i := 2; i < len(chunks); i++ {
		if chunks[i].Heading != "Details" {
			t.Errorf("chunk %d Heading = %q, want Details", i, chunks[i].Heading)
		}
	}
}

func TestEncodeDecodeFloat32s(t *testing.T) {
	if EncodeFloat32s(nil) != nil {
		t.Error("expected nil for empty EncodeFloat32s")
	}
	if DecodeFloat32s(nil) != nil {
		t.Error("expected nil for empty DecodeFloat32s")
	}
	if DecodeFloat32s([]byte{1, 2, 3}) != nil {
		t.Error("expected nil for non-multiple-of-4 bytes")
	}

	orig := []float32{0.0, -1.5, 3.14159, 42.25}
	blob := EncodeFloat32s(orig)
	got := DecodeFloat32s(blob)
	if len(got) != len(orig) {
		t.Fatalf("len mismatch: got %d, want %d", len(got), len(orig))
	}
	for i := range orig {
		if math.Abs(float64(got[i]-orig[i])) > 1e-6 {
			t.Errorf("index %d: got %v, want %v", i, got[i], orig[i])
		}
	}
}
