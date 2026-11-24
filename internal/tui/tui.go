package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

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

const tabCount = 4

// searchTopK bounds query results; the list scrolls, so this is a ceiling, not a page size.
const searchTopK = 200

type ProposalItem struct {
	VaultName string
	VaultRoot string
	Proposal  proposal.Proposal
}

type harnessRow struct{ id, label, rel string }

var harnessRows = []harnessRow{
	{"claude", "Claude Code", ".claude"},
	{"cursor", "Cursor", ".cursor"},
	{"codex", "Codex", ".codex"},
	{"antigravity", "Antigravity", ".gemini"},
	{"hermes", "Hermes", ".hermes"},
}

// settingsRowCount = embeddings + curator + harnesses.
var settingsRowCount = 2 + len(harnessRows)

type pendingAction struct {
	kind string // approve | reject
	id   string
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
	diffScroll   int
	pending      *pendingAction

	// Vaults & audit state
	auditEvents []audit.Event
	vaultNotes  map[string]int
	vaultScroll int

	// Settings state
	settingIdx int
	dirty      bool
	quitArmed  bool

	showHelp   bool
	helpScroll int
	statusMsg  string
}

var (
	mint      = lipgloss.Color("#58cc7d")
	mintSoft  = lipgloss.Color("#7ee09e")
	ink       = lipgloss.Color("#141415")
	fg        = lipgloss.Color("#f5f5f7")
	fgSoft    = lipgloss.Color("#c9c9d1")
	muted     = lipgloss.Color("#8a8a96")
	faint     = lipgloss.Color("#55555f")
	borderCol = lipgloss.Color("#2f2f36")
	sky       = lipgloss.Color("#6bb5ff")
	lavender  = lipgloss.Color("#d5a8f8")
	amber     = lipgloss.Color("#ebb85e")
	coral     = lipgloss.Color("#f08a7a")

	brandStyle   = lipgloss.NewStyle().Bold(true).Foreground(mint)
	versionStyle = lipgloss.NewStyle().Foreground(muted)
	tabActive    = lipgloss.NewStyle().Bold(true).Foreground(ink).Background(mint)
	tabInactive  = lipgloss.NewStyle().Foreground(muted)
	borderStyle  = lipgloss.NewStyle().Foreground(borderCol)
	focusBorder  = lipgloss.NewStyle().Foreground(lipgloss.Color("#3d6b4c"))
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(fg)
	selStyle     = lipgloss.NewStyle().Bold(true).Foreground(mint)
	textStyle    = lipgloss.NewStyle().Foreground(fgSoft)
	mutedStyle   = lipgloss.NewStyle().Foreground(muted)
	faintStyle   = lipgloss.NewStyle().Foreground(faint)
	statusStyle  = lipgloss.NewStyle().Foreground(mintSoft)
	warnStyle    = lipgloss.NewStyle().Foreground(amber)
	addStyle     = lipgloss.NewStyle().Foreground(mint)
	delStyle     = lipgloss.NewStyle().Foreground(coral)
	hunkStyle    = lipgloss.NewStyle().Foreground(sky)
	keyStyle     = lipgloss.NewStyle().Foreground(fg).Bold(true)
	cursorStyle  = lipgloss.NewStyle().Foreground(ink).Background(mint)

	mdH1     = lipgloss.NewStyle().Bold(true).Foreground(mint)
	mdH2     = lipgloss.NewStyle().Bold(true).Foreground(fg)
	mdH3     = lipgloss.NewStyle().Bold(true).Foreground(fgSoft)
	mdCode   = lipgloss.NewStyle().Foreground(sky)
	mdGutter = lipgloss.NewStyle().Foreground(lipgloss.Color("#2f4a63"))
	mdInline = lipgloss.NewStyle().Foreground(amber)
	mdQuote  = lipgloss.NewStyle().Foreground(lavender)
	mdFront  = lipgloss.NewStyle().Foreground(faint)
	mdFrontK = lipgloss.NewStyle().Foreground(muted)

	vaultPalette = []lipgloss.Color{mint, sky, lavender, amber, coral}

	reInlineCode = regexp.MustCompile("`[^`]+`")
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
	m.refreshVaults()
	m.loadDefaultNotes()
	return m
}

func Run(cfg *config.Config, st *index.Store, aud *audit.Logger, cwd, explicitVault, version string) error {
	return RunWithConfigPath(cfg, "", st, aud, cwd, explicitVault, version)
}

