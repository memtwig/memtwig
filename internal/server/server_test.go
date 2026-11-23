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
	"github.com/vittoriocompagno/memtwig/internal/ui"
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

func TestServerUIAndNoteEndpoints(t *testing.T) {
	tmp := t.TempDir()
	acmeDir := filepath.Join(tmp, "acme")
	betaDir := filepath.Join(tmp, "beta")
	for _, d := range []string{filepath.Join(acmeDir, "projects"), filepath.Join(betaDir, "projects")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	_ = os.WriteFile(filepath.Join(acmeDir, "projects", "api.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Acme API\nUses Postgres 17.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(betaDir, "projects", "secret.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Beta Secret\nConfidential.\n"), 0o644)

	cfg := config.Default()
	cfg.Vaults["acme"] = config.VaultConfig{Path: acmeDir, Kind: "org", DefaultWrite: "propose"}
	cfg.Vaults["beta"] = config.VaultConfig{Path: betaDir, Kind: "org", DefaultWrite: "propose"}
	cfg.Server.Tokens = []config.TokenPolicy{
		{Name: "acme-reader", Token: "tok-read", Role: "reader", Vaults: []string{"acme"}},
		{Name: "admin", Token: "tok-admin", Role: "admin", Vaults: []string{"*"}},
	}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}

	uiFS := ui.FS()
	cfgFile := filepath.Join(tmp, "config.toml")
	srv := &server.Server{Cfg: cfg, ConfigPath: cfgFile, Store: st, Audit: audit.New(filepath.Join(tmp, "audit.jsonl")), StaticUI: uiFS, Version: "1.4.0-test"}
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Embedded UI serves index.html at / and SPA fallback at /proposals
	for _, p := range []string{"/", "/proposals"} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(buf.String(), "<div id=\"root\">") {
			t.Fatalf("expected embedded SPA HTML for %s, got %d", p, resp.StatusCode)
		}
	}

	// 2. Unknown /api/* route returns 404 JSON, never HTML
	resp, err := http.Get(ts.URL + "/api/v1/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown /api route, got %d", resp.StatusCode)
	}

	// 3. GET /api/v1/notes respects vault RBAC
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/notes", nil)
	req.Header.Set("Authorization", "Bearer tok-read")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var notesResp struct {
		Notes []struct {
			Vault string `json:"vault"`
			Path  string `json:"path"`
		} `json:"notes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&notesResp)
	resp.Body.Close()
	if len(notesResp.Notes) != 1 || notesResp.Notes[0].Vault != "acme" {
		t.Fatalf("expected only acme note, got %+v", notesResp.Notes)
	}

	// 4. GET /api/v1/note returns raw note for allowed vault, denies forbidden vault & traversal
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/note?vault=acme&path=projects/api.md", nil)
	req.Header.Set("Authorization", "Bearer tok-read")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var noteBody struct {
		Raw string `json:"raw"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&noteBody)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(noteBody.Raw, "Postgres 17") {
		t.Fatalf("expected raw note, got %d %q", resp.StatusCode, noteBody.Raw)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/note?vault=beta&path=projects/secret.md", nil)
	req.Header.Set("Authorization", "Bearer tok-read")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for beta vault, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/note?vault=acme&path=../beta/projects/secret.md", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for path traversal, got %d", resp.StatusCode)
	}

	// 5. Proposal includes diff field
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/proposals", strings.NewReader(`{"vault":"acme","path":"projects/api.md","kind":"append","content":"- Added Redis","reason":"check diff"}`))
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/proposals", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	var propsResp struct {
		Proposals []struct {
			Diff string `json:"diff"`
		} `json:"proposals"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&propsResp)
	resp.Body.Close()
	if len(propsResp.Proposals) != 1 || !strings.Contains(propsResp.Proposals[0].Diff, "+ - Added Redis") && !strings.Contains(propsResp.Proposals[0].Diff, "+- Added Redis") {
		t.Fatalf("expected non-empty diff with added line, got %+v", propsResp.Proposals)
	}

	// 6. PUT /api/v1/note enforces default_write=direct (rejects propose vault with 409)
	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/v1/note", strings.NewReader(`{"vault":"acme","path":"projects/api.md","content":"# overwrite"}`))
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on PUT to propose vault, got %d", resp.StatusCode)
	}

	// 7. GET /api/v1/graph returns scoped nodes and edges (never leaks beta to acme-reader)
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/graph", nil)
	req.Header.Set("Authorization", "Bearer tok-read")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var graphResp struct {
		Nodes []struct {
			ID    string `json:"id"`
			Vault string `json:"vault"`
		} `json:"nodes"`
		Edges []struct {
			Source string `json:"source"`
			Target string `json:"target"`
		} `json:"edges"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&graphResp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || len(graphResp.Nodes) != 2 || len(graphResp.Edges) != 1 {
		t.Fatalf("expected 2 nodes and 1 contains edge for acme-reader, got %d nodes=%+v edges=%+v", resp.StatusCode, graphResp.Nodes, graphResp.Edges)
	}
	for _, n := range graphResp.Nodes {
		if n.Vault == "beta" {
			t.Fatalf("graph isolation breach: saw beta node %+v", n)
		}
	}

	// 8. POST /api/v1/reindex & /api/v1/curate enforce writer role
	for _, ep := range []string{"/api/v1/reindex", "/api/v1/curate"} {
		req, _ = http.NewRequest(http.MethodPost, ts.URL+ep, nil)
		req.Header.Set("Authorization", "Bearer tok-read")
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403 for reader on %s, got %d", ep, resp.StatusCode)
		}

		req, _ = http.NewRequest(http.MethodPost, ts.URL+ep, nil)
		req.Header.Set("Authorization", "Bearer tok-admin")
		resp, _ = http.DefaultClient.Do(req)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 for admin on %s, got %d", ep, resp.StatusCode)
		}
	}

	// 9. GET/PUT /api/v1/config and POST/DELETE /api/v1/vaults enforce admin role and persist config.toml
	req, _ = http.NewRequest(http.MethodGet, ts.URL+"/api/v1/config", nil)
	req.Header.Set("Authorization", "Bearer tok-read")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for reader on /api/v1/config, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodPut, ts.URL+"/api/v1/config", strings.NewReader(`{"embeddings":{"provider":"deepinfra","model":"BAAI/bge-m3","api_key_env":"DEEPINFRA_API_KEY","dimensions":1024},"llm":{"provider":"groq","model":"llama-3.3-70b-versatile","api_key_env":"GROQ_API_KEY"}}`))
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on PUT /api/v1/config, got %d", resp.StatusCode)
	}
	reloaded, err := config.Load(cfgFile)
	if err != nil || reloaded.Embeddings.Provider != "deepinfra" || reloaded.LLM.Provider != "groq" {
		t.Fatalf("expected persisted deepinfra+groq config, got err=%v emb=%+v llm=%+v", err, reloaded.Embeddings, reloaded.LLM)
	}

	gammaDir := filepath.Join(tmp, "gamma")
	req, _ = http.NewRequest(http.MethodPost, ts.URL+"/api/v1/vaults", strings.NewReader(`{"name":"gamma","path":"`+gammaDir+`","kind":"org","default_write":"propose","init_if_missing":true}`))
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on POST /api/v1/vaults, got %d", resp.StatusCode)
	}
	if _, err := os.Stat(filepath.Join(gammaDir, "memtwig.toml")); err != nil {
		t.Fatalf("expected gamma vault initialized on disk: %v", err)
	}

	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/vaults/gamma", nil)
	req.Header.Set("Authorization", "Bearer tok-admin")
	resp, _ = http.DefaultClient.Do(req)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on DELETE /api/v1/vaults/gamma, got %d", resp.StatusCode)
	}
}
