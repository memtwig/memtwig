package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/vittoriocompagno/memtwig/internal/audit"
	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/curator"
	"github.com/vittoriocompagno/memtwig/internal/index"
	"github.com/vittoriocompagno/memtwig/internal/mcp"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/scope"
	"github.com/vittoriocompagno/memtwig/internal/search"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

type Server struct {
	Cfg      *config.Config
	Store    *index.Store
	Audit    *audit.Logger
	StaticUI fs.FS
	Version  string
}

type principal struct {
	Name      string
	Role      string // reader | proposer | writer | admin
	Vaults    map[string]bool
	PathGlobs []string
}

func (p principal) allowsRole(required string) bool {
	rank := map[string]int{
		"reader":   1,
		"proposer": 2,
		"writer":   3,
		"admin":    4,
	}
	return rank[p.Role] >= rank[required]
}

func (p principal) allowsVault(name string) bool {
	if len(p.Vaults) == 0 || p.Vaults["*"] {
		return true
	}
	return p.Vaults[name]
}

// Handler returns the HTTP mux exposing Streamable HTTP MCP (/mcp) and REST API (/api/v1/*) with RBAC + audit.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "version": s.Version})
	})
	mux.HandleFunc("/mcp", s.withAuth("reader", s.handleMCP))
	mux.HandleFunc("/api/v1/status", s.withAuth("reader", s.handleStatus))
	mux.HandleFunc("/api/v1/notes", s.withAuth("reader", s.handleNotes))
	mux.HandleFunc("/api/v1/note", s.withAuth("reader", s.handleNote))
	mux.HandleFunc("/api/v1/search", s.withAuth("reader", s.handleSearch))
	mux.HandleFunc("/api/v1/proposals", s.withAuth("reader", s.handleProposals))
	mux.HandleFunc("/api/v1/proposals/", s.withAuth("writer", s.handleProposalAction))
	mux.HandleFunc("/api/v1/reindex", s.withAuth("writer", s.handleReindex))
	mux.HandleFunc("/api/v1/curate", s.withAuth("writer", s.handleCurate))
	mux.HandleFunc("/api/v1/audit", s.withAuth("admin", s.handleAudit))
	if s.StaticUI != nil {
		fileSrv := http.FileServer(http.FS(s.StaticUI))
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
				return
			}
			clean := strings.TrimPrefix(filepath.Clean(r.URL.Path), "/")
			if clean == "" || clean == "." {
				clean = "index.html"
			}
			if _, err := fs.Stat(s.StaticUI, clean); err == nil {
				fileSrv.ServeHTTP(w, r)
				return
			}
			// SPA fallback to index.html
			raw, err := fs.ReadFile(s.StaticUI, "index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(raw)
		})
	}
	return mux
}

func (s *Server) authenticate(r *http.Request) (principal, bool) {
	// If no tokens are configured, allow local-only admin access (developer mode).
	if len(s.Cfg.Server.Tokens) == 0 {
		return principal{Name: "local", Role: "admin", Vaults: map[string]bool{"*": true}}, true
	}
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return principal{}, false
	}
	given := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	if given == "" {
		return principal{}, false
	}
	for _, tp := range s.Cfg.Server.Tokens {
		expected := tp.Token
		if tp.TokenEnv != "" {
			expected = os.Getenv(tp.TokenEnv)
		}
		if expected == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(given), []byte(expected)) == 1 {
			vset := map[string]bool{}
			for _, v := range tp.Vaults {
				vset[v] = true
			}
			role := tp.Role
			if role == "" {
				role = "reader"
			}
			return principal{
				Name:      tp.Name,
				Role:      role,
				Vaults:    vset,
				PathGlobs: tp.PathGlobs,
			}, true
		}
	}
	return principal{}, false
}