func RunWithConfigPath(cfg *config.Config, cfgPath string, st *index.Store, aud *audit.Logger, cwd, explicitVault, version string) error {
	m := New(cfg, st, aud, cwd, explicitVault, version)
	m.cfgPath = cfgPath
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func (m Model) WithConfigPath(p string) Model {
	m.cfgPath = p
	return m
}

func (m Model) Init() tea.Cmd { return nil }

// ---------------------------------------------------------------------------
// Layout geometry (shared by View and mouse hit-testing)
// ---------------------------------------------------------------------------

type paneGeom struct {
	stacked             bool
	listX, listY, listW int
	listH               int
	detailX, detailY    int
	detailW, detailH    int
}

func (m Model) dims() (w, h int) {
	w, h = m.width, m.height
	if w <= 0 {
		w = 100
	}
	if h <= 0 {
		h = 30
	}
	if w < 32 {
		w = 32
	}
	if h < 12 {
		h = 12
	}
	return w, h
}

// bodyHeight is the space between the tab bar and the two footer lines (status + keys).
func (m Model) bodyHeight() int {
	_, h := m.dims()
	return h - 3
}

// splitPanes lays out a list pane and a detail pane starting at row top with height paneH.
func splitPanes(w, top, paneH int) paneGeom {
	if w >= 80 {
		listW := w * 2 / 5
		if listW < 30 {
			listW = 30
		}
		if listW > 56 {
			listW = 56
		}
		return paneGeom{
			listX: 0, listY: top, listW: listW, listH: paneH,
			detailX: listW + 1, detailY: top, detailW: w - listW - 1, detailH: paneH,
		}
	}
	listH := paneH * 2 / 5
	if listH < 5 {
		listH = 5
	}
	if paneH-listH < 4 {
		listH = paneH - 4
		if listH < 3 {
			listH = 3
		}
	}
	return paneGeom{
		stacked: true,
		listX:   0, listY: top, listW: w, listH: listH,
		detailX: 0, detailY: top + listH, detailW: w, detailH: paneH - listH,
	}
}

// Body starts at row 1 (row 0 is the tab bar). Search has a 3-row query box first.
func (m Model) searchGeom() paneGeom {
	w, _ := m.dims()
	paneH := m.bodyHeight() - 3
	if paneH < 7 {
		paneH = 7
	}
	return splitPanes(w, 4, paneH)
}

func (m Model) proposalGeom() paneGeom {
	w, _ := m.dims()
	return splitPanes(w, 1, m.bodyHeight())
}

// listWindow returns the first visible row so the selection stays centred while scrolling.
func listWindow(selected, total, rows int) int {
	if rows <= 0 || total <= rows {
		return 0
	}
	start := selected - rows/2
	if start < 0 {
		start = 0
	}
	if start > total-rows {
		start = total - rows
	}
	return start
}

func clamp(v, lo, hi int) int {
	if hi < lo {
		hi = lo
	}
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScrolls()
		return m, nil
	case tea.MouseMsg:
		return m.handleMouse(msg)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if k == "ctrl+c" {
		return m, tea.Quit
	}

	if m.showHelp {
		switch k {
		case "?", "esc", "q", "enter":
			m.showHelp = false
		case "j", "down":
			m.helpScroll = clamp(m.helpScroll+1, 0, m.helpMaxScroll())
		case "k", "up":
			m.helpScroll = clamp(m.helpScroll-1, 0, m.helpMaxScroll())
		case "J", "pgdown", "G", "end":
			m.helpScroll = m.helpMaxScroll()
		case "K", "pgup", "g", "home":
			m.helpScroll = 0
		}
		return m, nil
	}

	if m.pending != nil {
		switch k {
		case "y", "enter":
			m.confirmPending()
		case "a":
			if m.pending.kind == "approve" {
				m.confirmPending()
			}
		case "x", "r":
			if m.pending.kind == "reject" {
				m.confirmPending()
			}
		default:
			m.pending = nil
			m.statusMsg = "cancelled"
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
		case "backspace", "ctrl+h":
			if len(m.query) > 0 {
				r := []rune(m.query)
				m.query = string(r[:len(r)-1])
			}
			return m, nil
		case "ctrl+w":
			q := strings.TrimRight(m.query, " ")
			if i := strings.LastIndex(q, " "); i >= 0 {
				m.query = q[:i+1]
			} else {
				m.query = ""
			}
			return m, nil
		default:
			if len(msg.Runes) > 0 {
				m.query += string(msg.Runes)
			}
			return m, nil
		}
	}

	if k != "q" {
		m.quitArmed = false
	}

	switch k {
	case "q":
		if m.dirty && !m.quitArmed {
			m.quitArmed = true
			m.statusMsg = "unsaved settings · press s to save or q again to quit"
			return m, nil
		}
		return m, tea.Quit
	case "?":
		m.showHelp = true
		m.helpScroll = 0
		return m, nil
	case "esc":
		if m.tab == TabSearch && m.query != "" {
			m.query = ""
			m.loadDefaultNotes()
			m.statusMsg = "cleared search"
		}
		return m, nil
	case "1", "2", "3", "4":
		m.switchTab(Tab(k[0] - '1'))
		return m, nil
	case "tab":
		m.switchTab((m.tab + 1) % tabCount)
		return m, nil
	case "shift+tab":
		m.switchTab((m.tab + tabCount - 1) % tabCount)
		return m, nil
	case "/":
		m.switchTab(TabSearch)
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
			m.refreshVaults()
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
		m.moveSelection(-1)
		return m, nil
	case "down", "j":
		m.moveSelection(1)
		return m, nil
	case "home", "g":
		m.moveSelection(-1 << 20)
		return m, nil
	case "end", "G":
		m.moveSelection(1 << 20)
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
		if m.tab == TabSettings || m.dirty {
			m.saveSettings()
		}
		return m, nil
	case "J", "pgdown", "ctrl+d":
		m.scrollDetail(m.detailPage())
		return m, nil
	case "K", "pgup", "ctrl+u":
		m.scrollDetail(-m.detailPage())
		return m, nil
	case "a":
		if m.tab == TabProposals {
			m.armPending("approve")
		}
		return m, nil
	case "x", "r":
		if m.tab == TabProposals {
			m.armPending("reject")
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.showHelp {
		switch {
		case msg.Button == tea.MouseButtonWheelDown:
			m.helpScroll = clamp(m.helpScroll+2, 0, m.helpMaxScroll())
		case msg.Button == tea.MouseButtonWheelUp:
			m.helpScroll = clamp(m.helpScroll-2, 0, m.helpMaxScroll())
		case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft:
			m.showHelp = false
		}
		return m, nil
	}
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		dir := 1
		if msg.Button == tea.MouseButtonWheelUp {
			dir = -1
		}
		m.wheel(msg.X, msg.Y, dir)
		return m, nil
	}
	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, nil
	}
	if msg.Y == 0 {
		for i, span := range m.tabSpans() {
			if msg.X >= span.x0 && msg.X < span.x1 {
				m.pending = nil
				m.switchTab(Tab(i))
				return m, nil
			}
		}
		return m, nil
	}
	if m.pending != nil {
		return m, nil
	}
	switch m.tab {
	case TabSearch:
		if msg.Y >= 1 && msg.Y <= 3 {
			m.typingQuery = true
			return m, nil
		}
		m.typingQuery = false
		g := m.searchGeom()
		if idx, ok := listHit(g, msg.X, msg.Y, m.selectedIdx, len(m.results)); ok {
			m.selectedIdx = idx
			m.previewScroll = 0
			m.loadSelectedPreview()
		}
	case TabProposals:
		g := m.proposalGeom()
		if idx, ok := listHit(g, msg.X, msg.Y, m.selectedProp, len(m.proposals)); ok {
			m.selectedProp = idx
			m.diffScroll = 0
		}
	case TabSettings:
		if idx, ok := m.settingsHit(msg.Y); ok {
			m.settingIdx = idx
		}
	}
	return m, nil
}

// listHit maps a click inside a list pane to an item index (first content row is a header).
func listHit(g paneGeom, x, y, selected, total int) (int, bool) {
	if x < g.listX || x >= g.listX+g.listW {
		return 0, false
	}
	rows := g.listH - 2
	row := y - (g.listY + 1)
	if row < 0 || row >= rows {
		return 0, false
	}
	idx := listWindow(selected, total, rows) + row
	if idx >= total {
		return 0, false
	}
	return idx, true
}

// wheel scrolls whichever pane sits under the pointer.
func (m *Model) wheel(x, y, dir int) {
	inList := func(g paneGeom) bool {
		return x >= g.listX && x < g.listX+g.listW && y >= g.listY && y < g.listY+g.listH
	}
	switch m.tab {
	case TabSearch:
		if inList(m.searchGeom()) {
			m.moveSelection(dir)
		} else {
			m.scrollDetail(3 * dir)
		}
	case TabProposals:
		if inList(m.proposalGeom()) {
			m.moveSelection(dir)
		} else {
			m.scrollDetail(3 * dir)
		}
	case TabVaults:
		m.scrollDetail(3 * dir)
	case TabSettings:
		m.moveSelection(dir)
	}
}

func (m *Model) switchTab(t Tab) {
	m.tab = t
	m.typingQuery = false
	switch t {
	case TabProposals:
		m.refreshProposals()
	case TabVaults:
		m.refreshVaults()
	}
}

func (m *Model) moveSelection(delta int) {
	switch m.tab {
	case TabSearch:
		if len(m.results) == 0 {
			return
		}
		next := clamp(m.selectedIdx+delta, 0, len(m.results)-1)
		if next != m.selectedIdx {
			m.selectedIdx = next
			m.previewScroll = 0
			m.loadSelectedPreview()
		}
	case TabProposals:
		if len(m.proposals) == 0 {
			return
		}
		next := clamp(m.selectedProp+delta, 0, len(m.proposals)-1)
		if next != m.selectedProp {
			m.selectedProp = next
			m.diffScroll = 0
		}
	case TabVaults:
		m.vaultScroll = clamp(m.vaultScroll+delta, 0, m.vaultMaxScroll())
	case TabSettings:
		m.settingIdx = clamp(m.settingIdx+delta, 0, settingsRowCount-1)
	}
}

func (m *Model) scrollDetail(delta int) {
	switch m.tab {
	case TabSearch:
		m.previewScroll = clamp(m.previewScroll+delta, 0, m.previewMaxScroll())
	case TabProposals:
		m.diffScroll = clamp(m.diffScroll+delta, 0, m.diffMaxScroll())
	case TabVaults:
		m.vaultScroll = clamp(m.vaultScroll+delta, 0, m.vaultMaxScroll())
	}
}

func (m Model) detailPage() int {
	switch m.tab {
	case TabSearch:
		return max(1, m.previewRows()-2)
	case TabProposals:
		return max(1, m.diffRows()-2)
	default:
		return max(1, m.bodyHeight()-4)
	}
}

func (m *Model) clampScrolls() {
	m.previewScroll = clamp(m.previewScroll, 0, m.previewMaxScroll())
	m.diffScroll = clamp(m.diffScroll, 0, m.diffMaxScroll())
	m.vaultScroll = clamp(m.vaultScroll, 0, m.vaultMaxScroll())
}

func (m *Model) armPending(kind string) {
	if len(m.proposals) == 0 || m.selectedProp >= len(m.proposals) {
		m.statusMsg = "no proposal selected"
		return
	}
	m.pending = &pendingAction{kind: kind, id: m.proposals[m.selectedProp].Proposal.ID}
}

func (m *Model) confirmPending() {
	pa := m.pending
	m.pending = nil
	if pa == nil {
		return
	}
	var item *ProposalItem
	for i := range m.proposals {
		if m.proposals[i].Proposal.ID == pa.id {
			item = &m.proposals[i]
			break
		}
	}
	if item == nil {
		m.statusMsg = "proposal no longer open"
		return
	}
	switch pa.kind {
	case "approve":
		if _, err := proposal.Approve(item.VaultRoot, item.Proposal.ID); err != nil {
			m.statusMsg = "approve error: " + err.Error()
			return
		}
		_, _ = m.store.EnsureFresh(context.Background(), m.cfg)
		m.statusMsg = fmt.Sprintf("approved %s -> %s:%s", item.Proposal.ID, item.VaultName, item.Proposal.Path)
	case "reject":
		if _, err := proposal.Reject(item.VaultRoot, item.Proposal.ID, "rejected in tui"); err != nil {
			m.statusMsg = "reject error: " + err.Error()
			return
		}
		m.statusMsg = fmt.Sprintf("rejected %s (%s:%s)", item.Proposal.ID, item.VaultName, item.Proposal.Path)
	}
	m.diffScroll = 0
	m.refreshProposals()
}

func (m *Model) saveSettings() {
	if err := config.Save(m.cfgPath, m.cfg); err != nil {
		m.statusMsg = "save error: " + err.Error()
		return
	}
	stats, _ := m.store.EnsureFresh(context.Background(), m.cfg)
	m.dirty = false
	m.quitArmed = false
	m.statusMsg = fmt.Sprintf("saved config.toml · reindexed %d note(s)", stats.Indexed)
}

// ---------------------------------------------------------------------------
// Data loading
// ---------------------------------------------------------------------------

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
		}
	}
	m.selectedIdx = 0
	m.previewScroll = 0
	m.loadSelectedPreview()
}

