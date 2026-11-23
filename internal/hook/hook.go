package hook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/scope"
	"github.com/memtwig/memtwig/internal/search"
)

var (
	reUserReq = regexp.MustCompile(`(?is)<user_request>\s*(.*?)\s*</user_request>`)
	reT3Instr = regexp.MustCompile(`(?is)<t3_code_instructions>.*?</t3_code_instructions>`)
	reRuntime = regexp.MustCompile(`(?is)<runtime_info>.*?</runtime_info>`)
)

// RunSessionStart injects the configured identity/SOUL.md file at session start when enabled.
func RunSessionStart(harness string, cfg *config.Config, w io.Writer) error {
	if !cfg.Identity.InjectOnSessionStart || strings.TrimSpace(cfg.Identity.SoulFile) == "" {
		return writeEmpty(harness, w)
	}
	soulPath := config.ExpandHome(cfg.Identity.SoulFile)
	raw, err := os.ReadFile(soulPath)
	if err != nil || len(strings.TrimSpace(string(raw))) == 0 {
		return writeEmpty(harness, w)
	}
	content := fmt.Sprintf("## Identity & Rules (%s)\n%s", cfg.Identity.SoulFile, strings.TrimSpace(string(raw)))
	switch harness {
	case "antigravity":
		return writeInjected("antigravity", content, w)
	default:
		out := map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "SessionStart",
				"additionalContext": content,
			},
		}
		b, err := json.Marshal(out)
		if err != nil {
			return err
		}
		_, err = w.Write(append(b, '\n'))
		return err
	}
}

// Run reads hook JSON payload from r, performs gated scoped retrieval, and writes harness-specific JSON to w.
func Run(ctx context.Context, harness string, cfg *config.Config, st *index.Store, r io.Reader, w io.Writer) error {
	_, _ = st.EnsureFresh(ctx, cfg)
	raw, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	var payload map[string]any
	if len(strings.TrimSpace(string(raw))) > 0 {
		_ = json.Unmarshal(raw, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}

	cwd := getString(payload, "cwd")
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	convID := getString(payload, "conversation_id")
	if convID == "" {
		convID = getString(payload, "session_id")
	}

	var rawPrompt string
	switch harness {
	case "antigravity":
		rawPrompt = getString(payload, "lastUserInput")
		if rawPrompt == "" {
			rawPrompt = getString(payload, "prompt")
		}
	default:
		rawPrompt = getString(payload, "prompt")
	}

	prompt := cleanPrompt(rawPrompt)
	if prompt == "" || search.IsCourtesy(prompt) {
		return writeEmpty(harness, w)
	}
	if convID != "" && alreadySeen(convID, prompt) {
		return writeEmpty(harness, w)
	}

	active := scope.Resolve(cfg, cwd, "")
	results, _, err := search.Query(ctx, st, cfg, active, prompt, search.Options{
		TopK:        cfg.Retrieval.DefaultTopK,
		MaxChars:    cfg.Retrieval.MaxSnippetChars,
		RequireGate: true,
	})
	if err != nil {
		return writeEmpty(harness, w)
	}

	var blocks []string
	if len(results) > 0 {
		var sb strings.Builder
		sb.WriteString("## From memtwig (relevant, non-exhaustive)\n")
		for _, res := range results {
			sb.WriteString(fmt.Sprintf("- [%s] %s: %s\n", res.Vault, res.Path, res.Snippet))
		}
		blocks = append(blocks, strings.TrimSpace(sb.String()))
	}
	if search.IsCorrection(prompt) {
		blocks = append(blocks, "Note: the user appears to be correcting a preference or fact. Persist durable rules or project facts in the appropriate memtwig vault before finishing.")
	}

	if len(blocks) == 0 {
		return writeEmpty(harness, w)
	}

	injected := strings.Join(blocks, "\n\n")
	return writeInjected(harness, injected, w)
}

func cleanPrompt(raw string) string {
	if raw == "" {
		return ""
	}
	if m := reUserReq.FindStringSubmatch(raw); len(m) == 2 {
		return strings.TrimSpace(m[1])
	}
	s := reT3Instr.ReplaceAllString(raw, "")
	s = reRuntime.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func writeEmpty(harness string, w io.Writer) error {
	if harness == "antigravity" {
		_, err := w.Write([]byte("{}\n"))
		return err
	}
	_, err := w.Write([]byte("{}\n"))
	return err
}

func writeInjected(harness, text string, w io.Writer) error {
	var out any
	switch harness {
	case "antigravity":
		out = map[string]any{
			"injectSteps": []map[string]any{
				{"role": "system", "content": text},
			},
		}
	default: // claude, codex
		out = map[string]any{
			"hookSpecificOutput": map[string]any{
				"hookEventName":     "UserPromptSubmit",
				"additionalContext": text,
			},
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		return err
	}
	_, err = w.Write(append(b, '\n'))
	return err
}

func getString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func alreadySeen(convID, prompt string) bool {
	cacheDir, err := config.DefaultCacheDir()
	if err != nil {
		return false
	}
	_ = os.MkdirAll(cacheDir, 0o755)
	p := filepath.Join(cacheDir, "hook-seen.json")
	seen := map[string]string{}
	if b, err := os.ReadFile(p); err == nil {
		_ = json.Unmarshal(b, &seen)
	}
	sum := sha256.Sum256([]byte(prompt))
	digest := hex.EncodeToString(sum[:])[:16]
	if seen[convID] == digest {
		return true
	}
	seen[convID] = digest
	if len(seen) > 200 {
		seen = map[string]string{convID: digest}
	}
	if b, err := json.Marshal(seen); err == nil {
		_ = os.WriteFile(p, b, 0o644)
	}
	return false
}