func (s *Server) withAuth(requiredRole string, next func(http.ResponseWriter, *http.Request, principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, ok := s.authenticate(r)
		if !ok {
			_ = s.Audit.Log(audit.Event{Actor: "anonymous", Action: r.Method + " " + r.URL.Path, Outcome: "denied", Detail: "invalid or missing bearer token"})
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if !p.allowsRole(requiredRole) {
			_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: r.Method + " " + r.URL.Path, Outcome: "denied", Detail: "insufficient role"})
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden: requires role " + requiredRole})
			return
		}
		next(w, r, p)
	}
}

func (s *Server) scopedConfigFor(p principal) *config.Config {
	cloned := *s.Cfg
	cloned.Vaults = map[string]config.VaultConfig{}
	for name, vc := range s.Cfg.Vaults {
		if !p.allowsVault(name) {
			continue
		}
		// Enforce role ceiling on write permissions
		switch p.Role {
		case "reader":
			vc.DefaultWrite = "readonly"
		case "proposer":
			if vc.DefaultWrite == "direct" {
				vc.DefaultWrite = "propose"
			}
		}
		cloned.Vaults[name] = vc
	}
	return &cloned
}

func (s *Server) activeVaultsFor(p principal, vaultOverride string) []scope.ActiveVault {
	cfg := s.scopedConfigFor(p)
	var out []scope.ActiveVault
	for name, vc := range cfg.Vaults {
		if vaultOverride != "" && name != vaultOverride {
			continue
		}
		av := scope.ActiveVault{
			Name:        name,
			Config:      vc,
			FullAccess:  len(p.PathGlobs) == 0,
			PathGlobs:   p.PathGlobs,
			MatchReason: "rbac:" + p.Role,
		}
		out = append(out, av)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Server) handleMCP(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	scopedCfg := s.scopedConfigFor(p)
	mcpSrv := &mcp.Server{
		Cfg:     scopedCfg,
		Store:   s.Store,
		Cwd:     r.Header.Get("X-Memtwig-Cwd"),
		Version: s.Version,
	}
	var out bytes.Buffer
	if err := mcpSrv.ServeStdio(r.Context(), bytes.NewReader(append(bytes.TrimSpace(body), '\n')), &out); err != nil {
		_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Harness: r.Header.Get("X-Memtwig-Harness"), Action: "mcp", Outcome: "error", Detail: err.Error()})
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Harness: r.Header.Get("X-Memtwig-Harness"), Action: "mcp", Outcome: "ok"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out.Bytes())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request, p principal) {
	active := s.activeVaultsFor(p, r.URL.Query().Get("vault"))
	type vInfo struct {
		Name          string `json:"name"`
		Kind          string `json:"kind"`
		DefaultWrite  string `json:"default_write"`
		OpenProposals int    `json:"open_proposals"`
	}
	var vaults []vInfo
	for _, av := range active {
		props, _ := proposal.List(config.ExpandHome(av.Config.Path), false)
		vaults = append(vaults, vInfo{
			Name:          av.Name,
			Kind:          av.Config.Kind,
			DefaultWrite:  av.Config.DefaultWrite,
			OpenProposals: len(props),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"actor":  p.Name,
		"role":   p.Role,
		"vaults": vaults,
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request, p principal) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing q parameter"})
		return
	}
	limit := s.Cfg.Retrieval.DefaultTopK
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 50 {
			limit = v
		}
	}
	active := s.activeVaultsFor(p, r.URL.Query().Get("vault"))
	results, warn, err := search.Query(r.Context(), s.Store, s.Cfg, active, q, search.Options{TopK: limit})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "search", Vault: r.URL.Query().Get("vault"), Outcome: "ok", Detail: q})
	writeJSON(w, http.StatusOK, map[string]any{"results": results, "warning": warn})
}