func (m *Model) runSearch() {
	q := strings.TrimSpace(m.query)
	if q == "" {
		m.loadDefaultNotes()
		m.statusMsg = fmt.Sprintf("showing all %d notes in scope", len(m.results))
		return
	}
	res, warn, err := search.Query(context.Background(), m.store, m.cfg, m.activeVaults, q, search.Options{TopK: searchTopK})
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
		m.previewText = ""
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
	m.diffScroll = clamp(m.diffScroll, 0, m.diffMaxScroll())
}

func (m *Model) refreshVaults() {
	m.vaultNotes = make(map[string]int, len(m.activeVaults))
	for _, av := range m.activeVaults {
		notes, err := vault.WalkNotes(av.Name, config.ExpandHome(av.Config.Path))
		if err != nil {
			m.vaultNotes[av.Name] = -1
			continue
		}
		count := 0
		for _, n := range notes {
			if av.AllowsPath(n.RelPath) {
				count++
			}
		}
		m.vaultNotes[av.Name] = count
	}
	if m.auditLog == nil {
		m.auditEvents = nil
	} else {
		evs, _ := m.auditLog.Tail(40)
		m.auditEvents = evs
	}
	m.vaultScroll = clamp(m.vaultScroll, 0, m.vaultMaxScroll())
}

