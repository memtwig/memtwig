package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/curator"
	"github.com/vittoriocompagno/memtwig/internal/index"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/scope"
	"github.com/vittoriocompagno/memtwig/internal/search"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

type Server struct {
	Cfg           *config.Config
	Store         *index.Store
	Cwd           string
	ExplicitVault string
	Version       string
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ServeStdio runs a line-delimited JSON-RPC 2.0 MCP server over r/w.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	enc := json.NewEncoder(w)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}
		// Notifications have no ID
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		resp := s.handle(ctx, req)
		if err := enc.Encode(resp); err != nil {
			return err
		}
	}
	return sc.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) rpcResponse {
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "memtwig",
				"version": s.Version,
			},
		}
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		resp.Result = map[string]any{"tools": toolDefinitions()}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &rpcError{Code: -32602, Message: "invalid params"}
			return resp
		}
		text, isErr := s.callTool(ctx, params.Name, params.Arguments)
		resp.Result = map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": isErr,
		}
	default:
		resp.Error = &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
	return resp
}

func (s *Server) activeVaults(vaultOverride string) []scope.ActiveVault {
	v := vaultOverride
	if v == "" {
		v = s.ExplicitVault
	}
	return scope.Resolve(s.Cfg, s.Cwd, v)
}

func (s *Server) callTool(ctx context.Context, name string, args map[string]any) (string, bool) {
	strArg := func(k string) string {
		if v, ok := args[k].(string); ok {
			return v
		}
		return ""
	}
	intArg := func(k string, def int) int {
		if v, ok := args[k].(float64); ok {
			return int(v)
		}
		return def
	}

	switch name {
	case "memory_status":
		active := s.activeVaults("")
		type vInfo struct {
			Name          string `json:"name"`
			Kind          string `json:"kind"`
			DefaultWrite  string `json:"default_write"`
			FullAccess    bool   `json:"full_access"`
			MatchReason   string `json:"match_reason"`
			OpenProposals int    `json:"open_proposals"`
		}
		out := make([]vInfo, 0, len(active))
		for _, av := range active {
			props, _ := proposal.List(config.ExpandHome(av.Config.Path), false)
			out = append(out, vInfo{
				Name:          av.Name,
				Kind:          av.Config.Kind,
				DefaultWrite:  av.Config.DefaultWrite,
				FullAccess:    av.FullAccess,
				MatchReason:   av.MatchReason,
				OpenProposals: len(props),
			})
		}
		b, _ := json.MarshalIndent(out, "", "  ")
		return string(b), false

	case "memory_search":
		q := strArg("query")
		if q == "" {
			return "query is required", true
		}
		active := s.activeVaults(strArg("vault"))
		results, warn, err := search.Query(ctx, s.Store, s.Cfg, active, q, search.Options{
			TopK: intArg("limit", s.Cfg.Retrieval.DefaultTopK),
		})
		if err != nil {
			return err.Error(), true
		}
		payload := map[string]any{"results": results}
		if warn != "" {
			payload["warning"] = warn
		}
		b, _ := json.MarshalIndent(payload, "", "  ")
		return string(b), false

	case "memory_read":
		vName := strArg("vault")
		relPath := filepath.ToSlash(strings.TrimPrefix(strArg("path"), "/"))
		if vName == "" || relPath == "" {
			return "vault and path are required", true
		}
		av, ok := findAllowedVault(s.activeVaults(""), vName)
		if !ok || !av.AllowsPath(relPath) {
			return fmt.Sprintf("access denied to %s:%s in current scope", vName, relPath), true
		}
		abs, err := safeJoin(config.ExpandHome(av.Config.Path), relPath)
		if err != nil {
			return err.Error(), true
		}
		data, err := os.ReadFile(abs)
		if err != nil {
			return err.Error(), true
		}
		return string(data), false

	case "memory_write":
		vName := strArg("vault")
		relPath := filepath.ToSlash(strings.TrimPrefix(strArg("path"), "/"))
		content := strArg("content")
		if vName == "" || relPath == "" || content == "" {
			return "vault, path, and content are required", true
		}
		av, ok := findAllowedVault(s.activeVaults(""), vName)
		if !ok || !av.AllowsPath(relPath) {
			return fmt.Sprintf("access denied to %s:%s in current scope", vName, relPath), true
		}
		if av.Config.DefaultWrite != "direct" {
			return fmt.Sprintf("vault %q has default_write=%q: use memory_propose instead", vName, av.Config.DefaultWrite), true
		}
		root := config.ExpandHome(av.Config.Path)
		abs, err := safeJoin(root, relPath)
		if err != nil {
			return err.Error(), true
		}
		if existing, err := vault.ParseNote(vName, root, relPath); err == nil && existing.Owner != "" {
			return fmt.Sprintf("note %s:%s is owner-locked (owner: %s): use memory_propose instead", vName, relPath, existing.Owner), true
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err.Error(), true
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			return err.Error(), true
		}
		return fmt.Sprintf("written %s:%s (%d bytes)", vName, relPath, len(content)), false

	case "memory_propose":
		vName := strArg("vault")
		relPath := filepath.ToSlash(strings.TrimPrefix(strArg("path"), "/"))
		content := strArg("content")
		kind := strArg("kind")
		reason := strArg("reason")
		if vName == "" || relPath == "" || content == "" {
			return "vault, path, and content are required", true
		}
		av, ok := findAllowedVault(s.activeVaults(""), vName)
		if !ok {
			return fmt.Sprintf("vault %q is not in current scope", vName), true
		}
		if av.Config.DefaultWrite == "readonly" {
			return fmt.Sprintf("vault %q is readonly", vName), true
		}
		root := config.ExpandHome(av.Config.Path)
		if _, err := safeJoin(root, relPath); err != nil {
			return err.Error(), true
		}
		p, created, err := proposal.Add(root, proposal.Proposal{
			Kind:    kind,
			Path:    relPath,
			Content: content,
			Reason:  reason,
			Author:  "mcp-agent",
		})
		if err != nil {
			return err.Error(), true
		}
		if !created {
			return fmt.Sprintf("proposal %s already exists or was previously rejected", p.ID), false
		}
		return fmt.Sprintf("proposal %s queued for review on %s:%s", p.ID, vName, relPath), false

	case "memory_decide":
		vName := strArg("vault")
		summary := strArg("summary")
		project := strArg("project")
		notePath := strArg("note_path")
		if vName == "" || summary == "" {
			return "vault and summary are required", true
		}
		av, ok := findAllowedVault(s.activeVaults(""), vName)
		if !ok {
			return fmt.Sprintf("vault %q is not in current scope", vName), true
		}
		if av.Config.DefaultWrite == "readonly" {
			return fmt.Sprintf("vault %q is readonly", vName), true
		}
		if project == "" {
			project = vName
		}
		root := config.ExpandHome(av.Config.Path)
		if notePath != "" {
			if _, err := safeJoin(root, notePath); err != nil {
				return err.Error(), true
			}
		}
		msg, err := curator.Decide(root, project, summary, notePath, av.Config.DefaultWrite == "direct")
		if err != nil {
			return err.Error(), true
		}
		return msg, false

	default:
		return "unknown tool: " + name, true
	}
}

