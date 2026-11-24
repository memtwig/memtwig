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

func TestMCPMemoryDeleteGovernanceAndIndex(t *testing.T) {
	tmp := t.TempDir()
	personalDir := filepath.Join(tmp, "personal")
	orgDir := filepath.Join(tmp, "org")
	for _, d := range []string{
		filepath.Join(personalDir, "projects"),
		filepath.Join(personalDir, "decisions"),
		filepath.Join(orgDir, "projects"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(personalDir, "projects", "temp.md"), []byte("# Temp\nDisposable note alpha777.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(personalDir, "projects", "locked.md"), []byte("owner: vittorio\n# Locked\nDo not delete.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(personalDir, "decisions", "2026-11.md"), []byte("# Decisions\n- 2026-11-24 Keep.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(orgDir, "projects", "shared.md"), []byte("# Shared\nOrg note.\n"), 0o644)

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{
		Path:          personalDir,
		Kind:          "personal",
		DefaultWrite:  "direct",
		AlwaysInclude: []string{"projects/**", "decisions/**"},
	}
	cfg.Vaults["acme"] = config.VaultConfig{
		Path:         orgDir,
		Kind:         "org",
		DefaultWrite: "propose",
		MatchPaths:   []string{tmp + "/**"},
	}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}

	srv := &mcp.Server{Cfg: cfg, Store: st, Cwd: filepath.Join(tmp, "work"), Version: "1.5.2-test"}

	reqs := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_delete","arguments":{"vault":"personal","path":"projects/temp.md"}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"memory_delete","arguments":{"vault":"personal","path":"projects/locked.md"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"memory_delete","arguments":{"vault":"personal","path":"decisions/2026-11.md"}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"memory_delete","arguments":{"vault":"acme","path":"projects/shared.md"}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"memory_propose","arguments":{"vault":"acme","path":"projects/shared.md","kind":"delete","reason":"no longer relevant"}}}`,
	}, "\n") + "\n"

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(reqs), &out); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 responses, got %d: %s", len(lines), out.String())
	}

	type toolResp struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}

	// 1. tools/list includes memory_delete
	if !strings.Contains(lines[0], `"memory_delete"`) {
		t.Fatalf("tools/list missing memory_delete: %s", lines[0])
	}

	// 2. Direct delete succeeds and removes file + index row
	var r2 toolResp
	_ = json.Unmarshal([]byte(lines[1]), &r2)
	if r2.Result.IsError || !strings.Contains(r2.Result.Content[0].Text, "deleted personal:projects/temp.md") {
		t.Fatalf("expected direct delete success, got %+v", r2)
	}
	if _, err := os.Stat(filepath.Join(personalDir, "projects", "temp.md")); !os.IsNotExist(err) {
		t.Fatalf("temp.md should be removed from disk")
	}
	var cnt int
	_ = st.DB.QueryRow(`SELECT COUNT(*) FROM documents WHERE vault_id = 'personal' AND rel_path = 'projects/temp.md'`).Scan(&cnt)
	if cnt != 0 {
		t.Fatalf("temp.md should be removed from SQLite index")
	}

	// 3. Owner-locked delete blocked
	var r3 toolResp
	_ = json.Unmarshal([]byte(lines[2]), &r3)
	if !r3.Result.IsError || !strings.Contains(r3.Result.Content[0].Text, "owner-locked") {
		t.Fatalf("expected owner-locked error, got %+v", r3)
	}

	// 4. Decision log delete blocked
	var r4 toolResp
	_ = json.Unmarshal([]byte(lines[3]), &r4)
	if !r4.Result.IsError || !strings.Contains(r4.Result.Content[0].Text, "decision log") {
		t.Fatalf("expected decision log error, got %+v", r4)
	}

	// 5. Propose vault direct delete blocked
	var r5 toolResp
	_ = json.Unmarshal([]byte(lines[4]), &r5)
	if !r5.Result.IsError || !strings.Contains(r5.Result.Content[0].Text, "memory_propose") {
		t.Fatalf("expected propose-vault error, got %+v", r5)
	}

	// 6. Proposing kind=delete without content succeeds
	var r6 toolResp
	_ = json.Unmarshal([]byte(lines[5]), &r6)
	if r6.Result.IsError || !strings.Contains(r6.Result.Content[0].Text, "queued for review") {
		t.Fatalf("expected delete proposal queued, got %+v", r6)
	}
	props, err := proposal.List(orgDir, false)
	if err != nil || len(props) != 1 || props[0].Kind != "delete" {
		t.Fatalf("expected 1 delete proposal in org vault, got %+v (err=%v)", props, err)
	}
}