// ---------------------------------------------------------------------------
// Settings
// ---------------------------------------------------------------------------

var embProviders = []struct{ p, model, env, endpoint string }{
	{"none", "", "", ""},
	{"ollama", "bge-m3", "", "http://127.0.0.1:11434/api/embeddings"},
	{"openrouter", "baai/bge-m3", "OPENROUTER_API_KEY", ""},
	{"deepinfra", "BAAI/bge-m3", "DEEPINFRA_API_KEY", ""},
	{"openai", "text-embedding-3-small", "OPENAI_API_KEY", ""},
	{"cloudflare", "@cf/baai/bge-m3", "CLOUDFLARE_API_TOKEN", ""},
}

var llmProviders = []struct{ p, model, env string }{
	{"none", "", ""},
	{"groq", "llama-3.3-70b-versatile", "GROQ_API_KEY"},
	{"openrouter", "openai/gpt-4o-mini", "OPENROUTER_API_KEY"},
	{"openai", "gpt-4o-mini", "OPENAI_API_KEY"},
	{"ollama", "llama3.2", ""},
}

func (m *Model) applySettingAction(dir int) {
	switch m.settingIdx {
	case 0:
		cur := 0
		for i, ep := range embProviders {
			if ep.p == m.cfg.Embeddings.Provider {
				cur = i
				break
			}
		}
		p := embProviders[(cur+dir+len(embProviders))%len(embProviders)]
		m.cfg.Embeddings.Provider = p.p
		m.cfg.Embeddings.Model = p.model
		m.cfg.Embeddings.APIKeyEnv = p.env
		m.cfg.Embeddings.Endpoint = p.endpoint
		if m.cfg.Embeddings.Dimensions <= 0 {
			m.cfg.Embeddings.Dimensions = 1024
		}
		m.dirty = true
		m.statusMsg = fmt.Sprintf("embeddings -> %s · press s to save", providerLabel(p.p, p.model))
	case 1:
		cur := 0
		for i, lp := range llmProviders {
			if lp.p == m.cfg.LLM.Provider {
				cur = i
				break
			}
		}
		p := llmProviders[(cur+dir+len(llmProviders))%len(llmProviders)]
		m.cfg.LLM.Provider = p.p
		m.cfg.LLM.Model = p.model
		m.cfg.LLM.APIKeyEnv = p.env
		m.dirty = true
		m.statusMsg = fmt.Sprintf("curator -> %s · press s to save", curatorLabel(p.p, p.model))
	default:
		i := m.settingIdx - 2
		if i < 0 || i >= len(harnessRows) {
			return
		}
		hID := harnessRows[i].id
		res, err := install.Install(hID, "", "", false)
		if err != nil {
			m.statusMsg = "connect " + hID + " error: " + err.Error()
		} else {
			m.statusMsg = fmt.Sprintf("connected %s (%s)", res.Harness, res.ConfigPath)
		}
	}
}

func providerLabel(p, model string) string {
	if p == "none" || p == "" {
		return "off (BM25 keyword only)"
	}
	if model == "" {
		return p
	}
	return p + " · " + model
}

func curatorLabel(p, model string) string {
	if p == "none" || p == "" {
		return "agent-native (no extra key)"
	}
	if model == "" {
		return p
	}
	return p + " · " + model
}

func envBadge(varName string) string {
	if strings.TrimSpace(varName) == "" {
		return mutedStyle.Render("no key needed")
	}
	if strings.TrimSpace(os.Getenv(varName)) != "" {
		return addStyle.Render("● " + varName + " set")
	}
	return warnStyle.Render("● " + varName + " missing")
}

// settingsLayout returns the content row (relative to the box content start) of each setting.
func settingsLayout() []int {
	rows := make([]int, settingsRowCount)
	line := 2 // section title + blank
	for i := 0; i < settingsRowCount; i++ {
		if i == 2 {
			line += 2 // blank + "Harnesses" title
		}
		rows[i] = line
		line += 2 // label line + detail line
	}
	return rows
}

// settingsOffset keeps the selected setting visible on short terminals.
func (m Model) settingsOffset() int {
	layout := settingsLayout()
	total := layout[len(layout)-1] + 2
	rows := m.bodyHeight() - 2
	need := layout[m.settingIdx] + 2 - rows
	return clamp(need, 0, max(0, total-rows))
}

func (m Model) settingsHit(y int) (int, bool) {
	// box top border at row 1, content starts at row 2
	rel := y - 2 + m.settingsOffset()
	for i, r := range settingsLayout() {
		if rel == r || rel == r+1 {
			return i, true
		}
	}
	return 0, false
}

// ---------------------------------------------------------------------------
// Rendering helpers
// ---------------------------------------------------------------------------

func repeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(s, n)
}

func padRight(s string, w int) string {
	sw := ansi.StringWidth(s)
	if sw >= w {
		return s
	}
	return s + strings.Repeat(" ", w-sw)
}

func fit(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if ansi.StringWidth(s) > w {
		s = ansi.Truncate(s, w, "…")
	}
	return padRight(s, w)
}

