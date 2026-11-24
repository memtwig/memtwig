package tui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/memtwig/memtwig/internal/audit"
	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/proposal"
	"github.com/memtwig/memtwig/internal/tui"
	"github.com/memtwig/memtwig/internal/vault"
)

func key(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }

func send(t *testing.T, m tea.Model, msgs ...tea.Msg) tea.Model {
	t.Helper()
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

type fixture struct {
	tmp  string
	vDir string
	cfg  *config.Config
	st   *index.Store
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	tmp := t.TempDir()
	vDir := filepath.Join(tmp, "vault")
	if err := vault.Init(vDir); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vDir, Kind: "personal", DefaultWrite: "direct"}
	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return fixture{tmp: tmp, vDir: vDir, cfg: cfg, st: st}
}

func (f fixture) write(t *testing.T, rel, body string) {
	t.Helper()
	abs := filepath.Join(f.vDir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) model() tui.Model {
	return tui.New(f.cfg, f.st, audit.New(filepath.Join(f.tmp, "audit.jsonl")), f.tmp, "", "1.2.0-test")
}

func assertFits(t *testing.T, view string, w, h int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > h {
		t.Fatalf("view has %d lines, terminal is %d rows", len(lines), h)
	}
	for i, l := range lines {
		if lw := ansi.StringWidth(l); lw > w {
			t.Fatalf("line %d is %d cols wide (limit %d): %q", i, lw, w, ansi.Strip(l))
		}
	}
}

func TestTUISearchAndProposalApprove(t *testing.T) {
	f := newFixture(t)
	f.write(t, "projects/api.md", "---\nstatus: active\nupdated: 2026-11-23\n---\n# API\nRuns on Fly.io.\n")

	p, _, err := proposal.Add(f.vDir, proposal.Proposal{
		Kind:    "append",
		Path:    "projects/api.md",
		Content: "- [2026-11-23 · agent] Added Redis",
		Reason:  "detected in compose",
		Author:  "claude",
	})
	if err != nil {
		t.Fatal(err)
	}

	m := f.model()

	v1 := m.View()
	if !strings.Contains(v1, "memtwig 1.2.0-test") || !strings.Contains(v1, "2 Proposals (1)") {
		t.Fatalf("unexpected initial TUI view:\n%s", ansi.Strip(v1))
	}

	m2 := send(t, m, key('2'))
	v2 := m2.View()
	if !strings.Contains(v2, p.ID) || !strings.Contains(v2, "Added Redis") {
		t.Fatalf("expected proposal diff in tab 2:\n%s", ansi.Strip(v2))
	}

	// 'a' only arms the action; nothing is written until it is confirmed.
	m3 := send(t, m2, key('a'))
	if !strings.Contains(ansi.Strip(m3.View()), "y confirm") {
		t.Fatalf("expected confirmation prompt, got:\n%s", ansi.Strip(m3.View()))
	}
	raw, _ := os.ReadFile(filepath.Join(f.vDir, "projects", "api.md"))
	if strings.Contains(string(raw), "Added Redis") {
		t.Fatalf("proposal applied before confirmation")
	}
	m3 = send(t, m3, key('y'))
	if !strings.Contains(m3.View(), "approved "+p.ID) {
		t.Fatalf("expected approval status message, got:\n%s", ansi.Strip(m3.View()))
	}
	raw, _ = os.ReadFile(filepath.Join(f.vDir, "projects", "api.md"))
	if !strings.Contains(string(raw), "Added Redis") {
		t.Fatalf("expected approved content written to projects/api.md")
	}

	m4 := send(t, m3, key('3'))
	v4 := ansi.Strip(m4.View())
	if !strings.Contains(v4, "Vaults & Audit") || !strings.Contains(v4, "personal") || !strings.Contains(v4, "notes") {
		t.Fatalf("expected vaults tab view, got:\n%s", v4)
	}

	cfgPath := filepath.Join(f.tmp, "config.toml")
	m5 := m4.(tui.Model).WithConfigPath(cfgPath)
	m6 := send(t, m5, key('4'))
	v6 := ansi.Strip(m6.View())
	for _, want := range []string{"Providers", "Harnesses", "Claude Code", "Hermes", "agent-native"} {
		if !strings.Contains(v6, want) {
			t.Fatalf("expected %q in settings tab, got:\n%s", want, v6)
		}
	}
	m7 := send(t, m6, tea.KeyMsg{Type: tea.KeyEnter})
	if !strings.Contains(ansi.Strip(m7.View()), "unsaved") {
		t.Fatalf("expected unsaved indicator after change, got:\n%s", ansi.Strip(m7.View()))
	}
	m8 := send(t, m7, key('s'))
	v8 := ansi.Strip(m8.View())
	if !strings.Contains(v8, "saved config.toml") || strings.Contains(v8, "unsaved") {
		t.Fatalf("expected saved config status, got:\n%s", v8)
	}
	savedCfg, err := config.Load(cfgPath)
	if err != nil || savedCfg.Embeddings.Provider == "none" {
		t.Fatalf("expected updated embeddings provider persisted, got err=%v prov=%q", err, savedCfg.Embeddings.Provider)
	}
}

func TestTUIRejectNeedsConfirmAndCanCancel(t *testing.T) {
	f := newFixture(t)
	f.write(t, "projects/api.md", "# API\n")
	p, _, err := proposal.Add(f.vDir, proposal.Proposal{Kind: "append", Path: "projects/api.md", Content: "- x", Author: "codex"})
	if err != nil {
		t.Fatal(err)
	}
	m := send(t, f.model(), key('2'), key('x'), key('n'))
	if v := ansi.Strip(m.View()); !strings.Contains(v, "cancelled") || !strings.Contains(v, "2 Proposals (1)") {
		t.Fatalf("expected cancelled reject, got:\n%s", v)
	}
	m = send(t, m, key('x'), key('y'))
	if v := ansi.Strip(m.View()); !strings.Contains(v, "rejected "+p.ID) || !strings.Contains(v, "2 Proposals (0)") {
		t.Fatalf("expected confirmed reject, got:\n%s", v)
	}
}

func TestTUIListHasNoCapAndScrolls(t *testing.T) {
	f := newFixture(t)
	for i := 0; i < 45; i++ {
		f.write(t, fmt.Sprintf("projects/note-%02d.md", i), fmt.Sprintf("# Note %02d\nbody\n", i))
	}
	m := send(t, f.model(), tea.WindowSizeMsg{Width: 100, Height: 30})
	v := ansi.Strip(m.View())
	total := strings.Count(v, "note-") // only visible rows
	if total == 0 || !strings.Contains(v, "1/4") {
		t.Fatalf("expected list header with total count, got:\n%s", v)
	}
	m = send(t, m, key('G'))
	v = ansi.Strip(m.View())
	if !strings.Contains(v, "note-44.md") {
		t.Fatalf("expected last note reachable (no 30-note cap), got:\n%s", v)
	}
	if !strings.Contains(v, "/4") {
		t.Fatalf("expected position counter, got:\n%s", v)
	}
}

func TestTUITypingShowsCursorAndRunsSearch(t *testing.T) {
	f := newFixture(t)
	f.write(t, "projects/redis.md", "# Redis cache\nCache layer for the API.\n")
	f.write(t, "projects/web.md", "# Web\nFrontend.\n")
	m := send(t, f.model(), key('/'), key('r'), key('e'), key('d'))
	v := m.View()
	if !strings.Contains(ansi.Strip(v), "/ red") {
		t.Fatalf("expected query echoed while typing, got:\n%s", ansi.Strip(v))
	}
	if !strings.Contains(ansi.Strip(v), "enter search") {
		t.Fatalf("expected typing key hints, got:\n%s", ansi.Strip(v))
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyEnter})
	v = ansi.Strip(m.View())
	if !strings.Contains(v, "Results") || !strings.Contains(v, "redis.md") {
		t.Fatalf("expected search results, got:\n%s", v)
	}
}

