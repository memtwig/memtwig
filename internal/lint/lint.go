package lint

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vittoriocompagno/memtwig/internal/vault"
)

type Issue struct {
	Severity string `json:"severity"` // error | warning
	Vault    string `json:"vault"`
	Path     string `json:"path"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type Report struct {
	VaultName string  `json:"vault"`
	Checked   int     `json:"checked"`
	Errors    int     `json:"errors"`
	Warnings  int     `json:"warnings"`
	Issues    []Issue `json:"issues"`
}

var (
	secretPatterns = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"openai_key", regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{20,}\b`)},
		{"anthropic_key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`)},
		{"github_token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{25,}\b`)},
		{"github_pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{25,}\b`)},
		{"aws_access_key", regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)},
		{"slack_token", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`)},
		{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	}

	reIndexLink = regexp.MustCompile(`\]\(([^)]+\.md)\)`)
)

// CheckVault runs deterministic hygiene and secret checks on a vault.
func CheckVault(vaultName, root string, maxNoteBytes int) (Report, error) {
	if maxNoteBytes <= 0 {
		maxNoteBytes = 8192
	}
	rep := Report{VaultName: vaultName}
	notes, err := vault.WalkNotes(vaultName, root)
	if err != nil {
		return rep, err
	}
	today := time.Now().Format("2006-01-02")

	add := func(sev, rel, code, msg string) {
		rep.Issues = append(rep.Issues, Issue{
			Severity: sev,
			Vault:    vaultName,
			Path:     rel,
			Code:     code,
			Message:  msg,
		})
		if sev == "error" {
			rep.Errors++
		} else {
			rep.Warnings++
		}
	}

	for _, n := range notes {
		rep.Checked++
		if !utf8.ValidString(n.Raw) {
			add("error", n.RelPath, "invalid_utf8", "file is not valid UTF-8")
		}
		// Secret scan
		for _, sp := range secretPatterns {
			if sp.re.MatchString(n.Raw) {
				add("error", n.RelPath, "secret_detected", fmt.Sprintf("potential secret matched (%s)", sp.name))
			}
		}
		// Future date check
		if n.Updated != "" && n.Updated > today {
			add("error", n.RelPath, "future_date", fmt.Sprintf("updated date %s is in the future", n.Updated))
		}
		// Size warning (soft cap)
		if int(n.SizeBytes) > maxNoteBytes {
			add("warning", n.RelPath, "oversize_note", fmt.Sprintf("size %d bytes exceeds %d bytes cap (consider archiving)", n.SizeBytes, maxNoteBytes))
		}
		// Missing metadata warning
		if n.Updated == "" && !strings.HasPrefix(n.RelPath, "decisions/") && !strings.HasPrefix(n.RelPath, "decisioni/") {
			add("warning", n.RelPath, "missing_updated", "missing updated/Aggiornato date in header")
		}
	}

	// Check broken links in index.md if present
	indexPath := filepath.Join(root, "index.md")
	if data, err := os.ReadFile(indexPath); err == nil {
		matches := reIndexLink.FindAllStringSubmatch(string(data), -1)
		seen := map[string]int{}
		for _, m := range matches {
			target := m[1]
			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
				continue
			}
			seen[target]++
			full := filepath.Join(root, filepath.FromSlash(target))
			if _, err := os.Stat(full); err != nil {
				add("error", "index.md", "broken_link", fmt.Sprintf("broken link to %s", target))
			}
		}
		for target, count := range seen {
			if count > 1 {
				add("warning", "index.md", "duplicate_link", fmt.Sprintf("duplicate link to %s (%dx)", target, count))
			}
		}
	}

	return rep, nil
}
