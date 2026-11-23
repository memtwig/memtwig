package vault

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Note struct {
	VaultName string
	RelPath   string // slash-separated path relative to vault root
	AbsPath   string
	Title     string
	Status    string // active | parked | closed (also maps attivo/parcheggiato/chiuso)
	ValidFrom string
	Updated   string
	Owner     string
	Tags      []string
	Body      string
	Raw       string
	SHA256    string
	SizeBytes int64
}

type frontmatter struct {
	Status    string   `yaml:"status"`
	Stato     string   `yaml:"stato"`
	ValidFrom string   `yaml:"valid_from"`
	Updated   string   `yaml:"updated"`
	Owner     string   `yaml:"owner"`
	Tags      []string `yaml:"tags"`
}

var (
	reHeading   = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reStatus    = regexp.MustCompile(`(?i)^(?:status|stato)\s*:\s*(\S+)`)
	reValidFrom = regexp.MustCompile(`(?i)^(?:valid_from|valido\s+dal)\s*:\s*(\d{4}-\d{2}-\d{2})`)
	reUpdated   = regexp.MustCompile(`(?i)^(?:updated|aggiornat[oa])\s*:\s*≤?(\d{4}-\d{2}-\d{2})`)
	reOwner     = regexp.MustCompile(`(?i)^(?:owner|proprietario)\s*:\s*(\S+)`)
)

// InitOptions customizes vault scaffolding.
type InitOptions struct {
	NoScaffold bool
	GitInit    bool
}

// Init scaffolds a new memtwig vault directory with default options.
func Init(root string) error {
	return InitWithOptions(root, InitOptions{})
}

// InitWithOptions scaffolds a new memtwig vault directory.
func InitWithOptions(root string, opts InitOptions) error {
	dirs := []string{
		"projects",
		"decisions",
		"persona",
		"infra",
		".curator",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			return err
		}
	}
	today := time.Now().Format("2006-01-02")

	vaultToml := filepath.Join(root, "memtwig.toml")
	if _, err := os.Stat(vaultToml); os.IsNotExist(err) {
		content := "version = 1\nmax_note_bytes = 8192\n"
		if err := os.WriteFile(vaultToml, []byte(content), 0o644); err != nil {
			return err
		}
	}

	gitignore := filepath.Join(root, ".gitignore")
	if _, err := os.Stat(gitignore); os.IsNotExist(err) {
		content := "private/\n*.db\n*.db-wal\n*.db-shm\n"
		if err := os.WriteFile(gitignore, []byte(content), 0o644); err != nil {
			return err
		}
	}

	if !opts.NoScaffold {
		welcome := filepath.Join(root, "projects", "getting-started.md")
		if _, err := os.Stat(welcome); os.IsNotExist(err) {
			content := fmt.Sprintf(`---
status: active
valid_from: %s
updated: %s
---

# Getting Started

- [%s · memtwig] Vault initialized. Keep facts in projects/ and infra/, decisions in decisions/, and user preferences in persona/.
`, today, today, today)
			if err := os.WriteFile(welcome, []byte(content), 0o644); err != nil {
				return err
			}
		}
	}

	if opts.GitInit {
		if _, err := os.Stat(filepath.Join(root, ".git")); os.IsNotExist(err) {
			if gitBin, err := exec.LookPath("git"); err == nil {
				_ = exec.Command(gitBin, "-C", root, "init", "-q").Run()
			}
		}
	}

	_, err := GenerateIndex(root)
	return err
}