// box renders a rounded panel of exactly w columns and h rows with titles in the top border.
func box(title, right string, lines []string, w, h int, focused bool) string {
	if w < 6 {
		w = 6
	}
	if h < 2 {
		h = 2
	}
	bs := borderStyle
	if focused {
		bs = focusBorder
	}
	inner := w - 2
	avail := inner - 2
	t := ""
	if title != "" {
		t = " " + title + " "
	}
	r := ""
	if right != "" {
		r = " " + right + " "
	}
	if ansi.StringWidth(t)+ansi.StringWidth(r) > avail {
		r = ""
	}
	if ansi.StringWidth(t) > avail {
		t = ansi.Truncate(t, avail, "… ")
	}
	fill := inner - 1 - ansi.StringWidth(t) - ansi.StringWidth(r)
	if r != "" {
		fill--
	}
	top := bs.Render("╭─") + t + bs.Render(repeat("─", fill))
	if r != "" {
		top += r + bs.Render("─")
	}
	top += bs.Render("╮")

	var sb strings.Builder
	sb.WriteString(top)
	cw := w - 4
	for i := 0; i < h-2; i++ {
		line := ""
		if i < len(lines) {
			line = lines[i]
		}
		sb.WriteString("\n")
		sb.WriteString(bs.Render("│") + " " + fit(line, cw) + " " + bs.Render("│"))
	}
	sb.WriteString("\n" + bs.Render("╰"+repeat("─", w-2)+"╯"))
	return sb.String()
}

func sideBySide(a, b string) string {
	al := strings.Split(a, "\n")
	bl := strings.Split(b, "\n")
	n := max(len(al), len(bl))
	aw := 0
	for _, l := range al {
		aw = max(aw, ansi.StringWidth(l))
	}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		l, r := "", ""
		if i < len(al) {
			l = al[i]
		}
		if i < len(bl) {
			r = bl[i]
		}
		out[i] = padRight(l, aw) + " " + r
	}
	return strings.Join(out, "\n")
}

func (m Model) vaultColor(name string) lipgloss.Color {
	for i, av := range m.activeVaults {
		if av.Name == name {
			return vaultPalette[i%len(vaultPalette)]
		}
	}
	return muted
}

func shortID(id string) string {
	r := []rune(id)
	if len(r) > 7 {
		return string(r[:7])
	}
	return id
}

func scrollLabel(start, rows, total int) string {
	if total <= 0 {
		return ""
	}
	end := min(total, start+rows)
	if total <= rows {
		return fmt.Sprintf("%d lines", total)
	}
	pct := 100
	if total > rows {
		pct = start * 100 / (total - rows)
	}
	return fmt.Sprintf("%d–%d/%d · %d%%", start+1, end, total, pct)
}

func wrapPlain(s string, width int) []string {
	if width < 4 {
		width = 4
	}
	if s == "" {
		return []string{""}
	}
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}

func styleInline(s string) string {
	return reInlineCode.ReplaceAllStringFunc(s, func(tok string) string {
		return mdInline.Render(tok[1 : len(tok)-1])
	})
}

// renderMarkdown turns a note into wrapped, lightly styled terminal lines.
func renderMarkdown(raw string, width int) []string {
	if width < 8 {
		width = 8
	}
	src := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	var out []string
	i := 0
	if len(src) > 0 && strings.TrimSpace(src[0]) == "---" {
		j := 1
		for ; j < len(src); j++ {
			if strings.TrimSpace(src[j]) == "---" {
				break
			}
		}
		if j < len(src) {
			for _, fl := range src[1:j] {
				k, v, ok := strings.Cut(fl, ":")
				if ok {
					out = append(out, fit(mdFrontK.Render(strings.TrimSpace(k))+mdFront.Render(" "+strings.TrimSpace(v)), width))
				} else {
					out = append(out, fit(mdFront.Render(fl), width))
				}
			}
			out = append(out, faintStyle.Render(repeat("╌", min(width, 24))))
			i = j + 1
		}
	}

	inCode := false
	for ; i < len(src); i++ {
		line := src[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			lang := strings.TrimPrefix(trimmed, "```")
			if inCode {
				label := "code"
				if lang != "" {
					label = lang
				}
				out = append(out, mdGutter.Render("┌ ")+faintStyle.Render(label))
			} else {
				out = append(out, mdGutter.Render("└"))
			}
			continue
		}
		if inCode {
			for _, part := range strings.Split(ansi.Hardwrap(strings.ReplaceAll(line, "\t", "  "), width-2, true), "\n") {
				out = append(out, mdGutter.Render("│ ")+mdCode.Render(part))
			}
			continue
		}
		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "# "):
			for _, p := range wrapPlain(strings.TrimPrefix(trimmed, "# "), width) {
				out = append(out, mdH1.Render(p))
			}
		case strings.HasPrefix(trimmed, "## "):
			for _, p := range wrapPlain(strings.TrimPrefix(trimmed, "## "), width) {
				out = append(out, mdH2.Render(p))
			}
		case strings.HasPrefix(trimmed, "### "), strings.HasPrefix(trimmed, "#### "):
			for _, p := range wrapPlain(strings.TrimLeft(trimmed, "# "), width) {
				out = append(out, mdH3.Render(p))
			}
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			indent := (len(line) - len(strings.TrimLeft(line, " "))) / 2 * 2
			body := trimmed[2:]
			parts := wrapPlain(body, width-2-indent)
			for pi, p := range parts {
				prefix := repeat(" ", indent) + "  "
				if pi == 0 {
					prefix = repeat(" ", indent) + selStyle.Render("•") + " "
				}
				out = append(out, prefix+textStyle.Render(styleInline(p)))
			}
		case strings.HasPrefix(trimmed, "> "):
			for _, p := range wrapPlain(strings.TrimPrefix(trimmed, "> "), width-2) {
				out = append(out, mdQuote.Render("▎ "+p))
			}
		default:
			for _, p := range wrapPlain(trimmed, width) {
				out = append(out, textStyle.Render(styleInline(p)))
			}
		}
	}
	// trim trailing blank lines
	for len(out) > 0 && strings.TrimSpace(ansi.Strip(out[len(out)-1])) == "" {
		out = out[:len(out)-1]
	}
	return out
}

// ---------------------------------------------------------------------------
// Scroll extents
// ---------------------------------------------------------------------------

func (m Model) previewRows() int {
	g := m.searchGeom()
	return g.detailH - 2
}

func (m Model) previewLines() []string {
	g := m.searchGeom()
	if m.previewText == "" {
		return nil
	}
	return renderMarkdown(m.previewText, g.detailW-4)
}

func (m Model) previewMaxScroll() int {
	return max(0, len(m.previewLines())-m.previewRows())
}

const diffHeaderRows = 3

func (m Model) diffRows() int {
	g := m.proposalGeom()
	return g.detailH - 2 - diffHeaderRows
}