func TestTUIPreviewWrapsAndScrolls(t *testing.T) {
	f := newFixture(t)
	var sb strings.Builder
	sb.WriteString("---\nstatus: active\nupdated: 2026-11-23\n---\n# Long note\n\n")
	for i := 0; i < 80; i++ {
		fmt.Fprintf(&sb, "- line %02d with a fairly long sentence that should wrap inside a narrow preview pane cleanly\n", i)
	}
	sb.WriteString("\n```bash\nmemtwig search deploy\n```\n")
	f.write(t, "projects/long.md", sb.String())

	m := send(t, f.model(), tea.WindowSizeMsg{Width: 100, Height: 30}, key('j'))
	v := ansi.Strip(m.View())
	if !strings.Contains(v, "Long note") || !strings.Contains(v, "status active") || !strings.Contains(v, "%") {
		t.Fatalf("expected frontmatter and scroll indicator, got:\n%s", v)
	}
	before := v
	m = send(t, m, key('J'), key('J'))
	after := ansi.Strip(m.View())
	if before == after || !strings.Contains(after, "line ") {
		t.Fatalf("expected preview to scroll")
	}
	// Mouse wheel over the preview scrolls too.
	m2 := send(t, m, tea.MouseMsg{X: 80, Y: 10, Button: tea.MouseButtonWheelDown, Action: tea.MouseActionPress})
	if ansi.Strip(m2.View()) == after {
		t.Fatalf("expected wheel to scroll preview")
	}
	m3 := send(t, m2, key('J'), key('J'), key('J'), key('J'), key('J'), key('J'), key('J'), key('J'), key('J'), key('J'))
	if v := ansi.Strip(m3.View()); !strings.Contains(v, "memtwig search deploy") || !strings.Contains(v, "100%") {
		t.Fatalf("expected to reach end of note with code block, got:\n%s", v)
	}
}

