package tui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vittoriocompagno/memtwig/internal/audit"
	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/index"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/tui"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

func TestTUISearchAndProposalApprove(t *testing.T) {
	tmp := t.TempDir()
	vDir := filepath.Join(tmp, "vault")
	if err := vault.Init(vDir); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(vDir, "projects", "api.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# API\nRuns on Fly.io.\n"), 0o644)

	p, _, err := proposal.Add(vDir, proposal.Proposal{
		Kind:    "append",
		Path:    "projects/api.md",
		Content: "- [2026-11-23 · agent] Added Redis",
		Reason:  "detected in compose",
		Author:  "claude",
	})
	if err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vDir, Kind: "personal", DefaultWrite: "direct"}
	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	m := tui.New(cfg, st, audit.New(filepath.Join(tmp, "audit.jsonl")), tmp, "", "1.2.0-test")

	// Render initial search view
	v1 := m.View()
	if !strings.Contains(v1, "memtwig 1.2.0-test") || !strings.Contains(v1, "2 Proposals (1)") {
		t.Fatalf("unexpected initial TUI view:\n%s", v1)
	}

	// Switch to Proposals tab ('2') and verify diff preview
	m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	v2 := m2.View()
	if !strings.Contains(v2, p.ID) || !strings.Contains(v2, "Added Redis") {
		t.Fatalf("expected proposal diff in tab 2:\n%s", v2)
	}

	// Approve proposal ('a') -> file updated and proposal queue emptied
	m3, _ := m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	v3 := m3.View()
	if !strings.Contains(v3, "approved "+p.ID) {
		t.Fatalf("expected approval status message, got:\n%s", v3)
	}
	raw, _ := os.ReadFile(filepath.Join(vDir, "projects", "api.md"))
	if !strings.Contains(string(raw), "Added Redis") {
		t.Fatalf("expected approved content written to projects/api.md")
	}

	// Switch to Vaults tab ('3')
	m4, _ := m3.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if !strings.Contains(m4.View(), "Active Vaults in Current Scope") {
		t.Fatalf("expected vaults tab view")
	}
}
