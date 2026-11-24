package install

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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
	homeDir, binaryPath, err := resolveHomeAndBin(binaryPath, homeDir)
	if err != nil {
		return Result{}, err
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
	case "hermes":
		return installHermes(binaryPath, homeDir, dryRun)
	default:
		return Result{}, fmt.Errorf("unsupported harness %q (supported: claude, cursor, codex, antigravity, hermes, all)", harness)
	}
}

// InstallAll detects all installed supported AI harnesses under homeDir and configures them.
func InstallAll(binaryPath, homeDir string, dryRun bool) ([]Result, error) {
	homeDir, binaryPath, err := resolveHomeAndBin(binaryPath, homeDir)
	if err != nil {
		return nil, err
	}
	var results []Result
	if _, err := os.Stat(filepath.Join(homeDir, ".claude")); err == nil {
		r, err := installClaude(binaryPath, homeDir, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".cursor")); err == nil {
		r, err := installCursor(binaryPath, homeDir, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".codex")); err == nil {
		r, err := installCodex(binaryPath, homeDir, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	if len(findAntigravityConfigDirs(homeDir)) > 0 {
		r, err := installAntigravity(binaryPath, homeDir, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".hermes", "config.yaml")); err == nil {
		r, err := installHermes(binaryPath, homeDir, dryRun)
		if err != nil {
			return results, err
		}
		results = append(results, r)
	}
	return results, nil
}

func resolveHomeAndBin(binaryPath, homeDir string) (string, string, error) {
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return "", "", err
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
	return homeDir, binaryPath, nil
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

	hooksChanged := false
	ensureHook := func(event, cmdStr string) {
		arr, _ := hooks[event].([]any)
		for _, entry := range arr {
			if raw, _ := json.Marshal(entry); strings.Contains(string(raw), cmdStr) {
				return
			}
		}
		arr = append(arr, map[string]any{
			"hooks": []map[string]any{
				{"type": "command", "command": cmdStr},
			},
		})
		hooks[event] = arr
		hooksChanged = true
	}

	ensureHook("UserPromptSubmit", bin+" hook claude")
	ensureHook("SessionStart", bin+" hook claude --event session-start")
	data["hooks"] = hooks

	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return Result{}, err
	}

	mcpPath := filepath.Join(home, ".claude.json")
	mcpData, err := readJSONMap(mcpPath)
	if err != nil {
		return Result{}, err
	}
	servers, _ := mcpData["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	mcpChanged := false
	if _, exists := servers["memtwig"]; !exists {
		servers["memtwig"] = map[string]any{
			"type":    "stdio",
			"command": bin,
			"args":    []string{"mcp"},
		}
		mcpData["mcpServers"] = servers
		mcpChanged = true
	}
	mcpOut, err := json.MarshalIndent(mcpData, "", "  ")
	if err != nil {
		return Result{}, err
	}

	preview := fmt.Sprintf("# %s\n%s\n\n# %s\n%s", path, string(out), mcpPath, string(mcpOut))
	res := Result{
		Harness:    "claude",
		ConfigPath: path + " + " + mcpPath,
		Changed:    hooksChanged || mcpChanged,
		DryRun:     dryRun,
		Preview:    preview,
	}
	if !dryRun {
		if hooksChanged {
			if err := writeWithBackup(path, append(out, '\n')); err != nil {
				return res, err
			}
		}
		if mcpChanged {
			if err := writeWithBackup(mcpPath, append(mcpOut, '\n')); err != nil {
				return res, err
			}
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

func findAntigravityConfigDirs(home string) []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			return
		}
		clean := filepath.Clean(d)
		if !seen[clean] {
			seen[clean] = true
			dirs = append(dirs, clean)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "*", "config"))
	sort.Strings(matches)
	for _, m := range matches {
		add(m)
	}
	add(filepath.Join(home, ".gemini", "antigravity"))
	return dirs
}

func installAntigravity(bin, home string, dryRun bool) (Result, error) {
	dirs := findAntigravityConfigDirs(home)
	if len(dirs) == 0 {
		mcpPreview := map[string]any{
			"mcpServers": map[string]any{
				"memtwig": map[string]any{
					"command": bin,
					"args":    []string{"mcp"},
				},
			},
		}
		hooksPreview := map[string]any{
			"memtwig": map[string]any{
				"SessionStart": []map[string]any{
					{"type": "command", "command": bin + " hook antigravity --event session-start", "timeout": 5},
				},
				"PreInvocation": []map[string]any{
					{"type": "command", "command": bin + " hook antigravity", "timeout": 12},
				},
			},
		}
		mcpB, _ := json.MarshalIndent(mcpPreview, "", "  ")
		hooksB, _ := json.MarshalIndent(hooksPreview, "", "  ")
		return Result{
			Harness:    "antigravity",
			ConfigPath: "(no Antigravity config folder found — manual snippet)",
			Changed:    false,
			DryRun:     true,
			Preview:    fmt.Sprintf("# mcp_config.json\n%s\n\n# hooks.json\n%s", string(mcpB), string(hooksB)),
		}, nil
	}

	var configPaths []string
	var previews []string
	anyChanged := false

	for _, dir := range dirs {
		mcpPath := filepath.Join(dir, "mcp_config.json")
		mcpData, err := readJSONMap(mcpPath)
		if err != nil {
			return Result{}, err
		}
		servers, _ := mcpData["mcpServers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
		}
		mcpChanged := false
		if _, exists := servers["memtwig"]; !exists {
			servers["memtwig"] = map[string]any{
				"command": bin,
				"args":    []string{"mcp"},
			}
			mcpData["mcpServers"] = servers
			mcpChanged = true
		}
		mcpOut, err := json.MarshalIndent(mcpData, "", "  ")
		if err != nil {
			return Result{}, err
		}

		hooksPath := filepath.Join(dir, "hooks.json")
		hooksData, err := readJSONMap(hooksPath)
		if err != nil {
			return Result{}, err
		}
		rawHooks, _ := json.Marshal(hooksData)
		hasSession := strings.Contains(string(rawHooks), "hook antigravity --event session-start")
		hasPrompt := strings.Contains(string(rawHooks), "hook antigravity\"") || strings.Contains(string(rawHooks), "hook antigravity'") || strings.Contains(string(rawHooks), "hook antigravity ")
		hooksChanged := false
		if !hasSession || !hasPrompt {
			hooksData["memtwig"] = map[string]any{
				"SessionStart": []map[string]any{
					{"type": "command", "command": bin + " hook antigravity --event session-start", "timeout": 5},
				},
				"PreInvocation": []map[string]any{
					{"type": "command", "command": bin + " hook antigravity", "timeout": 12},
				},
			}
			hooksChanged = true
		}
		hooksOut, err := json.MarshalIndent(hooksData, "", "  ")
		if err != nil {
			return Result{}, err
		}

		if mcpChanged || hooksChanged {
			anyChanged = true
		}
		configPaths = append(configPaths, mcpPath, hooksPath)
		previews = append(previews, fmt.Sprintf("# %s\n%s\n\n# %s\n%s", mcpPath, string(mcpOut), hooksPath, string(hooksOut)))

		if !dryRun {
			if mcpChanged {
				if err := writeWithBackup(mcpPath, append(mcpOut, '\n')); err != nil {
					return Result{}, err
				}
			}
			if hooksChanged {
				if err := writeWithBackup(hooksPath, append(hooksOut, '\n')); err != nil {
					return Result{}, err
				}
			}
		}
	}

	return Result{
		Harness:    "antigravity",
		ConfigPath: strings.Join(configPaths, " + "),
		Changed:    anyChanged,
		DryRun:     dryRun,
		Preview:    strings.Join(previews, "\n\n"),
	}, nil
}

func installHermes(bin, home string, dryRun bool) (Result, error) {
	path := filepath.Join(home, ".hermes", "config.yaml")
	existingBytes, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return Result{}, err
	}
	existing := string(existingBytes)
	lines := strings.Split(existing, "\n")

	// Check if memtwig is already configured under mcp_servers:
	inMCP := false
	mcpLineIdx := -1
	mcpEmptyObjIdx := -1
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(l) - len(strings.TrimLeft(l, " "))
		if indent == 0 {
			inMCP = false
			if trimmed == "mcp_servers:" {
				inMCP = true
				mcpLineIdx = i
			} else if strings.HasPrefix(trimmed, "mcp_servers:") && strings.Contains(trimmed, "{}") {
				mcpEmptyObjIdx = i
			}
			continue
		}
		if inMCP && (trimmed == "memtwig:" || strings.HasPrefix(trimmed, "memtwig:")) {
			return Result{Harness: "hermes", ConfigPath: path, Changed: false, DryRun: dryRun, Preview: existing}, nil
		}
	}

	block := []string{
		"  memtwig:",
		"    command: " + bin,
		"    args:",
		"      - mcp",
		"    enabled: true",
	}

	var updated string
	switch {
	case mcpLineIdx >= 0:
		var out []string
		out = append(out, lines[:mcpLineIdx+1]...)
		out = append(out, block...)
		out = append(out, lines[mcpLineIdx+1:]...)
		updated = strings.Join(out, "\n")
	case mcpEmptyObjIdx >= 0:
		var out []string
		out = append(out, lines[:mcpEmptyObjIdx]...)
		out = append(out, "mcp_servers:")
		out = append(out, block...)
		out = append(out, lines[mcpEmptyObjIdx+1:]...)
		updated = strings.Join(out, "\n")
	default:
		base := strings.TrimRight(existing, "\n")
		if base != "" {
			base += "\n\n"
		}
		updated = base + "mcp_servers:\n" + strings.Join(block, "\n") + "\n"
	}

	res := Result{Harness: "hermes", ConfigPath: path, Changed: true, DryRun: dryRun, Preview: updated}
	if !dryRun {
		if err := writeWithBackup(path, []byte(updated)); err != nil {
			return res, err
		}
	}
	return res, nil
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