func TestTUIDiffScrolls(t *testing.T) {
	f := newFixture(t)
	f.write(t, "projects/api.md", "# API\n")
	var content strings.Builder
	for i := 0; i < 70; i++ {
		fmt.Fprintf(&content, "- added line %02d\n", i)
	}
	if _, _, err := proposal.Add(f.vDir, proposal.Proposal{Kind: "append", Path: "projects/api.md", Content: content.String(), Author: "claude"}); err != nil {
		t.Fatal(err)
	}
	m := send(t, f.model(), tea.WindowSizeMsg{Width: 100, Height: 30}, key('2'))
	v := ansi.Strip(m.View())
	if strings.Contains(v, "added line 69") {
		t.Fatalf("diff should not fit before scrolling")
	}
	for i := 0; i < 10; i++ {
		m = send(t, m, key('J'))
	}
	v = ansi.Strip(m.View())
	if !strings.Contains(v, "added line 69") || !strings.Contains(v, "100%") {
		t.Fatalf("expected diff scrolled to end, got:\n%s", v)
	}
	m = send(t, m, tea.KeyMsg{Type: tea.KeyPgUp})
	if strings.Contains(ansi.Strip(m.View()), "100%") {
		t.Fatalf("expected pgup to scroll diff back")
	}
}

func TestTUIMouseTabsAndNarrowLayout(t *testing.T) {
	f := newFixture(t)
	f.write(t, "projects/api.md", "# API\nRuns on Fly.io with a very long line that needs wrapping on a small terminal window.\n")

	for _, size := range [][2]int{{60, 20}, {44, 16}, {120, 40}} {
		m := send(t, f.model(), tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		for _, k := range []rune{'1', '2', '3', '4', '?'} {
			m = send(t, m, key(k))
			assertFits(t, m.View(), size[0], size[1])
			if k == '?' {
				m = send(t, m, key('?'))
			}
		}
	}

	m := send(t, f.model(), tea.WindowSizeMsg{Width: 120, Height: 30})
	// Find the "4 Settings" tab in the first row and click it.
	first := ansi.Strip(strings.Split(m.View(), "\n")[0])
	x := strings.Index(first, "4 Settings")
	if x < 0 {
		t.Fatalf("tab bar missing settings: %q", first)
	}
	m = send(t, m, tea.MouseMsg{X: x + 2, Y: 0, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	if !strings.Contains(ansi.Strip(m.View()), "Harnesses") {
		t.Fatalf("expected click on tab to open settings")
	}
}

func TestTUIHelpListsEveryTab(t *testing.T) {
	f := newFixture(t)
	m := send(t, f.model(), key('?'))
	v := ansi.Strip(m.View())
	for _, want := range []string{"Settings", "1 2 3 4", "J K", "mouse", "Approve selected", "ctrl+w", "s "} {
		if !strings.Contains(v, want) {
			t.Fatalf("help missing %q:\n%s", want, v)
		}
	}
}