func (m Model) diffLines() []string {
	if len(m.proposals) == 0 || m.selectedProp >= len(m.proposals) {
		return nil
	}
	g := m.proposalGeom()
	width := g.detailW - 4
	item := m.proposals[m.selectedProp]
	var out []string
	for _, dl := range strings.Split(proposal.FormatDiff(item.VaultRoot, item.Proposal), "\n") {
		st := textStyle
		switch {
		case strings.HasPrefix(dl, "+"):
			st = addStyle
		case strings.HasPrefix(dl, "-"):
			st = delStyle
		case strings.HasPrefix(dl, "@@"):
			st = hunkStyle
		}
		for _, part := range strings.Split(ansi.Hardwrap(dl, max(4, width), true), "\n") {
			out = append(out, st.Render(part))
		}
	}
	for len(out) > 0 && strings.TrimSpace(ansi.Strip(out[len(out)-1])) == "" {
		out = out[:len(out)-1]
	}
	return out
}

func (m Model) diffMaxScroll() int {
	return max(0, len(m.diffLines())-m.diffRows())
}

func (m Model) vaultLines() []string {
	w, _ := m.dims()
	width := w - 4
	var lines []string
	lines = append(lines, titleStyle.Render("Active vaults"), "")
	if len(m.activeVaults) == 0 {
		lines = append(lines, mutedStyle.Render("No vaults in scope · memtwig init ~/notes --name personal"))
	}
	for _, av := range m.activeVaults {
		col := m.vaultColor(av.Name)
		kind := av.Config.Kind
		if kind == "" {
			kind = "personal"
		}
		write := av.Config.DefaultWrite
		if write == "" {
			write = "direct"
		}
		access := "full access"
		if !av.FullAccess {
			access = "only " + strings.Join(av.PathGlobs, ", ")
		}
		notes := "?"
		if n, ok := m.vaultNotes[av.Name]; ok && n >= 0 {
			notes = fmt.Sprintf("%d notes", n)
		}
		head := lipgloss.NewStyle().Foreground(col).Render("●") + " " + titleStyle.Render(av.Name) +
			mutedStyle.Render("  "+kind+" · "+write+" · "+notes)
		lines = append(lines, head)
		kv := func(k, v string) {
			if v == "" {
				return
			}
			lines = append(lines, "  "+faintStyle.Render(fmt.Sprintf("%-7s", k))+" "+textStyle.Render(ansi.Truncate(v, max(4, width-10), "…")))
		}
		kv("path", av.Config.Path)
		kv("scope", access)
		kv("why", av.MatchReason)
		if len(av.Config.MatchPaths) > 0 {
			kv("cwd", strings.Join(av.Config.MatchPaths, ", "))
		}
		if len(av.Config.MatchGitRemotes) > 0 {
			kv("remote", strings.Join(av.Config.MatchGitRemotes, ", "))
		}
		lines = append(lines, "")
	}

	lines = append(lines, titleStyle.Render("Recent audit events"), "")
	if len(m.auditEvents) == 0 {
		lines = append(lines, mutedStyle.Render("No audit events yet"))
	}
	for i := len(m.auditEvents) - 1; i >= 0; i-- {
		ev := m.auditEvents[i]
		oc := addStyle
		switch ev.Outcome {
		case "denied":
			oc = warnStyle
		case "error":
			oc = delStyle
		}
		ts := ev.Timestamp
		if len(ts) >= 16 {
			ts = strings.Replace(ts[:16], "T", " ", 1)
		}
		where := ev.Vault
		if ev.Path != "" {
			where += ":" + ev.Path
		}
		line := faintStyle.Render(ts) + "  " + oc.Render("●") + " " + textStyle.Render(fmt.Sprintf("%-14s", ev.Action)) +
			" " + mutedStyle.Render(ev.Actor) + "  " + textStyle.Render(where)
		lines = append(lines, line)
	}
	return lines
}

func (m Model) vaultMaxScroll() int {
	return max(0, len(m.vaultLines())-(m.bodyHeight()-2))
}

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

type tabSpan struct {
	label  string
	x0, x1 int
}

func (m Model) tabLabels(short bool) []string {
	if short {
		return []string{"1 Search", fmt.Sprintf("2 Props (%d)", len(m.proposals)), "3 Vaults", "4 Setup"}
	}
	return []string{
		"1 Search",
		fmt.Sprintf("2 Proposals (%d)", len(m.proposals)),
		fmt.Sprintf("3 Vaults (%d)", len(m.activeVaults)),
		"4 Settings",
	}
}

func (m Model) brand() string {
	return brandStyle.Render("memtwig") + " " + versionStyle.Render(m.version)
}

func (m Model) tabSpans() []tabSpan {
	w, _ := m.dims()
	labels := m.tabLabels(false)
	tabsW := 0
	for _, l := range labels {
		tabsW += ansi.StringWidth(l) + 2 + 1
	}
	brandW := ansi.StringWidth("memtwig "+m.version) + 3
	x := 0
	if brandW+tabsW <= w {
		x = brandW
	} else if tabsW > w {
		labels = m.tabLabels(true)
	}
	spans := make([]tabSpan, len(labels))
	for i, l := range labels {
		lw := ansi.StringWidth(l) + 2
		spans[i] = tabSpan{label: l, x0: x, x1: x + lw}
		x += lw + 1
	}
	return spans
}

func (m Model) viewTabBar() string {
	spans := m.tabSpans()
	var sb strings.Builder
	if spans[0].x0 > 0 {
		sb.WriteString(padRight(m.brand(), spans[0].x0))
	}
	for i, s := range spans {
		st := tabInactive
		if Tab(i) == m.tab {
			st = tabActive
		}
		sb.WriteString(st.Render(" " + s.label + " "))
		if i < len(spans)-1 {
			sb.WriteString(" ")
		}
	}
	w, _ := m.dims()
	line := sb.String()
	if m.dirty && ansi.StringWidth(line)+12 <= w {
		line = padRight(line, w-10) + warnStyle.Render("● unsaved")
	}
	return line
}

