package lint_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/lint"
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

func TestLintExemptsDecisionLogsFromOversizeWarning(t *testing.T) {
	tmp := t.TempDir()
	for _, dir := range []string{"decisions", "decisioni", "projects"} {
		if err := os.MkdirAll(filepath.Join(tmp, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	bigDecision := "# Decisions 2026-11\n\n" + strings.Repeat("- 2026-11-23 proj: entry\n", 500)
	if err := os.WriteFile(filepath.Join(tmp, "decisions", "2026-11.md"), []byte(bigDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "decisioni", "2026-11.md"), []byte(bigDecision), 0o644); err != nil {
		t.Fatal(err)
	}
	bigProject := "---\nstatus: active\nupdated: 2026-11-23\n---\n# Big\n" + strings.Repeat("fact line\n", 1000)
	if err := os.WriteFile(filepath.Join(tmp, "projects", "big.md"), []byte(bigProject), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := lint.CheckVault("test", tmp, 8192)
	if err != nil {
		t.Fatal(err)
	}
	var oversize []string
	for _, iss := range rep.Issues {
		if iss.Code == "oversize_note" {
			oversize = append(oversize, iss.Path)
		}
	}
	if len(oversize) != 1 || oversize[0] != "projects/big.md" {
		t.Fatalf("expected only projects/big.md oversize_note, got %v (all issues: %+v)", oversize, rep.Issues)
	}
}
