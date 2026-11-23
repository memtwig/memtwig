package proposal_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/proposal"
)

func TestFormatDiffAppendAndNewFile(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "projects"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "projects", "api.md"), []byte("# API\nLine 1\nLine 2\n"), 0o644)

	diffAppend := proposal.FormatDiff(tmp, proposal.Proposal{
		Kind:    "append",
		Path:    "projects/api.md",
		Content: "- [2026-11-23 · agent] Added cache",
	})
	if !strings.Contains(diffAppend, "+ - [2026-11-23 · agent] Added cache") || !strings.Contains(diffAppend, "Line 2") {
		t.Fatalf("unexpected append diff:\n%s", diffAppend)
	}

	diffNew := proposal.FormatDiff(tmp, proposal.Proposal{
		Kind:    "create",
		Path:    "projects/new.md",
		Content: "# New\nBody",
	})
	if !strings.Contains(diffNew, "@@ new file @@") || !strings.Contains(diffNew, "+ # New") {
		t.Fatalf("unexpected new file diff:\n%s", diffNew)
	}
}
