package proposal

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Proposal struct {
	ID        string `json:"id"`
	Kind      string `json:"kind"`      // update | create | append | delete | archive
	Path      string `json:"path"`      // relative path inside vault
	Content   string `json:"content"`   // proposed content or section
	Reason    string `json:"reason"`    // why the agent proposed this
	Author    string `json:"author"`    // agent / harness / user
	Status    string `json:"status"`    // open | approved | rejected
	CreatedAt string `json:"created_at"`
	ClosedAt  string `json:"closed_at,omitempty"`
	Outcome   string `json:"outcome,omitempty"`
}

func storePath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".curator", "proposals.json")
}

func historyPath(vaultRoot string) string {
	return filepath.Join(vaultRoot, ".curator", "history.jsonl")
}

// DeterministicID computes a stable short SHA1 id so rejected proposals are never reopened.
func DeterministicID(kind, relPath, content string) string {
	norm := strings.Join(strings.Fields(strings.ToLower(content)), " ")
	h := sha1.Sum([]byte(kind + "|" + filepath.ToSlash(relPath) + "|" + norm))
	return hex.EncodeToString(h[:])[:12]
}

func LoadAll(vaultRoot string) (map[string]Proposal, error) {
	p := storePath(vaultRoot)
	data, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]Proposal{}, nil
		}
		return nil, err
	}
	out := map[string]Proposal{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("parse %s: %w", p, err)
	}
	return out, nil
}

func saveAll(vaultRoot string, m map[string]Proposal) error {
	p := storePath(vaultRoot)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(data, '\n'), 0o644)
}

func closedIDs(vaultRoot string) map[string]bool {
	out := map[string]bool{}
	f, err := os.Open(historyPath(vaultRoot))
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var p Proposal
		if err := json.Unmarshal(sc.Bytes(), &p); err == nil && p.ID != "" {
			out[p.ID] = true
		}
	}
	return out
}

