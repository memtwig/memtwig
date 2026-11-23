package hook_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/hook"
)

func TestRunSessionStartInjectsSoulFile(t *testing.T) {
	tmp := t.TempDir()
	soul := filepath.Join(tmp, "SOUL.md")
	if err := os.WriteFile(soul, []byte("Always answer concisely.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Identity.SoulFile = soul
	cfg.Identity.InjectOnSessionStart = true

	var out bytes.Buffer
	if err := hook.RunSessionStart("claude", cfg, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "SessionStart") || !strings.Contains(out.String(), "Always answer concisely.") {
		t.Fatalf("expected SessionStart soul injection, got %s", out.String())
	}

	var outAG bytes.Buffer
	if err := hook.RunSessionStart("antigravity", cfg, &outAG); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(outAG.String(), "injectSteps") || !strings.Contains(outAG.String(), "Always answer concisely.") {
		t.Fatalf("expected antigravity injectSteps, got %s", outAG.String())
	}
}
