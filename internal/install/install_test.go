package install_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/install"
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
