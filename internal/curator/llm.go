package curator

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

type SemanticFinding struct {
	NoteA      string `json:"note_a"`
	NoteB      string `json:"note_b"`
	Issue      string `json:"issue"`
	Suggestion string `json:"suggestion"`
}

type SemanticReport struct {
	VaultName        string            `json:"vault"`
	PairsExamined    int               `json:"pairs_examined"`
	Findings         []SemanticFinding `json:"findings"`
	ProposalsCreated int               `json:"proposals_created"`
}

// CurateSemantic compares pairs of related notes in a vault using an OpenAI-compatible or Ollama LLM endpoint.
// It NEVER modifies live notes directly; all findings become proposals in .curator/proposals.json.
func CurateSemantic(ctx context.Context, vaultName, vaultRoot string, llmCfg config.LLMConfig, maxPairs int) (SemanticReport, error) {
	rep := SemanticReport{VaultName: vaultName}
	if llmCfg.Provider == "" || llmCfg.Provider == "none" {
		return rep, errors.New("LLM provider is disabled in config ([llm] provider = \"openai\" | \"ollama\")")
	}
	if maxPairs <= 0 {
		maxPairs = 6
	}

	notes, err := vault.WalkNotes(vaultName, vaultRoot)
	if err != nil {
		return rep, err
	}
	pairs := pickRelatedPairs(notes, maxPairs)
	client := &http.Client{Timeout: time.Duration(llmCfg.TimeoutMS) * time.Millisecond}

	for _, pair := range pairs {
		rep.PairsExamined++
		finding, err := comparePairWithLLM(ctx, client, llmCfg, pair[0], pair[1])
		if err != nil || finding == nil {
			continue
		}
		rep.Findings = append(rep.Findings, *finding)
		_, created, _ := proposal.Add(vaultRoot, proposal.Proposal{
			Kind:    "review",
			Path:    finding.NoteA,
			Content: fmt.Sprintf("Potential contradiction/overlap with `%s`: %s\nSuggested fix: %s", finding.NoteB, finding.Issue, finding.Suggestion),
			Reason:  "semantic curator finding",
			Author:  "memtwig-curator-llm",
		})
		if created {
			rep.ProposalsCreated++
		}
	}
	return rep, nil
}

// pickRelatedPairs selects note pairs sharing directory or significant title/content tokens.
func pickRelatedPairs(notes []vault.Note, maxPairs int) [][2]vault.Note {
	var out [][2]vault.Note
	for i := 0; i < len(notes) && len(out) < maxPairs; i++ {
		for j := i + 1; j < len(notes) && len(out) < maxPairs; j++ {
			a, b := notes[i], notes[j]
			if a.Status == "closed" || b.Status == "closed" {
				continue
			}
			if shareTopDir(a.RelPath, b.RelPath) && shareKeywords(a.Title+" "+a.RelPath, b.Title+" "+b.RelPath) {
				out = append(out, [2]vault.Note{a, b})
			}
		}
	}
	return out
}

func shareTopDir(a, b string) bool {
	pa := strings.Split(a, "/")[0]
	pb := strings.Split(b, "/")[0]
	return pa != "" && pa == pb
}

func shareKeywords(a, b string) bool {
	wordsA := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ").Replace(a))) {
		if len(w) >= 4 {
			wordsA[w] = true
		}
	}
	for _, w := range strings.Fields(strings.ToLower(strings.NewReplacer("-", " ", "_", " ", "/", " ", ".", " ").Replace(b))) {
		if len(w) >= 4 && wordsA[w] {
			return true
		}
	}
	return false
}

func comparePairWithLLM(ctx context.Context, client *http.Client, cfg config.LLMConfig, a, b vault.Note) (*SemanticFinding, error) {
	trim := func(s string, max int) string {
		if len(s) <= max {
			return s
		}
		return s[:max] + "\n...[truncated]"
	}
	prompt := fmt.Sprintf(
		"Compare these two Markdown memory notes for factual contradictions or duplicate stale facts.\n"+
			"Return strictly JSON: {\"has_issue\": bool, \"issue\": string, \"suggestion\": string}.\n"+
			"If there is no clear contradiction or duplication, return {\"has_issue\": false}.\n\n"+
			"=== NOTE A: %s ===\n%s\n\n=== NOTE B: %s ===\n%s\n",
		a.RelPath, trim(a.Body, 2500), b.RelPath, trim(b.Body, 2500),
	)

	endpoint := cfg.Endpoint
	if endpoint == "" {
		if cfg.Provider == "ollama" {
			endpoint = "http://127.0.0.1:11434/v1/chat/completions"
		} else {
			endpoint = "https://api.openai.com/v1/chat/completions"
		}
	}
	model := cfg.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	reqBody, _ := json.Marshal(map[string]any{
		"model":       model,
		"temperature": 0,
		"messages": []map[string]string{
			{"role": "system", "content": "You are a strict knowledge base curator. Reply with valid JSON only."},
			{"role": "user", "content": prompt},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKeyEnv != "" {
		if key := os.Getenv(cfg.APIKeyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	} else if key := os.Getenv("OPENAI_API_KEY"); key != "" && cfg.Provider == "openai" {
		req.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, string(raw))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &chatResp); err != nil || len(chatResp.Choices) == 0 {
		return nil, errors.New("empty LLM response")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var parsed struct {
		HasIssue   bool   `json:"has_issue"`
		Issue      string `json:"issue"`
		Suggestion string `json:"suggestion"`
	}
	if err := json.Unmarshal([]byte(content), &parsed); err != nil || !parsed.HasIssue {
		return nil, nil
	}
	return &SemanticFinding{
		NoteA:      a.RelPath,
		NoteB:      b.RelPath,
		Issue:      parsed.Issue,
		Suggestion: parsed.Suggestion,
	}, nil
}