func appendHistory(vaultRoot string, p Proposal) error {
	hp := historyPath(vaultRoot)
	if err := os.MkdirAll(filepath.Dir(hp), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(hp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, err := json.Marshal(p)
	if err != nil {
		return err
	}
	_, err = f.Write(append(line, '\n'))
	return err
}

// Add records a new proposal unless an identical one is already open or was previously rejected in history.
func Add(vaultRoot string, p Proposal) (Proposal, bool, error) {
	if p.Kind == "" {
		p.Kind = "append"
	}
	p.Path = filepath.ToSlash(strings.TrimPrefix(p.Path, "/"))
	p.ID = DeterministicID(p.Kind, p.Path, p.Content)
	if p.CreatedAt == "" {
		p.CreatedAt = time.Now().Format("2006-01-02")
	}
	p.Status = "open"

	if closedIDs(vaultRoot)[p.ID] {
		return p, false, nil
	}
	all, err := LoadAll(vaultRoot)
	if err != nil {
		return p, false, err
	}
	if prev, ok := all[p.ID]; ok {
		return prev, false, nil
	}
	all[p.ID] = p
	if err := saveAll(vaultRoot, all); err != nil {
		return p, false, err
	}
	return p, true, nil
}

func List(vaultRoot string, includeClosed bool) ([]Proposal, error) {
	all, err := LoadAll(vaultRoot)
	if err != nil {
		return nil, err
	}
	var out []Proposal
	for _, p := range all {
		if !includeClosed && p.Status != "open" {
			continue
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// Approve applies the proposal to the target file inside vaultRoot and archives it.
func Approve(vaultRoot, id string) (Proposal, error) {
	all, err := LoadAll(vaultRoot)
	if err != nil {
		return Proposal{}, err
	}
	p, ok := all[id]
	if !ok {
		return Proposal{}, fmt.Errorf("proposal %s not found", id)
	}
	if p.Status != "open" {
		return p, fmt.Errorf("proposal %s is already %s", id, p.Status)
	}

	target := filepath.Join(vaultRoot, filepath.FromSlash(p.Path))
	switch p.Kind {
	case "create", "update":
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return p, err
		}
		if err := os.WriteFile(target, []byte(p.Content), 0o644); err != nil {
			return p, err
		}
	case "append":
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return p, err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return p, err
		}
		_, werr := f.WriteString("\n" + strings.TrimSpace(p.Content) + "\n")
		f.Close()
		if werr != nil {
			return p, werr
		}
	case "delete":
		if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
			return p, err
		}
	default:
		return p, fmt.Errorf("unsupported proposal kind %q for automatic apply", p.Kind)
	}

	p.Status = "approved"
	p.ClosedAt = time.Now().Format("2006-01-02")
	p.Outcome = "applied"
	all[id] = p
	if err := saveAll(vaultRoot, all); err != nil {
		return p, err
	}
	_ = appendHistory(vaultRoot, p)
	return p, nil
}

// Get returns a single proposal by ID.
func Get(vaultRoot, id string) (Proposal, bool, error) {
	all, err := LoadAll(vaultRoot)
	if err != nil {
		return Proposal{}, false, err
	}
	p, ok := all[id]
	return p, ok, nil
}

// FormatDiff produces a readable unified-style diff preview against the current live file on disk.
func FormatDiff(vaultRoot string, p Proposal) string {
	target := filepath.Join(vaultRoot, filepath.FromSlash(p.Path))
	existingBytes, err := os.ReadFile(target)
	var sb strings.Builder
	if p.Kind == "delete" {
		sb.WriteString(fmt.Sprintf("--- a/%s\n+++ /dev/null\n", p.Path))
		if err != nil {
			sb.WriteString("@@ file already missing @@\n")
			return sb.String()
		}
		existing := strings.Split(strings.TrimRight(string(existingBytes), "\n"), "\n")
		sb.WriteString(fmt.Sprintf("@@ -1,%d +0,0 @@\n", len(existing)))
		for _, l := range existing {
			sb.WriteString("- " + l + "\n")
		}
		return sb.String()
	}
	sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", p.Path, p.Path))
	if err != nil {
		sb.WriteString("@@ new file @@\n")
		for _, l := range strings.Split(strings.TrimRight(p.Content, "\n"), "\n") {
			sb.WriteString("+ " + l + "\n")
		}
		return sb.String()
	}
	existing := strings.Split(strings.TrimRight(string(existingBytes), "\n"), "\n")
	proposed := strings.Split(strings.TrimRight(p.Content, "\n"), "\n")
	switch p.Kind {
	case "append":
		start := len(existing) - 3
		if start < 0 {
			start = 0
		}
		sb.WriteString(fmt.Sprintf("@@ append after line %d @@\n", len(existing)))
		for _, l := range existing[start:] {
			sb.WriteString("  " + l + "\n")
		}
		for _, l := range proposed {
			sb.WriteString("+ " + l + "\n")
		}
	default:
		sb.WriteString(fmt.Sprintf("@@ -1,%d +1,%d @@\n", len(existing), len(proposed)))
		for _, l := range existing {
			sb.WriteString("- " + l + "\n")
		}
		for _, l := range proposed {
			sb.WriteString("+ " + l + "\n")
		}
	}
	return sb.String()
}

// Reject marks a proposal rejected and writes it to append-only history so it never reopens.
func Reject(vaultRoot, id, reason string) (Proposal, error) {
	all, err := LoadAll(vaultRoot)
	if err != nil {
		return Proposal{}, err
	}
	p, ok := all[id]
	if !ok {
		return Proposal{}, fmt.Errorf("proposal %s not found", id)
	}
	p.Status = "rejected"
	p.ClosedAt = time.Now().Format("2006-01-02")
	p.Outcome = reason
	all[id] = p
	if err := saveAll(vaultRoot, all); err != nil {
		return p, err
	}
	_ = appendHistory(vaultRoot, p)
	return p, nil
}