func (s *Server) handleProposals(w http.ResponseWriter, r *http.Request, p principal) {
	vFilter := r.URL.Query().Get("vault")
	includeClosed := r.URL.Query().Get("all") == "1" || r.URL.Query().Get("all") == "true"
	active := s.activeVaultsFor(p, vFilter)

	switch r.Method {
	case http.MethodGet:
		type item struct {
			Vault string `json:"vault"`
			Diff  string `json:"diff"`
			proposal.Proposal
		}
		var out []item
		for _, av := range active {
			root := config.ExpandHome(av.Config.Path)
			props, _ := proposal.List(root, includeClosed)
			for _, pr := range props {
				out = append(out, item{Vault: av.Name, Diff: proposal.FormatDiff(root, pr), Proposal: pr})
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"proposals": out})

	case http.MethodPost:
		if !p.allowsRole("proposer") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "requires proposer role"})
			return
		}
		var req struct {
			Vault   string `json:"vault"`
			Path    string `json:"path"`
			Kind    string `json:"kind"`
			Content string `json:"content"`
			Reason  string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !p.allowsVault(req.Vault) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "vault not allowed"})
			return
		}
		vc, ok := s.Cfg.Vaults[req.Vault]
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "vault not found"})
			return
		}
		root := config.ExpandHome(vc.Path)
		if _, err := safeJoin(root, req.Path); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		pr, created, err := proposal.Add(root, proposal.Proposal{
			Kind:    req.Kind,
			Path:    req.Path,
			Content: req.Content,
			Reason:  req.Reason,
			Author:  p.Name,
		})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "propose", Vault: req.Vault, Path: req.Path, Outcome: "ok", Detail: pr.ID})
		writeJSON(w, http.StatusCreated, map[string]any{"proposal": pr, "created": created})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET or POST required"})
	}
}

