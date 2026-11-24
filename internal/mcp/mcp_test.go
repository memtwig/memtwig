package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/mcp"
	"github.com/memtwig/memtwig/internal/proposal"
)

func TestMCPGovernanceAndProposals(t *testing.T) {
	tmp := t.TempDir()
	vaultDir := filepath.Join(tmp, "vault")
	if err := os.MkdirAll(filepath.Join(vaultDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	lockedNote := "---\nstatus: active\nowner: vittorio\nupdated: 2026-11-23\n---\n# Core\nLocked fact.\n"
	if err := os.WriteFile(filepath.Join(vaultDir, "projects", "core.md"), []byte(lockedNote), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{
		Path:         vaultDir,
		Kind:         "personal",
		DefaultWrite: "direct",
	}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}

	srv := &mcp.Server{Cfg: cfg, Store: st, Cwd: tmp, Version: "0.1.0-test"}

	reqs := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_write","arguments":{"vault":"personal","path":"projects/core.md","content":"overwrite"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_propose","arguments":{"vault":"personal","path":"projects/core.md","kind":"append","content":"- [2026-11-23 · agent] New fact","reason":"observed in repo"}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(reqs), &out); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 responses, got %d: %s", len(lines), out.String())
	}

	// Response 1: initialize must include prescriptive instructions
	var resp1 struct {
		Result struct {
			Instructions string `json:"instructions"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &resp1); err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"memory_search", "memory_propose", "owner", "secrets"} {
		if !strings.Contains(strings.ToLower(resp1.Result.Instructions), needle) {
			t.Fatalf("initialize instructions missing %q: %s", needle, resp1.Result.Instructions)
		}
	}

	// Response 2: memory_write must be blocked because owner: vittorio
	var resp2 struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &resp2); err != nil {
		t.Fatal(err)
	}
	if !resp2.Result.IsError || !strings.Contains(resp2.Result.Content[0].Text, "owner-locked") {
		t.Fatalf("expected owner-locked error, got %+v", resp2)
	}

	// Proposal must exist, and once rejected, re-proposing the same content must not reopen it
	props, err := proposal.List(vaultDir, false)
	if err != nil || len(props) != 1 {
		t.Fatalf("expected 1 open proposal, got %d (err=%v)", len(props), err)
	}
	if _, err := proposal.Reject(vaultDir, props[0].ID, "not needed"); err != nil {
		t.Fatal(err)
	}
	_, created, err := proposal.Add(vaultDir, props[0])
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("rejected proposal must not be reopened")
	}
}
