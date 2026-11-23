package hook

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/index"
)

func setupHookEnv(t *testing.T) (*config.Config, *index.Store, string) {
	t.Helper()
	tmp := t.TempDir()
	cacheDir := filepath.Join(tmp, "cache")
	t.Setenv("MEMTWIG_CACHE_DIR", cacheDir)

	vaultDir := filepath.Join(tmp, "vault")
	if err := os.MkdirAll(filepath.Join(vaultDir, "projects"), 0o755); err != nil {
		t.Fatal(err)
	}
	note := `---
status: active
updated: 2026-10-01
---

# Memtwig Deployment

Memtwig runs on port 8787 using SQLite FTS5 for hybrid retrieval.
`
	if err := os.WriteFile(filepath.Join(vaultDir, "projects", "deploy.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{Path: vaultDir, Kind: "personal"}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}
	return cfg, st, tmp
}

func TestRunClaudeAndCodex(t *testing.T) {
	cfg, st, cwd := setupHookEnv(t)

	for _, harness := range []string{"claude", "codex"} {
		payload, _ := json.Marshal(map[string]any{
			"prompt":          "How does memtwig hybrid retrieval work?",
			"cwd":             cwd,
			"conversation_id": "conv-" + harness,
		})
		var out bytes.Buffer
		if err := Run(context.Background(), harness, cfg, st, bytes.NewReader(payload), &out); err != nil {
			t.Fatalf("%s Run failed: %v", harness, err)
		}
		var resp struct {
			HookSpecificOutput struct {
				HookEventName     string `json:"hookEventName"`
				AdditionalContext string `json:"additionalContext"`
			} `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			t.Fatalf("%s invalid JSON %q: %v", harness, out.String(), err)
		}
		if resp.HookSpecificOutput.HookEventName != "UserPromptSubmit" {
			t.Errorf("%s HookEventName = %q, want UserPromptSubmit", harness, resp.HookSpecificOutput.HookEventName)
		}
		if !strings.Contains(resp.HookSpecificOutput.AdditionalContext, "projects/deploy.md") {
			t.Errorf("%s expected deploy.md snippet in %q", harness, resp.HookSpecificOutput.AdditionalContext)
		}
	}
}

func TestRunAntigravityUnwrap(t *testing.T) {
	cfg, st, cwd := setupHookEnv(t)

	wrapped := "<t3_code_instructions>ignore</t3_code_instructions>\n<user_request>Check memtwig SQLite FTS5 port</user_request>\n<runtime_info>info</runtime_info>"
	payload, _ := json.Marshal(map[string]any{
		"lastUserInput":   wrapped,
		"cwd":             cwd,
		"conversation_id": "conv-agy",
	})
	var out bytes.Buffer
	if err := Run(context.Background(), "antigravity", cfg, st, bytes.NewReader(payload), &out); err != nil {
		t.Fatalf("antigravity Run failed: %v", err)
	}
	var resp struct {
		InjectSteps []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"injectSteps"`
	}
	if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON %q: %v", out.String(), err)
	}
	if len(resp.InjectSteps) != 1 || resp.InjectSteps[0].Role != "system" {
		t.Fatalf("unexpected injectSteps: %+v", resp.InjectSteps)
	}
	if !strings.Contains(resp.InjectSteps[0].Content, "projects/deploy.md") {
		t.Errorf("expected deploy.md snippet in %q", resp.InjectSteps[0].Content)
	}
}

func TestRunCourtesyGate(t *testing.T) {
	cfg, st, cwd := setupHookEnv(t)

	for _, msg := range []string{"grazie mille!", "ok, sounds good"} {
		payload, _ := json.Marshal(map[string]any{
			"prompt":          msg,
			"cwd":             cwd,
			"conversation_id": "conv-courtesy",
		})
		var out bytes.Buffer
		if err := Run(context.Background(), "claude", cfg, st, bytes.NewReader(payload), &out); err != nil {
			t.Fatal(err)
		}
		if strings.TrimSpace(out.String()) != "{}" {
			t.Errorf("courtesy %q returned %q, want {}", msg, out.String())
		}
	}
}

func TestRunCorrectionNudge(t *testing.T) {
	cfg, st, cwd := setupHookEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"prompt":          "No, smettila di usare quel formato per i log",
		"cwd":             cwd,
		"conversation_id": "conv-corr",
	})
	var out bytes.Buffer
	if err := Run(context.Background(), "claude", cfg, st, bytes.NewReader(payload), &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "correcting a preference or fact") {
		t.Errorf("expected correction nudge in output, got %q", out.String())
	}
}

func TestRunDedupViaCacheDir(t *testing.T) {
	cfg, st, cwd := setupHookEnv(t)

	payload, _ := json.Marshal(map[string]any{
		"prompt":          "How does memtwig hybrid retrieval work?",
		"cwd":             cwd,
		"conversation_id": "conv-dedup",
	})

	var first bytes.Buffer
	if err := Run(context.Background(), "claude", cfg, st, bytes.NewReader(payload), &first); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(first.String(), "projects/deploy.md") {
		t.Fatalf("expected first call to inject, got %q", first.String())
	}

	var second bytes.Buffer
	if err := Run(context.Background(), "claude", cfg, st, bytes.NewReader(payload), &second); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(second.String()) != "{}" {
		t.Fatalf("expected second identical call to dedup to {}, got %q", second.String())
	}

	seenFile := filepath.Join(os.Getenv("MEMTWIG_CACHE_DIR"), "hook-seen.json")
	if _, err := os.Stat(seenFile); err != nil {
		t.Errorf("expected hook-seen.json in MEMTWIG_CACHE_DIR: %v", err)
	}
}