func (m Model) View() string {
	w, _ := m.dims()
	bodyH := m.bodyHeight()

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

	status := ""
	switch {
	case m.pending != nil:
		label := "Approve"
		st := addStyle
		if m.pending.kind == "reject" {
			label = "Reject"
			st = delStyle
		}
		target := ""
		for _, it := range m.proposals {
			if it.Proposal.ID == m.pending.id {
				target = it.VaultName + ":" + it.Proposal.Path
			}
		}
		status = st.Bold(true).Render(label+" "+shortID(m.pending.id)) + mutedStyle.Render(" → ") + textStyle.Render(target) +
			mutedStyle.Render("   ") + keyStyle.Render("y") + mutedStyle.Render(" confirm · ") + keyStyle.Render("any key") + mutedStyle.Render(" cancel")
	case m.statusMsg != "":
		status = statusStyle.Render("▸ " + m.statusMsg)
	}

	var keys string
	switch {
	case m.showHelp:
		keys = "esc close help · j/k scroll"
	case m.typingQuery:
		keys = "enter search · ctrl+u clear · ctrl+w word · esc stop typing"
	case m.tab == TabSearch:
		keys = "/ search · j/k select · J/K scroll · esc clear · i reindex · ? help · q quit"
	case m.tab == TabProposals:
		keys = "j/k select · J/K scroll diff · a approve · x reject · c curate · ? help · q quit"
	case m.tab == TabVaults:
		keys = "j/k scroll · J/K page · i reindex · c curate · ? help · q quit"
	case m.tab == TabSettings:
		keys = "j/k select · enter/←/→ change · s save · ? help · q quit"
	}

	return strings.Join([]string{
		fit(m.viewTabBar(), w),
		body,
		fit(status, w),
		fit(mutedStyle.Render(keys), w),
	}, "\n")
}

type helpSection struct {
	title string
	rows  [][2]string
}

var helpSections = []helpSection{
	{"Navigation", [][2]string{
		{"1 2 3 4", "Search · Proposals · Vaults & Audit · Settings"},
		{"tab / shift+tab", "Next / previous tab"},
		{"j k  ↑ ↓", "Move selection (scroll on Vaults)"},
		{"g G  home end", "First / last item"},
		{"J K  pgdn pgup", "Scroll preview, diff or vault list"},
		{"mouse", "Wheel scrolls the pane under the pointer · click tabs and rows"},
	}},
	{"Search", [][2]string{
		{"/", "Type a query (click the search box too)"},
		{"enter", "Run search"},
		{"ctrl+u  ctrl+w", "Clear query · delete last word"},
		{"esc", "Stop typing · reset results to all notes"},
		{"i", "Incremental reindex"},
	}},
	{"Proposals", [][2]string{
		{"a", "Approve selected, then y to confirm"},
		{"x / r", "Reject selected, then y to confirm"},
		{"c", "Run curator pass"},
	}},
	{"Settings", [][2]string{
		{"enter  ← →", "Cycle provider · connect harness"},
		{"s", "Save config.toml and reindex"},
	}},
	{"General", [][2]string{
		{"?", "Toggle this help"},
		{"q  ctrl+c", "Quit (q twice with unsaved settings)"},
	}},
}

const helpKeyW = 17

func helpColumn(sections []helpSection, width int) []string {
	var out []string
	descW := max(8, width-2-helpKeyW)
	for si, sec := range sections {
		if si > 0 {
			out = append(out, "")
		}
		out = append(out, selStyle.Render(sec.title))
		for _, r := range sec.rows {
			for i, part := range wrapPlain(r[1], descW) {
				k := ""
				if i == 0 {
					k = r[0]
				}
				out = append(out, "  "+keyStyle.Render(fmt.Sprintf("%-*s", helpKeyW, k))+textStyle.Render(part))
			}
		}
	}
	return out
}

func (m Model) helpLines() []string {
	w, _ := m.dims()
	inner := w - 4
	if inner >= 96 {
		colW := (inner - 4) / 2
		left := helpColumn(helpSections[:2], colW)
		right := helpColumn(helpSections[2:], colW)
		n := max(len(left), len(right))
		out := make([]string, n)
		for i := 0; i < n; i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			out[i] = fit(l, colW) + "    " + r
		}
		return out
	}
	return helpColumn(helpSections, inner)
}

func (m Model) helpMaxScroll() int {
	return max(0, len(m.helpLines())-(m.bodyHeight()-2))
}

func (m Model) viewHelp(w, h int) string {
	lines := m.helpLines()
	rows := h - 2
	start := clamp(m.helpScroll, 0, max(0, len(lines)-rows))
	end := min(len(lines), start+rows)
	right := ""
	if len(lines) > rows {
		right = mutedStyle.Render(scrollLabel(start, rows, len(lines)))
	}
	return box(titleStyle.Render("Keyboard & mouse"), right, lines[start:end], w, h, true)
}

func (m Model) viewSearch(w, h int) string {
	g := m.searchGeom()

	prompt := mutedStyle.Render("/ ")
	q := m.query
	if m.typingQuery {
		prompt = selStyle.Render("/ ")
		q = textStyle.Render(q) + cursorStyle.Render(" ")
	} else if q == "" {
		q = faintStyle.Render("type / to search notes")
	} else {
		q = textStyle.Render(q)
	}
	qBox := box(titleStyle.Render("Search"), "", []string{prompt + q}, w, 3, m.typingQuery)

	// list
	rows := g.listH - 2
	var leftLines []string
	listTitle := "Notes"
	if strings.TrimSpace(m.query) != "" {
		listTitle = "Results"
	}
	right := ""
	if len(m.results) == 0 {
		leftLines = append(leftLines, mutedStyle.Render("No notes in scope"))
	} else {
		right = mutedStyle.Render(fmt.Sprintf("%d/%d", m.selectedIdx+1, len(m.results)))
		start := listWindow(m.selectedIdx, len(m.results), rows)
		for i := start; i < len(m.results) && i < start+rows; i++ {
			r := m.results[i]
			dot := lipgloss.NewStyle().Foreground(m.vaultColor(r.Vault)).Render("●")
			if i == m.selectedIdx {
				leftLines = append(leftLines, selStyle.Render("›")+" "+dot+" "+selStyle.Render(r.Path))
			} else {
				leftLines = append(leftLines, "  "+dot+" "+textStyle.Render(r.Path))
			}
		}
	}
	listBox := box(titleStyle.Render(listTitle), right, leftLines, g.listW, g.listH, !m.typingQuery)

	// preview
	prows := g.detailH - 2
	pl := m.previewLines()
	start := clamp(m.previewScroll, 0, max(0, len(pl)-prows))
	end := min(len(pl), start+prows)
	ptitle := "Preview"
	if len(m.results) > 0 && m.selectedIdx < len(m.results) {
		sel := m.results[m.selectedIdx]
		ptitle = lipgloss.NewStyle().Foreground(m.vaultColor(sel.Vault)).Render(sel.Vault) + mutedStyle.Render(" / ") + titleStyle.Render(sel.Path)
	}
	var visible []string
	if len(pl) == 0 {
		visible = []string{mutedStyle.Render("No note selected")}
	} else {
		visible = pl[start:end]
	}
	prevBox := box(ptitle, mutedStyle.Render(scrollLabel(start, prows, len(pl))), visible, g.detailW, g.detailH, false)

	if g.stacked {
		return qBox + "\n" + listBox + "\n" + prevBox
	}
	return qBox + "\n" + sideBySide(listBox, prevBox)
}