func (s *Server) handleProposalAction(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	// Path: /api/v1/proposals/<vault>/<id>/<approve|reject>
	trimmed := strings.TrimPrefix(r.URL.Path, "/api/v1/proposals/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expected /api/v1/proposals/<vault>/<id>/<approve|reject>"})
		return
	}
	vName, id, action := parts[0], parts[1], parts[2]
	if !p.allowsVault(vName) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "vault not allowed"})
		return
	}
	vc, ok := s.Cfg.Vaults[vName]
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "vault not found"})
		return
	}
	root := config.ExpandHome(vc.Path)
	switch action {
	case "approve":
		pr, err := proposal.Approve(root, id)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_, _ = s.Store.RebuildOrUpdate(r.Context(), s.Cfg, false)
		_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "proposal_approve", Vault: vName, Path: pr.Path, Outcome: "ok", Detail: id})
		writeJSON(w, http.StatusOK, map[string]any{"proposal": pr})
	case "reject":
		var body struct {
			Reason string `json:"reason"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		pr, err := proposal.Reject(root, id, body.Reason)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "proposal_reject", Vault: vName, Path: pr.Path, Outcome: "ok", Detail: id})
		writeJSON(w, http.StatusOK, map[string]any{"proposal": pr})
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action: " + action})
	}
}

func (s *Server) handleNotes(w http.ResponseWriter, r *http.Request, p principal) {
	active := s.activeVaultsFor(p, r.URL.Query().Get("vault"))
	type noteSummary struct {
		Vault     string `json:"vault"`
		Path      string `json:"path"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		Updated   string `json:"updated"`
		Owner     string `json:"owner"`
		SizeBytes int64  `json:"size_bytes"`
	}
	var out []noteSummary
	for _, av := range active {
		notes, err := vault.WalkNotes(av.Name, config.ExpandHome(av.Config.Path))
		if err != nil {
			continue
		}
		for _, n := range notes {
			if !av.AllowsPath(n.RelPath) {
				continue
			}
			out = append(out, noteSummary{
				Vault:     av.Name,
				Path:      n.RelPath,
				Title:     n.Title,
				Status:    n.Status,
				Updated:   n.Updated,
				Owner:     n.Owner,
				SizeBytes: n.SizeBytes,
			})
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"notes": out})
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method == http.MethodPut {
		if !p.allowsRole("writer") {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "requires writer role"})
			return
		}
		var req struct {
			Vault   string `json:"vault"`
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		relPath := filepath.ToSlash(strings.TrimPrefix(req.Path, "/"))
		if req.Vault == "" || relPath == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vault and path are required"})
			return
		}
		for _, av := range s.activeVaultsFor(p, req.Vault) {
			if av.Name != req.Vault || !av.AllowsPath(relPath) {
				continue
			}
			if av.Config.DefaultWrite != "direct" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "vault requires proposals; use POST /api/v1/proposals"})
				return
			}
			root := config.ExpandHome(av.Config.Path)
			target, err := safeJoin(root, relPath)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			if existing, err := vault.ParseNote(req.Vault, root, relPath); err == nil && existing.Owner != "" {
				writeJSON(w, http.StatusConflict, map[string]string{"error": "note is owner-locked (" + existing.Owner + "); use POST /api/v1/proposals"})
				return
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if err := os.WriteFile(target, []byte(req.Content), 0o644); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			_, _ = s.Store.EnsureFresh(r.Context(), s.Cfg)
			_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "note_write", Vault: req.Vault, Path: relPath, Outcome: "ok"})
			writeJSON(w, http.StatusOK, map[string]any{"saved": true, "vault": req.Vault, "path": relPath})
			return
		}
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
		return
	}

	vName := r.URL.Query().Get("vault")
	relPath := filepath.ToSlash(strings.TrimPrefix(r.URL.Query().Get("path"), "/"))
	if vName == "" || relPath == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vault and path are required"})
		return
	}
	for _, av := range s.activeVaultsFor(p, vName) {
		if av.Name != vName || !av.AllowsPath(relPath) {
			continue
		}
		root := config.ExpandHome(av.Config.Path)
		if _, err := safeJoin(root, relPath); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		n, err := vault.ParseNote(vName, root, relPath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"vault":         n.VaultName,
			"path":          n.RelPath,
			"title":         n.Title,
			"status":        n.Status,
			"valid_from":    n.ValidFrom,
			"updated":       n.Updated,
			"owner":         n.Owner,
			"size_bytes":    n.SizeBytes,
			"max_bytes":     av.Config.MaxNoteBytes,
			"default_write": av.Config.DefaultWrite,
			"raw":           n.Raw,
		})
		return
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "access denied"})
}

func (s *Server) handleReindex(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	stats, err := s.Store.EnsureFresh(r.Context(), s.Cfg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "reindex", Outcome: "ok"})
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleCurate(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST required"})
		return
	}
	var reports []curator.CurateReport
	for _, av := range s.activeVaultsFor(p, "") {
		if !av.FullAccess {
			continue
		}
		rep, err := curator.Curate(av.Name, config.ExpandHome(av.Config.Path), av.Config.MaxNoteBytes, 90)
		if err == nil {
			reports = append(reports, rep)
		}
	}
	_ = s.Audit.Log(audit.Event{Actor: p.Name, Role: p.Role, Action: "curate", Outcome: "ok"})
	writeJSON(w, http.StatusOK, map[string]any{"reports": reports})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request, p principal) {
	limit := 50
	if raw := r.URL.Query().Get("n"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 500 {
			limit = v
		}
	}
	events, err := s.Audit.Tail(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": events})
}

func ListenAndServe(ctx context.Context, addr string, srv *Server) error {
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),
	}
	go func() {
		<-ctx.Done()
		_ = httpSrv.Close()
	}()
	return httpSrv.ListenAndServe()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func safeJoin(root, rel string) (string, error) {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(cleanRoot, filepath.FromSlash(rel)))
	if err != nil {
		return "", err
	}
	if target != cleanRoot && !strings.HasPrefix(target, cleanRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("path traversal rejected: %s", rel)
	}
	return target, nil
}
