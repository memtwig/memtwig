package hook_test

import (
	"bytes"
	"encoding/json"
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
	var respAG struct {
		InjectSteps []struct {
			SystemMessage struct {
				SystemMessage string `json:"systemMessage"`
			} `json:"systemMessage"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal(outAG.Bytes(), &respAG); err != nil {
		t.Fatalf("invalid antigravity JSON %s: %v", outAG.String(), err)
	}
	if len(respAG.InjectSteps) != 1 || !strings.Contains(respAG.InjectSteps[0].SystemMessage.SystemMessage, "Always answer concisely.") {
		t.Fatalf("expected antigravity injectSteps.systemMessage.systemMessage, got %s", outAG.String())
	}
}
