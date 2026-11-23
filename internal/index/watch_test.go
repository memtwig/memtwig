package index_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/index"
)

func TestWatchAndEnsureFresh(t *testing.T) {
	tmp := t.TempDir()
	vDir := filepath.Join(tmp, "vault")
	_ = os.MkdirAll(filepath.Join(vDir, "projects"), 0o755)
	_ = os.WriteFile(filepath.Join(vDir, "projects", "a.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Alpha\nFirst version.\n"), 0o644)

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vDir, Kind: "personal", DefaultWrite: "direct"}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// EnsureFresh on empty DB indexes immediately
	s1, err := st.EnsureFresh(context.Background(), cfg)
	if err != nil || s1.Indexed != 1 {
		t.Fatalf("expected 1 indexed on first EnsureFresh, got %+v err=%v", s1, err)
	}

	// Watch detects live edits
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	updates := 0
	go func() {
		time.Sleep(80 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(vDir, "projects", "a.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Alpha\nUpdated content.\n"), 0o644)
	}()
	if err := st.Watch(ctx, cfg, 50*time.Millisecond, func(s index.Stats) {
		updates += s.Indexed
	}); err != nil {
		t.Fatal(err)
	}
	if updates < 1 {
		t.Fatalf("expected watch to detect modified note, updates=%d", updates)
	}
}
