package install_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/install"
)

func TestInstallClaudeCursorCodex(t *testing.T) {
	home := t.TempDir()

	// Claude: first install writes settings.json, second install is idempotent
	r1, err := install.Install("claude", "/usr/local/bin/memtwig", home, false)
	if err != nil || !r1.Changed {
		t.Fatalf("claude install 1 failed: %+v err=%v", r1, err)
	}
	r2, err := install.Install("claude", "/usr/local/bin/memtwig", home, false)
	if err != nil || r2.Changed {
		t.Fatalf("claude install 2 should be idempotent: %+v err=%v", r2, err)
	}

	// Cursor
	rc, err := install.Install("cursor", "/usr/local/bin/memtwig", home, false)
	if err != nil || !rc.Changed {
		t.Fatalf("cursor install failed: %+v err=%v", rc, err)
	}
	rawCursor, err := os.ReadFile(filepath.Join(home, ".cursor", "mcp.json"))
	if err != nil || !strings.Contains(string(rawCursor), "memtwig") {
		t.Fatalf("cursor mcp.json missing memtwig")
	}

	// Codex
	rx, err := install.Install("codex", "/usr/local/bin/memtwig", home, false)
	if err != nil || !rx.Changed {
		t.Fatalf("codex install failed: %+v err=%v", rx, err)
	}
	rawCodex, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
	if err != nil || !strings.Contains(string(rawCodex), "[mcp_servers.memtwig]") {
		t.Fatalf("codex config.toml missing memtwig")
	}
}

func TestInstallAntigravityHermesAndAll(t *testing.T) {
	home := t.TempDir()

	// Seed Antigravity T3 config with existing impeccable + firecrawl
	agCfg := filepath.Join(home, ".t3", "userdata", "providers", "antigravity", "prof1", "config")
	if err := os.MkdirAll(agCfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agCfg, "hooks.json"), []byte(`{"impeccable":{"Stop":[]}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agCfg, "mcp_config.json"), []byte(`{"mcpServers":{"firecrawl":{"command":"fc","args":["mcp"]}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// Seed Hermes config with comments
	hermesDir := filepath.Join(home, ".hermes")
	if err := os.MkdirAll(hermesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hermesDir, "config.yaml"), []byte("# top comment\nmcp_servers:\n  firecrawl:\n    command: fc\n    enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Install Antigravity
	ra, err := install.Install("antigravity", "/usr/local/bin/memtwig", home, false)
	if err != nil || !ra.Changed {
		t.Fatalf("antigravity install failed: %+v err=%v", ra, err)
	}
	hooksRaw, _ := os.ReadFile(filepath.Join(agCfg, "hooks.json"))
	mcpRaw, _ := os.ReadFile(filepath.Join(agCfg, "mcp_config.json"))
	if !strings.Contains(string(hooksRaw), "impeccable") || !strings.Contains(string(hooksRaw), "session-start") || !strings.Contains(string(hooksRaw), "PreInvocation") {
		t.Fatalf("unexpected antigravity hooks.json: %s", string(hooksRaw))
	}
	if !strings.Contains(string(mcpRaw), "firecrawl") || !strings.Contains(string(mcpRaw), "memtwig") {
		t.Fatalf("unexpected antigravity mcp_config.json: %s", string(mcpRaw))
	}
	// Idempotent
	ra2, err := install.Install("antigravity", "/usr/local/bin/memtwig", home, false)
	if err != nil || ra2.Changed {
		t.Fatalf("antigravity install should be idempotent: %+v err=%v", ra2, err)
	}

	// Install Hermes (comment-preserving)
	rh, err := install.Install("hermes", "/usr/local/bin/memtwig", home, false)
	if err != nil || !rh.Changed {
		t.Fatalf("hermes install failed: %+v err=%v", rh, err)
	}
	hermesRaw, _ := os.ReadFile(filepath.Join(hermesDir, "config.yaml"))
	if !strings.Contains(string(hermesRaw), "# top comment") || !strings.Contains(string(hermesRaw), "memtwig:") || !strings.Contains(string(hermesRaw), "firecrawl:") {
		t.Fatalf("unexpected hermes config.yaml: %s", string(hermesRaw))
	}
	rh2, err := install.Install("hermes", "/usr/local/bin/memtwig", home, false)
	if err != nil || rh2.Changed {
		t.Fatalf("hermes install should be idempotent: %+v err=%v", rh2, err)
	}

	// InstallAll covers existing targets idempotently + detects newly added Codex
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	results, err := install.InstallAll("/usr/local/bin/memtwig", home, false)
	if err != nil || len(results) != 3 {
		t.Fatalf("expected 3 results from InstallAll (codex, antigravity, hermes), got %+v err=%v", results, err)
	}
}