func (m Model) viewProposals(w, h int) string {
	g := m.proposalGeom()
	rows := g.listH - 2

	var leftLines []string
	right := ""
	if len(m.proposals) == 0 {
		leftLines = append(leftLines, mutedStyle.Render("Queue empty · c runs the curator"))
	} else {
		right = mutedStyle.Render(fmt.Sprintf("%d/%d", m.selectedProp+1, len(m.proposals)))
		start := listWindow(m.selectedProp, len(m.proposals), rows)
		for i := start; i < len(m.proposals) && i < start+rows; i++ {
			item := m.proposals[i]
			dot := lipgloss.NewStyle().Foreground(m.vaultColor(item.VaultName)).Render("●")
			id := faintStyle.Render(fmt.Sprintf("%-7s", shortID(item.Proposal.ID)))
			if i == m.selectedProp {
				leftLines = append(leftLines, selStyle.Render("›")+" "+id+" "+dot+" "+selStyle.Render(item.Proposal.Path))
			} else {
				leftLines = append(leftLines, "  "+id+" "+dot+" "+textStyle.Render(item.Proposal.Path))
			}
		}
	}
	listBox := box(titleStyle.Render("Queue"), right, leftLines, g.listW, g.listH, true)

	var detail []string
	scrollInfo := ""
	if len(m.proposals) > 0 && m.selectedProp < len(m.proposals) {
		item := m.proposals[m.selectedProp]
		p := item.Proposal
		head := titleStyle.Render(p.ID) + mutedStyle.Render("  "+p.Kind+" · ") +
			lipgloss.NewStyle().Foreground(m.vaultColor(item.VaultName)).Render(item.VaultName) + mutedStyle.Render(":") + textStyle.Render(p.Path)
		meta := mutedStyle.Render(fmt.Sprintf("by %s · %s", p.Author, p.CreatedAt))
		if p.Reason != "" {
			meta += mutedStyle.Render(" · ") + textStyle.Render(p.Reason)
		}
		detail = append(detail, head, meta, faintStyle.Render(repeat("╌", max(4, g.detailW-4))))
		dl := m.diffLines()
		drows := m.diffRows()
		start := clamp(m.diffScroll, 0, max(0, len(dl)-drows))
		end := min(len(dl), start+drows)
		detail = append(detail, dl[start:end]...)
		scrollInfo = mutedStyle.Render(scrollLabel(start, drows, len(dl)))
	} else {
		detail = append(detail, mutedStyle.Render("No proposal selected"))
	}
	diffBox := box(titleStyle.Render("Diff"), scrollInfo, detail, g.detailW, g.detailH, false)

	if g.stacked {
		return listBox + "\n" + diffBox
	}
	return sideBySide(listBox, diffBox)
}

func (m Model) viewVaults(w, h int) string {
	lines := m.vaultLines()
	rows := h - 2
	start := clamp(m.vaultScroll, 0, max(0, len(lines)-rows))
	end := min(len(lines), start+rows)
	return box(titleStyle.Render("Vaults & Audit"), mutedStyle.Render(scrollLabel(start, rows, len(lines))), lines[start:end], w, h, true)
}

func (m Model) viewSettings(w, h int) string {
	home, _ := os.UserHomeDir()
	type row struct{ label, value, detail string }
	rows := []row{
		{"Embeddings", providerLabel(m.cfg.Embeddings.Provider, m.cfg.Embeddings.Model),
			fmt.Sprintf("%dd · ", m.cfg.Embeddings.Dimensions) + envBadge(m.cfg.Embeddings.APIKeyEnv)},
		{"Curator", curatorLabel(m.cfg.LLM.Provider, m.cfg.LLM.Model), envBadge(m.cfg.LLM.APIKeyEnv)},
	}
	for _, hr := range harnessRows {
		state := mutedStyle.Render("not detected")
		if home != "" {
			if _, err := os.Stat(filepath.Join(home, hr.rel)); err == nil {
				state = addStyle.Render("detected") + mutedStyle.Render(" · ~/"+hr.rel)
			}
		}
		rows = append(rows, row{hr.label, "", state + mutedStyle.Render(" · enter connects")})
	}

	layout := settingsLayout()
	var lines []string
	lines = append(lines, titleStyle.Render("Providers"), "")
	for i, r := range rows {
		for len(lines) < layout[i] {
			lines = append(lines, "")
		}
		if i == 2 {
			lines[len(lines)-1] = titleStyle.Render("Harnesses")
		}
		marker := "  "
		label := textStyle.Render(fmt.Sprintf("%-12s", r.label))
		if i == m.settingIdx {
			marker = selStyle.Render("› ")
			label = selStyle.Render(fmt.Sprintf("%-12s", r.label))
		}
		value := ""
		if r.value != "" {
			value = textStyle.Render(r.value)
			if i == m.settingIdx {
				value = mutedStyle.Render("‹ ") + titleStyle.Render(r.value) + mutedStyle.Render(" ›")
			}
		}
		lines = append(lines, marker+label+value, "    "+r.detail)
	}
	right := mutedStyle.Render("saved")
	if m.dirty {
		right = warnStyle.Render("● unsaved · s to save")
	}
	off := m.settingsOffset()
	return box(titleStyle.Render("Settings"), right, lines[min(off, len(lines)):], w, h, true)
}
