package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Result struct {
	Harness    string
	ConfigPath string
	Changed    bool
	DryRun     bool
	Preview    string
}

// Install configures the given AI harness to use memtwig (hook and/or MCP server).
// homeDir may be overridden for tests; if empty, os.UserHomeDir() is used.
func Install(harness, binaryPath, homeDir string, dryRun bool) (Result, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
	}
	if binaryPath == "" {
		exe, err := os.Executable()
		if err == nil {
			binaryPath = exe
		} else {
			binaryPath = "memtwig"
		}
	}

	switch strings.ToLower(strings.TrimSpace(harness)) {
	case "claude":
		return installClaude(binaryPath, homeDir, dryRun)
	case "cursor":
		return installCursor(binaryPath, homeDir, dryRun)
	case "codex":
		return installCodex(binaryPath, homeDir, dryRun)
	case "antigravity":
		return installAntigravity(binaryPath, homeDir, dryRun)
	default:
		return Result{}, fmt.Errorf("unsupported harness %q (supported: claude, cursor, codex, antigravity)", harness)
	}
}

func installClaude(bin, home string, dryRun bool) (Result, error) {
	path := filepath.Join(home, ".claude", "settings.json")
	data, err := readJSONMap(path)
	if err != nil {
		return Result{}, err
	}
	hooks, _ := data["hooks"].(map[string]any)
	if hooks == nil {
		hooks = map[string]any{}
	}
	cmdStr := bin + " hook claude"
	ups, _ := hooks["UserPromptSubmit"].([]any)
	already := false
	for _, entry := range ups {
		if m, ok := entry.(map[string]any); ok {
			if raw, _ := json.Marshal(m); strings.Contains(string(raw), "memtwig hook claude") {
				already = true
				break
			}
		}
	}
	if !already {
		ups = append(ups, map[string]any{
			"hooks": []map[string]any{
				{"type": "command", "command": cmdStr},
			},
		})
		hooks["UserPromptSubmit"] = ups
		data["hooks"] = hooks
	}
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return Result{}, err
	}
	res := Result{Harness: "claude", ConfigPath: path, Changed: !already, DryRun: dryRun, Preview: string(out)}
	if !dryRun && !already {
		if err := writeWithBackup(path, append(out, '\n')); err != nil {
			return res, err
		}
	}
	return res, nil
}

func installCursor(bin, home string, dryRun bool) (Result, error) {
	path := filepath.Join(home, ".cursor", "mcp.json")
	data, err := readJSONMap(path)
	if err != nil {
		return Result{}, err
	}
	servers, _ := data["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	_, exists := servers["memtwig"]
	servers["memtwig"] = map[string]any{
		"command": bin,
		"args":    []string{"mcp"},
	}
	data["mcpServers"] = servers
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return Result{}, err
	}
	res := Result{Harness: "cursor", ConfigPath: path, Changed: !exists, DryRun: dryRun, Preview: string(out)}
	if !dryRun {
		if err := writeWithBackup(path, append(out, '\n')); err != nil {
			return res, err
		}
	}
	return res, nil
}

func installCodex(bin, home string, dryRun bool) (Result, error) {
	path := filepath.Join(home, ".codex", "config.toml")
	existingBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	existing := string(existingBytes)
	if strings.Contains(existing, "[mcp_servers.memtwig]") {
		return Result{Harness: "codex", ConfigPath: path, Changed: false, DryRun: dryRun, Preview: existing}, nil
	}
	block := fmt.Sprintf("\n[mcp_servers.memtwig]\ncommand = %q\nargs = [\"mcp\"]\n", bin)
	updated := strings.TrimRight(existing, "\n") + "\n" + block
	res := Result{Harness: "codex", ConfigPath: path, Changed: true, DryRun: dryRun, Preview: updated}
	if !dryRun {
		if err := writeWithBackup(path, []byte(updated)); err != nil {
			return res, err
		}
	}
	return res, nil
}

func installAntigravity(bin, home string, dryRun bool) (Result, error) {
	snippet := map[string]any{
		"mcpServers": map[string]any{
			"memtwig": map[string]any{
				"command": bin,
				"args":    []string{"mcp"},
			},
		},
		"hooks": map[string]any{
			"PreInvocation": []map[string]any{
				{"command": bin + " hook antigravity"},
			},
		},
	}
	out, _ := json.MarshalIndent(snippet, "", "  ")
	return Result{
		Harness:    "antigravity",
		ConfigPath: "(manual snippet — paste into your Antigravity/T3 MCP & hooks config)",
		Changed:    false,
		DryRun:     true,
		Preview:    string(out),
	}, nil
}

func readJSONMap(path string) (map[string]any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		return nil, err
	}
	m := map[string]any{}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

func writeWithBackup(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if prev, err := os.ReadFile(path); err == nil {
		_ = os.WriteFile(path+".bak", prev, 0o600)
	}
	return os.WriteFile(path, content, 0o644)
}
