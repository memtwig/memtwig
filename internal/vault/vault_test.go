package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	root := t.TempDir()
	if err := Init(root); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	expectedPaths := []string{
		"projects",
		"decisions",
		"persona",
		"infra",
		".curator",
		"memtwig.toml",
		".gitignore",
		"projects/getting-started.md",
		"index.md",
	}
	for _, rel := range expectedPaths {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("expected %s to exist after Init: %v", rel, err)
		}
	}

	// Idempotent: second Init preserves existing memtwig.toml
	customToml := "version = 1\nmax_note_bytes = 4096\n"
	if err := os.WriteFile(filepath.Join(root, "memtwig.toml"), []byte(customToml), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Init(root); err != nil {
		t.Fatalf("second Init failed: %v", err)
	}
	gotToml, err := os.ReadFile(filepath.Join(root, "memtwig.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gotToml) != customToml {
		t.Errorf("Init overwrote existing memtwig.toml: got %q", string(gotToml))
	}
}

func TestParseNoteYAMLFrontmatter(t *testing.T) {
	root := t.TempDir()
	rel := "projects/api.md"
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `---
status: active
valid_from: 2026-01-15
updated: 2026-02-20
owner: alice
tags:
  - go
  - api
---

# API Service

Service details here.
`
	if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ParseNote("acme", root, rel)
	if err != nil {
		t.Fatalf("ParseNote failed: %v", err)
	}
	if n.VaultName != "acme" || n.RelPath != rel {
		t.Errorf("unexpected vault/path: %+v", n)
	}
	if n.Title != "API Service" {
		t.Errorf("Title = %q, want %q", n.Title, "API Service")
	}
	if n.Status != "active" || n.ValidFrom != "2026-01-15" || n.Updated != "2026-02-20" || n.Owner != "alice" {
		t.Errorf("metadata mismatch: %+v", n)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "go" || n.Tags[1] != "api" {
		t.Errorf("Tags = %v, want [go api]", n.Tags)
	}
	if n.SHA256 == "" || n.SizeBytes != int64(len(content)) {
		t.Errorf("SHA256/SizeBytes mismatch: sha=%q size=%d", n.SHA256, n.SizeBytes)
	}
	if strings.HasPrefix(n.Body, "---") {
		t.Errorf("Body should have frontmatter stripped, got %q", n.Body)
	}
}

func TestParseNoteItalianLightweightHeaders(t *testing.T) {
	root := t.TempDir()
	rel := "progetti/memtwig.md"
	if err := os.MkdirAll(filepath.Join(root, "progetti"), 0o755); err != nil {
		t.Fatal(err)
	}
	content := `# Memtwig Italia
stato: attivo
valido dal: 2026-03-01
Aggiornato: 2026-04-10
proprietario: vittorio

- [2026-04-10 · vittorio] Prima nota leggera.
`
	if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := ParseNote("personal", root, rel)
	if err != nil {
		t.Fatalf("ParseNote failed: %v", err)
	}
	if n.Title != "Memtwig Italia" {
		t.Errorf("Title = %q, want %q", n.Title, "Memtwig Italia")
	}
	if n.Status != "active" {
		t.Errorf("Status = %q, want %q (normalized from attivo)", n.Status, "active")
	}
	if n.ValidFrom != "2026-03-01" {
		t.Errorf("ValidFrom = %q, want %q", n.ValidFrom, "2026-03-01")
	}
	if n.Updated != "2026-04-10" {
		t.Errorf("Updated = %q, want %q", n.Updated, "2026-04-10")
	}
	if n.Owner != "vittorio" {
		t.Errorf("Owner = %q, want %q", n.Owner, "vittorio")
	}
}

func TestWalkNotesSkipsSpecialDirsAndFiles(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"projects/a.md":       "# A\n",
		"infra/b.md":          "# B\n",
		"projects/ignore.txt": "not markdown",
		"index.md":            "# Index\n",
		"README.md":           "# Readme\n",
		".git/config.md":      "# Git\n",
		".curator/log.md":     "# Curator\n",
		"private/secret.md":   "# Secret\n",
		"archive/old.md":      "# Old\n",
		"archivio/vecchio.md": "# Vecchio\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	notes, err := WalkNotes("test", root)
	if err != nil {
		t.Fatalf("WalkNotes failed: %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("expected 2 notes, got %d: %+v", len(notes), notes)
	}
	if notes[0].RelPath != "infra/b.md" || notes[1].RelPath != "projects/a.md" {
		t.Fatalf("unexpected sorted notes: %+v", notes)
	}
}

func TestGenerateIndex(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "projects", "one.md"), []byte("stato: parcheggiato\n# Project One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "top.md"), []byte("# Top Note\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	count, err := GenerateIndex(root)
	if err != nil {
		t.Fatalf("GenerateIndex failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count=2, got %d", count)
	}
	idxBytes, err := os.ReadFile(filepath.Join(root, "index.md"))
	if err != nil {
		t.Fatalf("read index.md failed: %v", err)
	}
	idx := string(idxBytes)
	if !strings.Contains(idx, "## projects") || !strings.Contains(idx, "- [Project One](projects/one.md) (parked)") {
		t.Errorf("index.md missing expected project entry:\n%s", idx)
	}
	if !strings.Contains(idx, "## root") || !strings.Contains(idx, "- [Top Note](top.md)") {
		t.Errorf("index.md missing expected root entry:\n%s", idx)
	}
}

func TestValidateDeletableNotePath(t *testing.T) {
	allowed := []string{
		"test.md",
		"projects/tmp.md",
		"persona/draft.md",
		"/progetti/bozza.md",
	}
	for _, p := range allowed {
		if err := ValidateDeletableNotePath(p); err != nil {
			t.Errorf("expected %q to be deletable, got error: %v", p, err)
		}
	}

	rejected := []string{
		"",
		".",
		"../outside.md",
		"memtwig.toml",
		"index.md",
		"README.md",
		".git/HEAD.md",
		".curator/proposals.md",
		"private/secret.md",
		"projects/archive/old.md",
		"progetti/archivio/old.md",
		"decisions/2026-11.md",
		"decisioni/2026-11.md",
	}
	for _, p := range rejected {
		if err := ValidateDeletableNotePath(p); err == nil {
			t.Errorf("expected %q to be rejected for deletion", p)
		}
	}
}
