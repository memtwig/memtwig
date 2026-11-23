package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/audit"
	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/index"
	"github.com/vittoriocompagno/memtwig/internal/server"
)

func TestServerRBACMCPAndAudit(t *testing.T) {
	tmp := t.TempDir()
	acmeDir := filepath.Join(tmp, "acme")
	betaDir := filepath.Join(tmp, "beta")
	for _, d := range []string{filepath.Join(acmeDir, "projects"), filepath.Join(betaDir, "projects")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(acmeDir, "projects", "api.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Acme API\nAcme uses Postgres 17.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(betaDir, "projects", "secret.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Beta Secret\nBeta uses Postgres 17 confidential.\n"), 0o644)

	cfg := config.Default()
	cfg.Vaults["acme"] = config.VaultConfig{Path: acmeDir, Kind: "org", DefaultWrite: "propose"}
	cfg.Vaults["beta"] = config.VaultConfig{Path: betaDir, Kind: "org", DefaultWrite: "propose"}
	cfg.Server.Tokens = []config.TokenPolicy{
		{Name: "acme-bot", Token: "tok-acme", Role: "proposer", Vaults: []string{"acme"}},
		{Name: "admin-user", Token: "tok-admin", Role: "admin", Vaults: []string{"*"}},
	}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}

	aud := audit.New(filepath.Join(tmp, "audit.jsonl"))
	srv := &server.Server{Cfg: cfg, Store: st, Audit: aud, Version: "1.0.0-test"}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Unauthenticated must get 401
	resp, err := http.Get(ts.URL + "/api/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// 2. acme-bot searches "Postgres" -> must see acme only, NEVER beta
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/search?q=Postgres", nil)
	req.Header.Set("Authorization", "Bearer tok-acme")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var searchBody struct {
		Results []struct {
			Vault string `json:"vault"`
			Path  string `json:"path"`
		} `json:"results"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&searchBody)
	resp.Body.Close()
	if len(searchBody.Results) != 1 || searchBody.Results[0].Vault != "acme" {
		t.Fatalf("expected only acme result, got %+v", searchBody.Results)
	}

	// 3. acme-bot cannot read /api/v1/audit (admin only) -> 403
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/audit", nil)
	req.Header.Set("Authorization", "Bearer tok-acme")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for proposer on audit, got %d", resp.StatusCode)
	}

	// 4. acme-bot submits a proposal, admin approves it
	propPayload := `{"vault":"acme","path":"projects/api.md","kind":"append","content":"- [2026-11-23 · bot] Added Redis cache","reason":"observed"}`
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/proposals", strings.NewReader(propPayload))
	req.Header.Set("Authorization", "Bearer tok-acme")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var propResp struct {
		Proposal struct {
			ID string `json:"id"`
		} `json:"proposal"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&propResp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || propResp.Proposal.ID == "" {
		t.Fatalf("expected 201 proposal created, got %d %+v", resp.StatusCode, propResp)
	}

	// Approve via admin
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/proposals/acme/"+propResp.Proposal.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on approve, got %d", resp.StatusCode)
	}

	// 5. Streamable HTTP MCP initialize works with bearer token
	mcpReq := bytes.NewBufferString(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/mcp", mcpReq)
	req.Header.Set("Authorization", "Bearer tok-acme")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /mcp, got %d", resp.StatusCode)
	}

	// 6. Admin reads audit log -> events recorded
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/audit?n=20", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var audResp struct {
		Events []audit.Event `json:"events"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&audResp)
	resp.Body.Close()
	if len(audResp.Events) < 4 {
		t.Fatalf("expected >=4 audit events, got %d", len(audResp.Events))
	}
}