func findAllowedVault(active []scope.ActiveVault, name string) (scope.ActiveVault, bool) {
	for _, av := range active {
		if av.Name == name {
			return av, true
		}
	}
	return scope.ActiveVault{}, false
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

func toolDefinitions() []map[string]any {
	return []map[string]any{
		{
			"name":        "memory_search",
			"description": "Hybrid BM25 + vector search across active memtwig vaults in the current workspace scope.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query"},
					"limit": map[string]any{"type": "integer", "description": "Max results (default 5)"},
					"vault": map[string]any{"type": "string", "description": "Optional specific vault name"},
				},
				"required": []string{"query"},
			},
		},
		{
			"name":        "memory_read",
			"description": "Read a specific Markdown note from an active vault.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vault": map[string]any{"type": "string"},
					"path":  map[string]any{"type": "string", "description": "Relative path inside vault"},
				},
				"required": []string{"vault", "path"},
			},
		},
		{
			"name":        "memory_write",
			"description": "Write a note directly when vault allows direct writes and note is not owner-locked.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vault":   map[string]any{"type": "string"},
					"path":    map[string]any{"type": "string"},
					"content": map[string]any{"type": "string"},
				},
				"required": []string{"vault", "path", "content"},
			},
		},
		{
			"name":        "memory_propose",
			"description": "Propose a note create/update/append for human review (required for org/propose vaults and owner-locked notes).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vault":   map[string]any{"type": "string"},
					"path":    map[string]any{"type": "string"},
					"kind":    map[string]any{"type": "string", "enum": []string{"append", "update", "create"}},
					"content": map[string]any{"type": "string"},
					"reason":  map[string]any{"type": "string"},
				},
				"required": []string{"vault", "path", "content"},
			},
		},
		{
			"name":        "memory_decide",
			"description": "Log a dated decision in decisions/YYYY-MM.md and append/propose it on the related project note.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vault":     map[string]any{"type": "string"},
					"project":   map[string]any{"type": "string"},
					"summary":   map[string]any{"type": "string"},
					"note_path": map[string]any{"type": "string", "description": "Optional relative path to project note"},
				},
				"required": []string{"vault", "summary"},
			},
		},
		{
			"name":        "memory_status",
			"description": "List active vaults in current scope, write permissions, and open proposal counts.",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
}
