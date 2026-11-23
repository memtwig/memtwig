package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/memtwig/memtwig/internal/audit"
	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/curator"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/install"
	"github.com/memtwig/memtwig/internal/proposal"
	"github.com/memtwig/memtwig/internal/scope"
	"github.com/memtwig/memtwig/internal/search"
	"github.com/memtwig/memtwig/internal/vault"
)

type Tab int

const (
	TabSearch Tab = iota
	TabProposals
	TabVaults
	TabSettings
)

type ProposalItem struct {
	VaultName string
	VaultRoot string
	Proposal  proposal.Proposal
}

type Model struct {
	cfg           *config.Config
	cfgPath       string
	store         *index.Store
	auditLog      *audit.Logger
	activeVaults  []scope.ActiveVault
	version       string
	tab           Tab
	width, height int

	// Search state
	query         string
	typingQuery   bool
	results       []search.Result
	selectedIdx   int
	previewText   string
	previewScroll int

	// Proposals state
	proposals    []ProposalItem
	selectedProp int

	// Audit state
	auditEvents []audit.Event

	// Settings state
	settingIdx int

	showHelp  bool
	statusMsg string
}

var (
	teal       = lipgloss.Color("#1a7f78")
	tealLight  = lipgloss.Color("#5fd3c7")
	muted      = lipgloss.Color("#6b7f7d")
	borderCol  = lipgloss.Color("#2e4f4d")
	greenDiff  = lipgloss.Color("#22c55e")
	redDiff    = lipgloss.Color("#ef4444")

	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(tealLight)
	tabActive   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff")).Background(teal).Padding(0, 2)
	tabInactive = lipgloss.NewStyle().Foreground(muted).Padding(0, 2)
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderCol).Padding(0, 1)
	selStyle    = lipgloss.NewStyle().Bold(true).Foreground(tealLight)
	mutedStyle  = lipgloss.NewStyle().Foreground(muted)
	statusStyle = lipgloss.NewStyle().Foreground(tealLight)
)

func New(cfg *config.Config, st *index.Store, aud *audit.Logger, cwd, explicitVault, version string) Model {
	_, _ = st.EnsureFresh(context.Background(), cfg)
	active := scope.Resolve(cfg, cwd, explicitVault)
	m := Model{
		cfg:          cfg,
		store:        st,
		auditLog:     aud,
		activeVaults: active,
		version:      version,
		tab:          TabSearch,
		width:        110,
		height:       34,
	}
	m.refreshProposals()
	m.refreshAudit()
	m.loadDefaultNotes()
	return m
}

func Run(cfg *config.Config, st *index.Store, aud *audit.Logger, cwd, explicitVault, version string) error {
	return RunWithConfigPath(cfg, "", st, aud, cwd, explicitVault, version)
}

