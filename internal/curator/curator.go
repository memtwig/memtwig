package curator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/memtwig/memtwig/internal/proposal"
	"github.com/memtwig/memtwig/internal/vault"
)

type CurateReport struct {
	VaultName         string `json:"vault"`
	IndexNotes        int    `json:"index_notes"`
	ProposalsCreated  int    `json:"proposals_created"`
	OversizeNotes     int    `json:"oversize_notes"`
	StaleNotes        int    `json:"stale_notes"`
	UnsyncedDecisions int    `json:"unsynced_decisions"`
}

var reDecisionTarget = regexp.MustCompile(`^-\s+(\d{4}-\d{2}-\d{2})\s+(.+?)\s+->\s+(\S+\.md)\s*$`)

// Decide appends a dated decision at the top of decisions/YYYY-MM.md and optionally notes/proposes it on targetRel.
func Decide(vaultRoot, project, summary, targetRel string, allowDirectWrite bool) (string, error) {
	today := time.Now().Format("2006-01-02")
	month := today[:7]
	decDir := filepath.Join(vaultRoot, "decisions")
	if err := os.MkdirAll(decDir, 0o755); err != nil {
		return "", err
	}
	decFile := filepath.Join(decDir, month+".md")

	entry := fmt.Sprintf("- %s %s: %s", today, strings.TrimSpace(project), strings.TrimSpace(summary))
	if targetRel != "" {
		entry += " -> " + filepath.ToSlash(strings.TrimPrefix(targetRel, "/"))
	}

	existing, err := os.ReadFile(decFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", err
		}
		header := fmt.Sprintf("# Decisions %s\n\nUpdated: %s\n\n%s\n", month, today, entry)
		if err := os.WriteFile(decFile, []byte(header), 0o644); err != nil {
			return "", err
		}
	} else {
		text := string(existing)
		if strings.Contains(text, entry) {
			return "duplicate decision ignored", nil
		}
		// Insert right after header / first blank line so newest stays at top
		lines := strings.Split(text, "\n")
		insertAt := len(lines)
		for i, l := range lines {
			if strings.HasPrefix(l, "- ") {
				insertAt = i
				break
			}
		}
		var out []string
		out = append(out, lines[:insertAt]...)
		out = append(out, entry)
		out = append(out, lines[insertAt:]...)
		if err := os.WriteFile(decFile, []byte(strings.Join(out, "\n")), 0o644); err != nil {
			return "", err
		}
	}

	if targetRel == "" {
		return fmt.Sprintf("decision logged in decisions/%s.md", month), nil
	}

	targetRel = filepath.ToSlash(strings.TrimPrefix(targetRel, "/"))
	noteLine := fmt.Sprintf("- [%s · decision] %s", today, strings.TrimSpace(summary))
	parsed, parseErr := vault.ParseNote("", vaultRoot, targetRel)
	if allowDirectWrite && (parseErr != nil || parsed.Owner == "") {
		abs := filepath.Join(vaultRoot, filepath.FromSlash(targetRel))
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", err
		}
		f, err := os.OpenFile(abs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return "", err
		}
		_, werr := f.WriteString("\n" + noteLine + "\n")
		f.Close()
		if werr != nil {
			return "", werr
		}
		return fmt.Sprintf("decision logged in decisions/%s.md and appended to %s", month, targetRel), nil
	}

	p, _, err := proposal.Add(vaultRoot, proposal.Proposal{
		Kind:    "append",
		Path:    targetRel,
		Content: noteLine,
		Reason:  "decision recorded in decisions/" + month + ".md",
		Author:  "memtwig-decide",
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("decision logged in decisions/%s.md; proposal %s queued for %s", month, p.ID, targetRel), nil
}

// Invalidate moves lines matching needle from a live note into <dir>/archive/<file> (append-only)
// and leaves a dated pointer in the live note.
func Invalidate(vaultRoot, relPath, needle, reason string) (int, string, error) {
	relPath = filepath.ToSlash(strings.TrimPrefix(relPath, "/"))
	abs := filepath.Join(vaultRoot, filepath.FromSlash(relPath))
	raw, err := os.ReadFile(abs)
	if err != nil {
		return 0, "", err
	}
	lines := strings.Split(string(raw), "\n")
	var kept, moved []string
	lowerNeedle := strings.ToLower(strings.TrimSpace(needle))
	for _, l := range lines {
		if lowerNeedle != "" && strings.Contains(strings.ToLower(l), lowerNeedle) {
			moved = append(moved, l)
		} else {
			kept = append(kept, l)
		}
	}
	if len(moved) == 0 {
		return 0, "", fmt.Errorf("no lines matched %q in %s", needle, relPath)
	}

	today := time.Now().Format("2006-01-02")
	dir := filepath.Dir(relPath)
	base := filepath.Base(relPath)
	archiveRel := filepath.ToSlash(filepath.Join(dir, "archive", base))
	if dir == "." {
		archiveRel = "archive/" + base
	}
	archiveAbs := filepath.Join(vaultRoot, filepath.FromSlash(archiveRel))
	if err := os.MkdirAll(filepath.Dir(archiveAbs), 0o755); err != nil {
		return 0, "", err
	}

	var archBlock strings.Builder
	archBlock.WriteString(fmt.Sprintf("\n## Archived %s (%s)\n", today, reason))
	for _, m := range moved {
		archBlock.WriteString(m + "\n")
	}
	f, err := os.OpenFile(archiveAbs, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return 0, "", err
	}
	_, werr := f.WriteString(archBlock.String())
	f.Close()
	if werr != nil {
		return 0, "", werr
	}

	pointer := fmt.Sprintf("- [%s · invalidated] %d line(s) moved to `%s` (%s).", today, len(moved), archiveRel, reason)
	body := strings.TrimRight(strings.Join(kept, "\n"), "\n")
	if body != "" {
		body += "\n"
	}
	body += pointer + "\n"
	if err := os.WriteFile(abs, []byte(body), 0o644); err != nil {
		return 0, "", err
	}
	return len(moved), archiveRel, nil
}

// Curate runs a deterministic hygiene pass over a vault:
// regenerates index.md and creates proposals for oversize notes, stale active notes, and unsynced decisions.
func Curate(vaultName, vaultRoot string, maxNoteBytes int, staleDays int) (CurateReport, error) {
	if maxNoteBytes <= 0 {
		maxNoteBytes = 8192
	}
	if staleDays <= 0 {
		staleDays = 90
	}
	rep := CurateReport{VaultName: vaultName}

	notes, err := vault.WalkNotes(vaultName, vaultRoot)
	if err != nil {
		return rep, err
	}
	noteByPath := map[string]vault.Note{}
	cutoff := time.Now().AddDate(0, 0, -staleDays).Format("2006-01-02")

	for _, n := range notes {
		noteByPath[n.RelPath] = n
		if int(n.SizeBytes) > maxNoteBytes && !vault.IsDecisionLog(n.RelPath) {
			rep.OversizeNotes++
			_, created, _ := proposal.Add(vaultRoot, proposal.Proposal{
				Kind:    "archive",
				Path:    n.RelPath,
				Content: fmt.Sprintf("Archive historical entries from %s (%d bytes > %d bytes cap).", n.RelPath, n.SizeBytes, maxNoteBytes),
				Reason:  "note exceeds size cap",
				Author:  "memtwig-curator",
			})
			if created {
				rep.ProposalsCreated++
			}
		}
		if n.Status == "active" && n.Updated != "" && n.Updated < cutoff {
			rep.StaleNotes++
			_, created, _ := proposal.Add(vaultRoot, proposal.Proposal{
				Kind:    "review",
				Path:    n.RelPath,
				Content: fmt.Sprintf("Review active note %s (last updated %s, >%d days ago): update or park.", n.RelPath, n.Updated, staleDays),
				Reason:  "stale active note",
				Author:  "memtwig-curator",
			})
			if created {
				rep.ProposalsCreated++
			}
		}
	}

	// Check decisions/YYYY-MM.md for targets whose updated date is older than the decision date
	for _, decFolder := range []string{"decisions", "decisioni"} {
		entries, err := os.ReadDir(filepath.Join(vaultRoot, decFolder))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(vaultRoot, decFolder, e.Name()))
			if err != nil {
				continue
			}
			for _, line := range strings.Split(string(raw), "\n") {
				m := reDecisionTarget.FindStringSubmatch(strings.TrimSpace(line))
				if len(m) != 4 {
					continue
				}
				decDate, decText, target := m[1], m[2], m[3]
				if targetNote, ok := noteByPath[target]; ok {
					if targetNote.Updated != "" && targetNote.Updated < decDate {
						rep.UnsyncedDecisions++
						_, created, _ := proposal.Add(vaultRoot, proposal.Proposal{
							Kind:    "append",
							Path:    target,
							Content: fmt.Sprintf("- [%s · decision] %s", decDate, decText),
							Reason:  fmt.Sprintf("decision from %s/%s newer than note updated date (%s)", decFolder, e.Name(), targetNote.Updated),
							Author:  "memtwig-curator",
						})
						if created {
							rep.ProposalsCreated++
						}
					}
				}
			}
		}
	}

	if _, err := os.Stat(filepath.Join(vaultRoot, "memtwig.toml")); err == nil {
		n, err := vault.GenerateIndex(vaultRoot)
		if err == nil {
			rep.IndexNotes = n
		}
	} else {
		rep.IndexNotes = len(notes)
	}

	return rep, nil
}
