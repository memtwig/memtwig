package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/vault"
)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v (%s)", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func setupGitVault(t *testing.T, dir string) {
	t.Helper()
	if err := vault.Init(dir); err != nil {
		t.Fatalf("vault.Init failed: %v", err)
	}
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
}

func TestSyncVaultAbortsOnSecret(t *testing.T) {
	root := t.TempDir()
	setupGitVault(t, root)

	// Add a note containing a fake OpenAI key pattern (sk- + 20+ chars)
	badNote := filepath.Join(root, "projects", "leak.md")
	content := "---\nstatus: active\nupdated: 2026-10-01\n---\n\n# Leak\n\nkey = sk-abcdefghijklmnopqrstuvwxyz1234\n"
	if err := os.WriteFile(badNote, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := SyncVault("personal", root, 8192, Options{Message: "should abort"})
	if err == nil || !strings.Contains(err.Error(), "sync aborted") {
		t.Fatalf("expected sync aborted error, got %v", err)
	}
}

func TestSyncVaultCommitsDirtyFiles(t *testing.T) {
	root := t.TempDir()
	setupGitVault(t, root)

	res, err := SyncVault("personal", root, 8192, Options{Message: "initial vault commit"})
	if err != nil {
		t.Fatalf("SyncVault failed: %v", err)
	}
	if !res.Committed || res.Pushed || res.Summary != "committed locally" {
		t.Fatalf("unexpected result: %+v", res)
	}

	status := runGit(t, root, "status", "--porcelain")
	if status != "" {
		t.Errorf("expected clean working tree after sync, got %q", status)
	}
	lastMsg := runGit(t, root, "log", "-1", "--pretty=%s")
	if lastMsg != "initial vault commit" {
		t.Errorf("last commit message = %q, want %q", lastMsg, "initial vault commit")
	}

	// Second sync with no changes should be clean
	res2, err := SyncVault("personal", root, 8192, Options{})
	if err != nil {
		t.Fatalf("second SyncVault failed: %v", err)
	}
	if res2.Committed || res2.Pushed || res2.Summary != "clean" {
		t.Fatalf("expected clean result on second sync, got %+v", res2)
	}
}

func TestSyncVaultPushesToBareRemote(t *testing.T) {
	tmp := t.TempDir()
	bare := filepath.Join(tmp, "remote.git")
	if err := os.MkdirAll(bare, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, bare, "init", "--bare")

	local := filepath.Join(tmp, "local")
	if err := os.MkdirAll(local, 0o755); err != nil {
		t.Fatal(err)
	}
	setupGitVault(t, local)
	runGit(t, local, "remote", "add", "origin", bare)
	runGit(t, local, "add", "-A")
	runGit(t, local, "commit", "-m", "seed")
	branch := runGit(t, local, "rev-parse", "--abbrev-ref", "HEAD")
	runGit(t, local, "push", "-u", "origin", branch)

	// Create a new valid note and sync with Push=true
	note := "---\nstatus: active\nupdated: 2026-10-01\n---\n\n# Synced Note\n\nHello remote.\n"
	if err := os.WriteFile(filepath.Join(local, "projects", "synced.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := SyncVault("personal", local, 8192, Options{
		Push:    true,
		Message: "push synced note",
	})
	if err != nil {
		t.Fatalf("SyncVault push failed: %v", err)
	}
	if !res.Committed || !res.Pushed || res.Summary != "committed & pushed" {
		t.Fatalf("unexpected push result: %+v", res)
	}

	remoteMsg := runGit(t, bare, "log", "-1", "--pretty=%s", branch)
	if remoteMsg != "push synced note" {
		t.Errorf("bare remote head message = %q, want %q", remoteMsg, "push synced note")
	}
}
