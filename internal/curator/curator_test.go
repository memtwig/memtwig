package curator_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/curator"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

func TestDecideInvalidateAndCurate(t *testing.T) {
	tmp := t.TempDir()
	if err := vault.Init(tmp); err != nil {
		t.Fatal(err)
	}

	// Create a stale oversized note
	bigBody := strings.Repeat("old fact line\n", 700)
	staleNote := "---\nstatus: active\nupdated: 2024-01-01\n---\n# Legacy\nold endpoint http://old.example\n" + bigBody
	if err := os.WriteFile(filepath.Join(tmp, "projects", "legacy.md"), []byte(staleNote), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Decide (direct write)
	msg, err := curator.Decide(tmp, "Legacy", "Moved to new API v2", "projects/legacy.md", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "appended to projects/legacy.md") {
		t.Fatalf("unexpected decide msg: %s", msg)
	}

	// 2. Invalidate old endpoint line -> must move to projects/archive/legacy.md
	moved, arch, err := curator.Invalidate(tmp, "projects/legacy.md", "http://old.example", "migrated to v2")
	if err != nil {
		t.Fatal(err)
	}
	if moved != 1 || arch != "projects/archive/legacy.md" {
		t.Fatalf("unexpected invalidate result: moved=%d arch=%s", moved, arch)
	}
	archRaw, err := os.ReadFile(filepath.Join(tmp, filepath.FromSlash(arch)))
	if err != nil || !strings.Contains(string(archRaw), "http://old.example") {
		t.Fatalf("expected archived line in %s", arch)
	}

	// 3. Curate must flag oversize + stale and generate proposals
	rep, err := curator.Curate("test", tmp, 8192, 90)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OversizeNotes < 1 || rep.StaleNotes < 1 || rep.ProposalsCreated < 2 {
		t.Fatalf("unexpected curate report: %+v", rep)
	}
	props, err := proposal.List(tmp, false)
	if err != nil || len(props) < 2 {
		t.Fatalf("expected >=2 proposals, got %d", len(props))
	}
}
