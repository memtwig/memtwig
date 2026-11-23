package migrate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/migrate"
)

func TestMigrateExistingDirectory(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "obsidian")
	_ = os.MkdirAll(filepath.Join(src, "projects"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "projects", "note.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Note\nHello.\n"), 0o644)

	cfgPath := filepath.Join(tmp, "config.toml")
	rep, err := migrate.ImportExisting(cfgPath, "obsidian", src, "personal", "direct", []string{"persona/**"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Notes != 1 || rep.Errors != 0 {
		t.Fatalf("unexpected migrate report: %+v", rep)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg.Vaults["obsidian"]; !ok {
		t.Fatal("expected obsidian vault in saved config")
	}
}
