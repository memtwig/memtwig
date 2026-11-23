package lint_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/lint"
)

func TestLintDetectsSecretsAndFutureDates(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	bad := "---\nstatus: active\nupdated: 2099-01-01\n---\n# Bad\nLeaked token ghp_123456789012345678901234567890123456\n"
	if err := os.WriteFile(filepath.Join(tmp, "projects", "bad.md"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := lint.CheckVault("test", tmp, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Errors < 2 {
		t.Fatalf("expected at least 2 errors (secret + future_date), got %+v", rep)
	}
}