// WalkNotes reads all live Markdown notes in a vault, skipping .git, .curator, private, archive/archivio, and index.md.
func WalkNotes(vaultName, root string) ([]Note, error) {
	var notes []Note
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if name == ".git" || name == ".curator" || name == "private" || name == "archive" || name == "archivio" || name == "bin" || name == "node_modules" {
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
		if rel == "index.md" || rel == "README.md" {
			return nil
		}
		note, err := ParseNote(vaultName, root, rel)
		if err != nil {
			return nil
		}
		notes = append(notes, note)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(notes, func(i, j int) bool { return notes[i].RelPath < notes[j].RelPath })
	return notes, nil
}

// ParseNote parses a single Markdown note supporting both YAML frontmatter and lightweight top-of-file headers.
func ParseNote(vaultName, root, relPath string) (Note, error) {
	abs := filepath.Join(root, filepath.FromSlash(relPath))
	rawBytes, err := os.ReadFile(abs)
	if err != nil {
		return Note{}, err
	}
	sum := sha256.Sum256(rawBytes)
	raw := string(rawBytes)

	n := Note{
		VaultName: vaultName,
		RelPath:   filepath.ToSlash(relPath),
		AbsPath:   abs,
		Raw:       raw,
		Body:      raw,
		SHA256:    hex.EncodeToString(sum[:]),
		SizeBytes: int64(len(rawBytes)),
	}

	// 1. YAML frontmatter if present
	if strings.HasPrefix(raw, "---\n") || strings.HasPrefix(raw, "---\r\n") {
		rest := raw[4:]
		if idx := strings.Index(rest, "\n---"); idx >= 0 {
			fmRaw := rest[:idx]
			var fm frontmatter
			if err := yaml.Unmarshal([]byte(fmRaw), &fm); err == nil {
				n.Status = normalizeStatus(firstNonEmpty(fm.Status, fm.Stato))
				n.ValidFrom = fm.ValidFrom
				n.Updated = fm.Updated
				n.Owner = fm.Owner
				n.Tags = fm.Tags
			}
			after := rest[idx+4:]
			n.Body = strings.TrimLeft(after, "\r\n")
		}
	}

	// 2. Fallback lightweight headers in first 12 lines (compatible with ~/notes)
	lines := strings.Split(n.Body, "\n")
	limit := 12
	if len(lines) < limit {
		limit = len(lines)
	}
	for _, line := range lines[:limit] {
		trimmed := strings.TrimSpace(line)
		if n.Status == "" {
			if m := reStatus.FindStringSubmatch(trimmed); len(m) == 2 {
				n.Status = normalizeStatus(m[1])
			}
		}
		if n.ValidFrom == "" {
			if m := reValidFrom.FindStringSubmatch(trimmed); len(m) == 2 {
				n.ValidFrom = m[1]
			}
		}
		if n.Updated == "" {
			if m := reUpdated.FindStringSubmatch(trimmed); len(m) == 2 {
				n.Updated = m[1]
			}
		}
		if n.Owner == "" {
			if m := reOwner.FindStringSubmatch(trimmed); len(m) == 2 {
				n.Owner = m[1]
			}
		}
	}

	if m := reHeading.FindStringSubmatch(n.Body); len(m) == 2 {
		n.Title = strings.TrimSpace(m[1])
	} else {
		n.Title = strings.TrimSuffix(filepath.Base(relPath), filepath.Ext(relPath))
	}

	return n, nil
}

// GenerateIndex writes an autogenerated index.md listing all live notes grouped by top-level directory.
func GenerateIndex(root string) (int, error) {
	notes, err := WalkNotes("", root)
	if err != nil {
		return 0, err
	}
	groups := map[string][]Note{}
	for _, n := range notes {
		dir := filepath.Dir(n.RelPath)
		if dir == "." {
			dir = "root"
		} else {
			dir = strings.Split(dir, "/")[0]
		}
		groups[dir] = append(groups[dir], n)
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	sb.WriteString("# Vault Index\n\n")
	sb.WriteString(fmt.Sprintf("Updated: %s\n\n", time.Now().Format("2006-01-02")))
	sb.WriteString("<!-- Generated by memtwig. Do not edit manually. -->\n\n")

	for _, k := range keys {
		sb.WriteString("## " + k + "\n")
		for _, n := range groups[k] {
			meta := ""
			if n.Status != "" {
				meta = " (" + n.Status + ")"
			}
			sb.WriteString(fmt.Sprintf("- [%s](%s)%s\n", n.Title, n.RelPath, meta))
		}
		sb.WriteString("\n")
	}

	return len(notes), os.WriteFile(filepath.Join(root, "index.md"), []byte(sb.String()), 0o644)
}

func normalizeStatus(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active", "attivo":
		return "active"
	case "parked", "parcheggiato":
		return "parked"
	case "closed", "chiuso":
		return "closed"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
