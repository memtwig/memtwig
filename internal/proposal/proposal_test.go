package proposal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDeterministicID(t *testing.T) {
	id1 := DeterministicID("create", "projects/foo.md", "Hello   World\nLine 2")
	id2 := DeterministicID("create", "projects/foo.md", "hello world line 2")
	if id1 != id2 {
		t.Fatalf("expected deterministic ID to normalize case/whitespace: %q vs %q", id1, id2)
	}
	if len(id1) != 12 {
		t.Fatalf("expected 12-char hex ID, got %q (len=%d)", id1, len(id1))
	}
	id3 := DeterministicID("update", "projects/foo.md", "hello world line 2")
	if id3 == id1 {
		t.Fatalf("expected different kind to produce different ID")
	}
}

func TestAddListApproveCreateUpdateAppend(t *testing.T) {
	root := t.TempDir()

	// 1. Create proposal
	pCreate, added, err := Add(root, Proposal{
		Kind:    "create",
		Path:    "/projects/new.md",
		Content: "# New\n\nInitial text.\n",
		Reason:  "new project",
		Author:  "agent",
	})
	if err != nil || !added {
		t.Fatalf("Add create failed: added=%v err=%v", added, err)
	}
	if pCreate.Path != "projects/new.md" || pCreate.Status != "open" {
		t.Fatalf("unexpected proposal: %+v", pCreate)
	}

	// Duplicate Add returns existing without adding
	dup, added2, err := Add(root, Proposal{
		Kind:    "create",
		Path:    "projects/new.md",
		Content: "# New\n\nInitial text.\n",
	})
	if err != nil || added2 || dup.ID != pCreate.ID {
		t.Fatalf("expected duplicate Add to return existing: dup=%+v added=%v err=%v", dup, added2, err)
	}

	openList, err := List(root, false)
	if err != nil || len(openList) != 1 {
		t.Fatalf("List(false) = %+v, err=%v", openList, err)
	}

	// Approve create
	approvedCreate, err := Approve(root, pCreate.ID)
	if err != nil || approvedCreate.Status != "approved" {
		t.Fatalf("Approve create failed: %+v, err=%v", approvedCreate, err)
	}
	got, err := os.ReadFile(filepath.Join(root, "projects", "new.md"))
	if err != nil || string(got) != "# New\n\nInitial text.\n" {
		t.Fatalf("created file mismatch: %q, err=%v", string(got), err)
	}

	// 2. Append proposal
	pAppend, added, err := Add(root, Proposal{
		Kind:    "append",
		Path:    "projects/new.md",
		Content: "- Appended bullet",
	})
	if err != nil || !added {
		t.Fatalf("Add append failed: %v", err)
	}
	if _, err := Approve(root, pAppend.ID); err != nil {
		t.Fatalf("Approve append failed: %v", err)
	}
	gotAfterAppend, _ := os.ReadFile(filepath.Join(root, "projects", "new.md"))
	if !strings.Contains(string(gotAfterAppend), "Initial text.") || !strings.Contains(string(gotAfterAppend), "- Appended bullet") {
		t.Fatalf("append did not preserve + append content: %q", string(gotAfterAppend))
	}

	// 3. Update proposal
	pUpdate, added, err := Add(root, Proposal{
		Kind:    "update",
		Path:    "projects/new.md",
		Content: "# Replaced\n",
	})
	if err != nil || !added {
		t.Fatalf("Add update failed: %v", err)
	}
	if _, err := Approve(root, pUpdate.ID); err != nil {
		t.Fatalf("Approve update failed: %v", err)
	}
	gotAfterUpdate, _ := os.ReadFile(filepath.Join(root, "projects", "new.md"))
	if string(gotAfterUpdate) != "# Replaced\n" {
		t.Fatalf("update did not overwrite content: %q", string(gotAfterUpdate))
	}

	// Open list is empty, closed list has 3
	openList, _ = List(root, false)
	allList, _ := List(root, true)
	if len(openList) != 0 || len(allList) != 3 {
		t.Fatalf("expected 0 open and 3 total, got open=%d total=%d", len(openList), len(allList))
	}
}

func TestRejectAndHistoryIdempotency(t *testing.T) {
	root := t.TempDir()

	p, added, err := Add(root, Proposal{
		Kind:    "append",
		Path:    "persona/style.md",
		Content: "Always use comic sans.",
		Reason:  "bad idea",
	})
	if err != nil || !added {
		t.Fatalf("Add failed: %v", err)
	}

	rej, err := Reject(root, p.ID, "no thanks")
	if err != nil {
		t.Fatalf("Reject failed: %v", err)
	}
	if rej.Status != "rejected" || rej.Outcome != "no thanks" {
		t.Fatalf("unexpected rejected proposal: %+v", rej)
	}

	// Even if proposals.json is removed, history.jsonl prevents reopening the same proposal
	if err := os.Remove(filepath.Join(root, ".curator", "proposals.json")); err != nil {
		t.Fatal(err)
	}
	_, reopened, err := Add(root, Proposal{
		Kind:    "append",
		Path:    "persona/style.md",
		Content: "always use   comic sans.",
	})
	if err != nil {
		t.Fatalf("Add after Reject failed: %v", err)
	}
	if reopened {
		t.Fatalf("rejected proposal should never reopen")
	}
}

func TestDeleteProposalApproveAndDiff(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "projects", "obsolete.md")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("# Obsolete\nLine two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	p, added, err := Add(root, Proposal{
		Kind:   "delete",
		Path:   "projects/obsolete.md",
		Reason: "temporary note no longer needed",
		Author: "agent",
	})
	if err != nil || !added {
		t.Fatalf("Add delete proposal failed: added=%v err=%v", added, err)
	}

	diff := FormatDiff(root, p)
	if !strings.Contains(diff, "+++ /dev/null") || !strings.Contains(diff, "- # Obsolete") {
		t.Fatalf("unexpected delete diff:\n%s", diff)
	}

	approved, err := Approve(root, p.ID)
	if err != nil || approved.Status != "approved" {
		t.Fatalf("Approve delete failed: %+v err=%v", approved, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted after Approve, stat err=%v", err)
	}
}
