package lint

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/vault"
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

type CheckOptions struct {
	MaxNoteBytes int
	ScanEnvFiles []string
}

var (
	secretPatterns = []struct {
		name string
		re   *regexp.Regexp
	}{
		{"openai_key", regexp.MustCompile(`\bsk-(?:ant-|proj-)?[A-Za-z0-9_-]{20,}\b`)},
		{"anthropic_key", regexp.MustCompile(`\bsk-ant-[A-Za-z0-9_-]{20,}\b`)},
		{"github_token", regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9_]{20,}\b`)},
		{"github_pat", regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{25,}\b`)},
		{"aws_access_key", regexp.MustCompile(`\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
		{"slack_token", regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`)},
		{"stripe_key", regexp.MustCompile(`\b(?:sk|rk|pk)_(?:live|test)_[A-Za-z0-9]{10,}\b`)},
		{"google_key", regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{35}\b`)},
		{"jwt", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)},
		{"bearer_token", regexp.MustCompile(`(?i)\bbearer\s+[A-Za-z0-9_\-\.=]{24,}`)},
		{"secret_assignment", regexp.MustCompile(`(?i)\b(?:password|passwd|secret|token|api[_-]?key)\b\s*[:=]\s*["']?[A-Za-z0-9_\-/+=]{12,}`)},
		{"url_with_creds", regexp.MustCompile(`https?://[^/\s:@]+:[^/\s@]+@`)},
		{"private_key", regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`)},
	}

	reConflictMarker = regexp.MustCompile(`(?m)^(?:<<<<<<< |>>>>>>> |=======\r?$)`)
	reIndexLink      = regexp.MustCompile(`\]\(([^)]+\.md)\)`)
)

// CheckVault runs deterministic hygiene and secret checks on a vault.
func CheckVault(vaultName, root string, maxNoteBytes int) (Report, error) {
	return CheckVaultWithOptions(vaultName, root, CheckOptions{MaxNoteBytes: maxNoteBytes})
}

// CheckVaultWithOptions runs hygiene, secret regex, env literal secret, and merge-conflict checks on a vault.
func CheckVaultWithOptions(vaultName, root string, opts CheckOptions) (Report, error) {
	maxNoteBytes := opts.MaxNoteBytes
	if maxNoteBytes <= 0 {
		maxNoteBytes = 8192
	}
	rep := Report{VaultName: vaultName}
	notes, err := vault.WalkNotes(vaultName, root)
	if err != nil {
		return rep, err
	}
	literals := config.ExtractSecretLiterals(opts.ScanEnvFiles)
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

	scanSecretsAndConflicts := func(rel, raw string) {
		if !utf8.ValidString(raw) {
			add("error", rel, "invalid_utf8", "file is not valid UTF-8")
		}
		for _, sp := range secretPatterns {
			if sp.re.MatchString(raw) {
				add("error", rel, "secret_detected", fmt.Sprintf("potential secret matched (%s)", sp.name))
			}
		}
		for _, lit := range literals {
			if strings.Contains(raw, lit) {
				add("error", rel, "secret_literal", "literal secret value from configured env file detected (value hidden)")
				break
			}
		}
		if reConflictMarker.MatchString(raw) {
			add("error", rel, "merge_conflict", "unresolved git merge conflict marker found")
		}
	}

	for _, n := range notes {
		rep.Checked++
		scanSecretsAndConflicts(n.RelPath, n.Raw)
		if n.Updated != "" && n.Updated > today {
			add("error", n.RelPath, "future_date", fmt.Sprintf("updated date %s is in the future", n.Updated))
		}
		if int(n.SizeBytes) > maxNoteBytes && !vault.IsDecisionLog(n.RelPath) {
			add("warning", n.RelPath, "oversize_note", fmt.Sprintf("size %d bytes exceeds %d bytes cap (consider archiving)", n.SizeBytes, maxNoteBytes))
		}
		if n.Updated == "" && !vault.IsDecisionLog(n.RelPath) {
			add("warning", n.RelPath, "missing_updated", "missing updated/Aggiornato date in header")
		}
	}

	// Scan archived Markdown files for secrets and conflict markers without emitting oversize/date warnings.
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == ".curator" || name == "private" || name == "bin" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		wrapped := "/" + rel
		if !strings.Contains(wrapped, "/archive/") && !strings.Contains(wrapped, "/archivio/") {
			return nil
		}
		if raw, err := os.ReadFile(p); err == nil {
			scanSecretsAndConflicts(rel, string(raw))
		}
		return nil
	})

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