func RunWithConfigPath(cfg *config.Config, cfgPath string, st *index.Store, aud *audit.Logger, cwd, explicitVault, version string) error {
	m := New(cfg, st, aud, cwd, explicitVault, version)
	m.cfgPath = cfgPath
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func (m Model) WithConfigPath(p string) Model {
	m.cfgPath = p
	return m
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		k := msg.String()
		if k == "ctrl+c" {
			return m, tea.Quit
		}

		if m.showHelp {
			if k == "?" || k == "esc" || k == "q" || k == "enter" {
				m.showHelp = false
			}
			return m, nil
		}

		if m.typingQuery {
			switch k {
			case "esc":
				m.typingQuery = false
				return m, nil
			case "ctrl+u":
				m.query = ""
				return m, nil
			case "enter":
				m.typingQuery = false
				m.runSearch()
				return m, nil
			case "backspace":
				if len(m.query) > 0 {
					r := []rune(m.query)
					m.query = string(r[:len(r)-1])
				}
				return m, nil
			default:
				if len(msg.Runes) > 0 {
					m.query += string(msg.Runes)
				}
				return m, nil
			}
		}

		switch k {
		case "q":
			return m, tea.Quit
		case "?":
			m.showHelp = true
			return m, nil
		case "esc":
			if m.tab == TabSearch && m.query != "" {
				m.query = ""
				m.loadDefaultNotes()
				m.statusMsg = "cleared search"
			}
			return m, nil
		case "1":
			m.tab = TabSearch
			return m, nil
		case "2":
			m.tab = TabProposals
			m.refreshProposals()
			return m, nil
		case "3":
			m.tab = TabVaults
			m.refreshAudit()
			return m, nil
		case "4":
			m.tab = TabSettings
			return m, nil
		case "tab":
			m.tab = (m.tab + 1) % 4
			if m.tab == TabProposals {
				m.refreshProposals()
			} else if m.tab == TabVaults {
				m.refreshAudit()
			}
			return m, nil
		case "shift+tab":
			m.tab = (m.tab + 3) % 4
			if m.tab == TabProposals {
				m.refreshProposals()
			} else if m.tab == TabVaults {
				m.refreshAudit()
			}
			return m, nil
		case "/":
			m.tab = TabSearch
			m.typingQuery = true
			return m, nil
		case "i":
			stats, err := m.store.EnsureFresh(context.Background(), m.cfg)
			if err != nil {
				m.statusMsg = "reindex error: " + err.Error()
			} else {
				m.statusMsg = fmt.Sprintf("reindexed: %d updated, %d skipped, %d removed", stats.Indexed, stats.Skipped, stats.Removed)
				if m.query != "" {
					m.runSearch()
				} else {
					m.loadDefaultNotes()
				}
			}
			return m, nil
		case "c":
			created := 0
			for _, av := range m.activeVaults {
				if !av.FullAccess {
					continue
				}
				rep, err := curator.Curate(av.Name, config.ExpandHome(av.Config.Path), av.Config.MaxNoteBytes, 90)
				if err == nil {
					created += rep.ProposalsCreated
				}
			}
			m.refreshProposals()
			m.statusMsg = fmt.Sprintf("curate done: %d new proposal(s), %d open total", created, len(m.proposals))
			return m, nil
		case "up", "k":
			if m.tab == TabSearch && m.selectedIdx > 0 {
				m.selectedIdx--
				m.previewScroll = 0
				m.loadSelectedPreview()
			} else if m.tab == TabProposals && m.selectedProp > 0 {
				m.selectedProp--
			} else if m.tab == TabSettings && m.settingIdx > 0 {
				m.settingIdx--
			}
			return m, nil
		case "down", "j":
			if m.tab == TabSearch && m.selectedIdx < len(m.results)-1 {
				m.selectedIdx++
				m.previewScroll = 0
				m.loadSelectedPreview()
			} else if m.tab == TabProposals && m.selectedProp < len(m.proposals)-1 {
				m.selectedProp++
			} else if m.tab == TabSettings && m.settingIdx < 5 {
				m.settingIdx++
			}
			return m, nil
		case "enter", " ", "right", "l":
			if m.tab == TabSettings {
				m.applySettingAction(1)
			}
			return m, nil
		case "left", "h":
			if m.tab == TabSettings {
				m.applySettingAction(-1)
			}
			return m, nil
		case "s":
			if m.tab == TabSettings {
				if err := config.Save(m.cfgPath, m.cfg); err != nil {
					m.statusMsg = "save error: " + err.Error()
				} else {
					stats, _ := m.store.EnsureFresh(context.Background(), m.cfg)
					m.statusMsg = fmt.Sprintf("saved config.toml · reindexed %d note(s)", stats.Indexed)
				}
			}
			return m, nil
		case "J", "pgdown":
			if m.tab == TabSearch {
				m.previewScroll += 5
			}
			return m, nil
		case "K", "pgup":
			if m.tab == TabSearch && m.previewScroll > 0 {
				m.previewScroll -= 5
				if m.previewScroll < 0 {
					m.previewScroll = 0
				}
			}
			return m, nil
		case "a":
			if m.tab == TabProposals && len(m.proposals) > 0 && m.selectedProp < len(m.proposals) {
				item := m.proposals[m.selectedProp]
				if _, err := proposal.Approve(item.VaultRoot, item.Proposal.ID); err != nil {
					m.statusMsg = "approve error: " + err.Error()
				} else {
					_, _ = m.store.EnsureFresh(context.Background(), m.cfg)
					m.statusMsg = fmt.Sprintf("approved %s -> %s:%s", item.Proposal.ID, item.VaultName, item.Proposal.Path)
					m.refreshProposals()
				}
			}
			return m, nil
		case "x", "r":
			if m.tab == TabProposals && len(m.proposals) > 0 && m.selectedProp < len(m.proposals) {
				item := m.proposals[m.selectedProp]
				if _, err := proposal.Reject(item.VaultRoot, item.Proposal.ID, "rejected in tui"); err != nil {
					m.statusMsg = "reject error: " + err.Error()
				} else {
					m.statusMsg = fmt.Sprintf("rejected %s (%s:%s)", item.Proposal.ID, item.VaultName, item.Proposal.Path)
					m.refreshProposals()
				}
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) loadDefaultNotes() {
	m.results = nil
	for _, av := range m.activeVaults {
		notes, err := vault.WalkNotes(av.Name, config.ExpandHome(av.Config.Path))
		if err != nil {
			continue
		}
		for _, n := range notes {
			if !av.AllowsPath(n.RelPath) {
				continue
			}
			m.results = append(m.results, search.Result{
				Vault:   av.Name,
				Path:    n.RelPath,
				Title:   n.Title,
				Snippet: n.Updated,
			})
			if len(m.results) >= 30 {
				break
			}
		}
	}
	m.selectedIdx = 0
	m.loadSelectedPreview()
}

func (m *Model) runSearch() {
	q := strings.TrimSpace(m.query)
	if q == "" {
		m.loadDefaultNotes()
		m.statusMsg = "showing all notes in scope"
		return
	}
	res, warn, err := search.Query(context.Background(), m.store, m.cfg, m.activeVaults, q, search.Options{TopK: 20})
	if err != nil {
		m.statusMsg = "search error: " + err.Error()
		return
	}
	m.results = res
	m.selectedIdx = 0
	m.previewScroll = 0
	m.loadSelectedPreview()
	if warn != "" {
		m.statusMsg = warn
	} else {
		m.statusMsg = fmt.Sprintf("%d match(es) for %q", len(res), q)
	}
}

func (m *Model) loadSelectedPreview() {
	if len(m.results) == 0 || m.selectedIdx >= len(m.results) {
		m.previewText = "(no note selected)"
		return
	}
	sel := m.results[m.selectedIdx]
	for _, av := range m.activeVaults {
		if av.Name == sel.Vault {
			abs := filepath.Join(config.ExpandHome(av.Config.Path), filepath.FromSlash(sel.Path))
			raw, err := os.ReadFile(abs)
			if err != nil {
				m.previewText = "read error: " + err.Error()
				return
			}
			m.previewText = string(raw)
			return
		}
	}
	m.previewText = "(vault not found)"
}

func (m *Model) refreshProposals() {
	m.proposals = nil
	for _, av := range m.activeVaults {
		root := config.ExpandHome(av.Config.Path)
		props, err := proposal.List(root, false)
		if err != nil {
			continue
		}
		for _, p := range props {
			m.proposals = append(m.proposals, ProposalItem{
				VaultName: av.Name,
				VaultRoot: root,
				Proposal:  p,
			})
		}
	}
	if m.selectedProp >= len(m.proposals) && len(m.proposals) > 0 {
		m.selectedProp = len(m.proposals) - 1
	}
	if len(m.proposals) == 0 {
		m.selectedProp = 0
	}
}

func (m *Model) refreshAudit() {
	if m.auditLog == nil {
		m.auditEvents = nil
		return
	}
	evs, _ := m.auditLog.Tail(12)
	m.auditEvents = evs
}

func (m Model) View() string {
	w := m.width
	if w < 60 {
		w = 80
	}
	h := m.height
	if h < 18 {
		h = 24
	}

	// Top header + tabs
	t1 := tabInactive.Render("1 Search")
	t2 := tabInactive.Render(fmt.Sprintf("2 Proposals (%d)", len(m.proposals)))
	t3 := tabInactive.Render(fmt.Sprintf("3 Vaults (%d)", len(m.activeVaults)))
	t4 := tabInactive.Render("4 Settings")
	switch m.tab {
	case TabSearch:
		t1 = tabActive.Render("1 Search")
	case TabProposals:
		t2 = tabActive.Render(fmt.Sprintf("2 Proposals (%d)", len(m.proposals)))
	case TabVaults:
		t3 = tabActive.Render(fmt.Sprintf("3 Vaults (%d)", len(m.activeVaults)))
	case TabSettings:
		t4 = tabActive.Render("4 Settings")
	}
	topBar := lipgloss.JoinHorizontal(lipgloss.Center,
		headerStyle.Render("memtwig "+m.version),
		"   ",
		t1, " ", t2, " ", t3, " ", t4,
	)

	bodyH := h - 6
	if bodyH < 10 {
		bodyH = 10
	}

	var body string
	if m.showHelp {
		body = m.viewHelp(w, bodyH)
	} else {
		switch m.tab {
		case TabSearch:
			body = m.viewSearch(w, bodyH)
		case TabProposals:
			body = m.viewProposals(w, bodyH)
		case TabVaults:
			body = m.viewVaults(w, bodyH)
		case TabSettings:
			body = m.viewSettings(w, bodyH)
		}
	}

	var helpText string
	switch {
	case m.showHelp:
		helpText = "Esc / ? close help"
	case m.typingQuery:
		helpText = "Enter run search · Ctrl+U clear · Esc cancel"
	case m.tab == TabSearch:
		helpText = "/ search · Esc clear · j/k select · J/K scroll · 1-4 tabs · i reindex · ? help · q quit"
	case m.tab == TabProposals:
		helpText = "j/k select · a approve · x reject · c curate · 1-4 tabs · ? help · q quit"
	case m.tab == TabSettings:
		helpText = "j/k select · Enter/←/→ cycle or connect · s save & reindex · 1-4 tabs · q quit"
	default:
		helpText = "i reindex · c curate · 1-4 tabs · ? help · q quit"
	}
	help := mutedStyle.Render(helpText)
	status := ""
	if m.statusMsg != "" {
		status = statusStyle.Render("▸ " + m.statusMsg)
	}

	parts := []string{topBar, body}
	if status != "" {
		parts = append(parts, status)
	}
	parts = append(parts, help)
	return strings.Join(parts, "\n")
}

func (m Model) viewHelp(w, h int) string {
	lines := []string{
		selStyle.Render("Keyboard Shortcuts"),
		"",
		"  Navigation",
		"    1 / 2 / 3        Switch between Search, Proposals, Vaults",
		"    Tab / Shift+Tab  Cycle tabs forward / backward",
		"    j / k  or ↑ / ↓  Move selection",
		"    J / K  or PgUp/Dn Scroll note preview",
		"",
		"  Search & Maintenance",
		"    /                Focus search bar (prefix matching supported)",
		"    Ctrl+U           Clear search input while typing",
		"    Esc              Cancel typing or reset active search",
		"    i                Incremental reindex",
		"    c                Run deterministic curator pass",
		"",
		"  Proposals",
		"    a                Approve selected proposal and reindex",
		"    x / r            Reject selected proposal (archived, never reopens)",
		"",
		"  General",
		"    ?                Toggle this help screen",
		"    q / Ctrl+C       Quit",
	}
	return boxStyle.Width(w - 2).Height(h - 1).Render(strings.Join(lines, "\n"))
}

func (m Model) viewSearch(w, h int) string {
	promptLabel := "Search (/): "
	if m.typingQuery {
		promptLabel = "Search [typing, Enter to run]: "
	}
	qBar := boxStyle.Width(w - 2).Render(promptLabel + m.query)

	leftW := w*38/100 - 2
	rightW := w - leftW - 6
	paneH := h - 3
	if paneH < 6 {
		paneH = 6
	}

	var leftLines []string
	if len(m.results) == 0 {
		leftLines = append(leftLines, selStyle.Render("Notes (0)"))
		leftLines = append(leftLines, mutedStyle.Render("No notes found in active scope."))
	} else {
		leftLines = append(leftLines, selStyle.Render(fmt.Sprintf("Notes (%d/%d)", m.selectedIdx+1, len(m.results))))
		maxRows := paneH - 2
		start := 0
		if m.selectedIdx >= maxRows {
			start = m.selectedIdx - maxRows + 1
		}
		for i := start; i < len(m.results) && len(leftLines)-1 < maxRows; i++ {
			r := m.results[i]
			label := truncate(fmt.Sprintf("[%s] %s", r.Vault, r.Path), leftW-4)
			if i == m.selectedIdx {
				leftLines = append(leftLines, selStyle.Render("› "+label))
			} else {
				leftLines = append(leftLines, "  "+label)
			}
		}
	}

	previewLines := strings.Split(m.previewText, "\n")
	start := m.previewScroll
	if start >= len(previewLines) {
		start = max(0, len(previewLines)-1)
	}
	end := start + paneH - 2
	if end > len(previewLines) {
		end = len(previewLines)
	}
	var rightLines []string
	if len(m.results) > 0 && m.selectedIdx < len(m.results) {
		sel := m.results[m.selectedIdx]
		rightLines = append(rightLines, selStyle.Render(truncate(fmt.Sprintf("%s:%s", sel.Vault, sel.Path), rightW-2)))
	}
	for _, l := range previewLines[start:end] {
		rightLines = append(rightLines, truncate(l, rightW-2))
	}

	left := boxStyle.Width(leftW).Height(paneH).Render(strings.Join(leftLines, "\n"))
	right := boxStyle.Width(rightW).Height(paneH).Render(strings.Join(rightLines, "\n"))
	return qBar + "\n" + lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m Model) viewProposals(w, h int) string {
	leftW := w*38/100 - 2
	rightW := w - leftW - 6
	paneH := h - 1

	var leftLines []string
	if len(m.proposals) == 0 {
		leftLines = append(leftLines, mutedStyle.Render("No open proposals. Press 'c' to run curate."))
	} else {
		for i, item := range m.proposals {
			if i >= paneH-1 {
				break
			}
			label := truncate(fmt.Sprintf("%s [%s] %s", item.Proposal.ID[:6], item.VaultName, item.Proposal.Path), leftW-4)
			if i == m.selectedProp {
				leftLines = append(leftLines, selStyle.Render("› "+label))
			} else {
				leftLines = append(leftLines, "  "+label)
			}
		}
	}

	var rightLines []string
	if len(m.proposals) > 0 && m.selectedProp < len(m.proposals) {
		item := m.proposals[m.selectedProp]
		p := item.Proposal
		rightLines = append(rightLines,
			selStyle.Render(fmt.Sprintf("%s (%s) · %s:%s", p.ID, p.Kind, item.VaultName, p.Path)),
			mutedStyle.Render(fmt.Sprintf("author: %s · created: %s · reason: %s", p.Author, p.CreatedAt, p.Reason)),
			"",
		)
		diff := proposal.FormatDiff(item.VaultRoot, p)
		for _, dl := range strings.Split(diff, "\n") {
			if len(rightLines) >= paneH-1 {
				break
			}
			dl = truncate(dl, rightW-2)
			switch {
			case strings.HasPrefix(dl, "+ "):
				rightLines = append(rightLines, lipgloss.NewStyle().Foreground(greenDiff).Render(dl))
			case strings.HasPrefix(dl, "- "):
				rightLines = append(rightLines, lipgloss.NewStyle().Foreground(redDiff).Render(dl))
			case strings.HasPrefix(dl, "@@"):
				rightLines = append(rightLines, lipgloss.NewStyle().Foreground(tealLight).Render(dl))
			default:
				rightLines = append(rightLines, dl)
			}
		}
	} else {
		rightLines = append(rightLines, mutedStyle.Render("Select a proposal to inspect its diff. Press 'a' to approve or 'x' to reject."))
	}

	left := boxStyle.Width(leftW).Height(paneH).Render(strings.Join(leftLines, "\n"))
	right := boxStyle.Width(rightW).Height(paneH).Render(strings.Join(rightLines, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, " ", right)
}

func (m Model) viewVaults(w, h int) string {
	var lines []string
	lines = append(lines, selStyle.Render("Active Vaults in Current Scope"))
	if len(m.activeVaults) == 0 {
		lines = append(lines, mutedStyle.Render("No vaults configured. Run: memtwig init ~/notes --name personal"))
	} else {
		for _, av := range m.activeVaults {
			access := "full"
			if !av.FullAccess {
				access = strings.Join(av.PathGlobs, ",")
			}
			lines = append(lines, fmt.Sprintf("  %-14s kind=%-8s write=%-8s access=%-16s reason=%s",
				av.Name, av.Config.Kind, av.Config.DefaultWrite, access, av.MatchReason))
		}
	}

	lines = append(lines, "", selStyle.Render("Recent Audit Events"))
	if len(m.auditEvents) == 0 {
		lines = append(lines, mutedStyle.Render("  (no audit events recorded yet)"))
	} else {
		for _, ev := range m.auditEvents {
			lines = append(lines, truncate(fmt.Sprintf("  %s  %-10s %-14s %s %s (%s)",
				ev.Timestamp, ev.Actor, ev.Action, ev.Vault, ev.Path, ev.Outcome), w-6))
		}
	}
	return boxStyle.Width(w - 2).Height(h - 1).Render(strings.Join(lines, "\n"))
}

func (m *Model) applySettingAction(dir int) {
	embProviders := []struct{ p, model, env, endpoint string }{
		{"none", "", "", ""},
		{"ollama", "bge-m3", "", "http://127.0.0.1:11434/api/embeddings"},
		{"openrouter", "baai/bge-m3", "OPENROUTER_API_KEY", ""},
		{"deepinfra", "BAAI/bge-m3", "DEEPINFRA_API_KEY", ""},
		{"openai", "text-embedding-3-small", "OPENAI_API_KEY", ""},
		{"cloudflare", "@cf/baai/bge-m3", "CLOUDFLARE_API_TOKEN", ""},
	}
	llmProviders := []struct{ p, model, env string }{
		{"none", "", ""},
		{"groq", "llama-3.3-70b-versatile", "GROQ_API_KEY"},
		{"openrouter", "openai/gpt-4o-mini", "OPENROUTER_API_KEY"},
		{"openai", "gpt-4o-mini", "OPENAI_API_KEY"},
		{"ollama", "llama3.2", ""},
	}
	harnesses := []string{"claude", "cursor", "codex", "antigravity"}

	switch m.settingIdx {
	case 0:
		cur := 0
		for i, ep := range embProviders {
			if ep.p == m.cfg.Embeddings.Provider {
				cur = i
				break
			}
		}
		next := (cur + dir + len(embProviders)) % len(embProviders)
		p := embProviders[next]
		m.cfg.Embeddings.Provider = p.p
		m.cfg.Embeddings.Model = p.model
		m.cfg.Embeddings.APIKeyEnv = p.env
		m.cfg.Embeddings.Endpoint = p.endpoint
		if m.cfg.Embeddings.Dimensions <= 0 {
			m.cfg.Embeddings.Dimensions = 1024
		}
		m.statusMsg = fmt.Sprintf("embeddings -> %s (%s) · press 's' to save", p.p, p.model)
	case 1:
		cur := 0
		for i, lp := range llmProviders {
			if lp.p == m.cfg.LLM.Provider {
				cur = i
				break
			}
		}
		next := (cur + dir + len(llmProviders)) % len(llmProviders)
		p := llmProviders[next]
		m.cfg.LLM.Provider = p.p
		m.cfg.LLM.Model = p.model
		m.cfg.LLM.APIKeyEnv = p.env
		m.statusMsg = fmt.Sprintf("llm curator -> %s (%s) · press 's' to save", p.p, p.model)
	case 2, 3, 4, 5:
		hID := harnesses[m.settingIdx-2]
		res, err := install.Install(hID, "", "", false)
		if err != nil {
			m.statusMsg = "connect " + hID + " error: " + err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("connected %s (%s)", res.Harness, res.ConfigPath)
		}
	}
}

func envBadge(varName string) string {
	if strings.TrimSpace(varName) == "" {
		return "no key needed"
	}
	if strings.TrimSpace(os.Getenv(varName)) != "" {
		return lipgloss.NewStyle().Foreground(greenDiff).Render(varName + " [SET]")
	}
	return lipgloss.NewStyle().Foreground(redDiff).Render(varName + " [MISSING]")
}

func (m Model) viewSettings(w, h int) string {
	home, _ := os.UserHomeDir()
	harnessRows := []struct{ id, label, rel string }{
		{"claude", "Claude Code", ".claude"},
		{"cursor", "Cursor", ".cursor"},
		{"codex", "Codex", ".codex"},
		{"antigravity", "Antigravity", ".gemini"},
	}

	rows := []string{
		fmt.Sprintf("Embeddings provider : %-12s model=%-22s dim=%d  %s",
			m.cfg.Embeddings.Provider, m.cfg.Embeddings.Model, m.cfg.Embeddings.Dimensions, envBadge(m.cfg.Embeddings.APIKeyEnv)),
		fmt.Sprintf("LLM curator provider: %-12s model=%-22s %s",
			m.cfg.LLM.Provider, m.cfg.LLM.Model, envBadge(m.cfg.LLM.APIKeyEnv)),
	}
	for _, hr := range harnessRows {
		state := "not detected"
		if _, err := os.Stat(filepath.Join(home, hr.rel)); err == nil {
			state = "detected"
		}
		rows = append(rows, fmt.Sprintf("Connect harness     : %-12s (%s)  [Enter to install hooks/MCP]", hr.label, state))
	}

	lines := []string{
		selStyle.Render("Providers & AI Harnesses (no manual config.toml editing needed)"),
		mutedStyle.Render("Use j/k to select, Enter or ←/→ to change provider / connect harness, 's' to save & reindex."),
		"",
	}
	for i, r := range rows {
		if i == 2 {
			lines = append(lines, "", selStyle.Render("AI Harnesses"))
		}
		if i == m.settingIdx {
			lines = append(lines, selStyle.Render("› "+r))
		} else {
			lines = append(lines, "  "+r)
		}
	}
	return boxStyle.Width(w - 2).Height(h - 1).Render(strings.Join(lines, "\n"))
}

func truncate(s string, maxLen int) string {
	if maxLen <= 3 {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen-1]) + "…"
}
