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

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/proposal"
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
	return CurateSemanticWithOptions(ctx, vaultName, vaultRoot, llmCfg, CandidateOptions{MaxPairs: maxPairs})
}

// CurateSemanticWithOptions compares candidate note pairs using opts and persists reviewed pairs in .curator/judged_pairs.json.
func CurateSemanticWithOptions(ctx context.Context, vaultName, vaultRoot string, llmCfg config.LLMConfig, opts CandidateOptions) (SemanticReport, error) {
	rep := SemanticReport{VaultName: vaultName}
	if llmCfg.Provider == "" || llmCfg.Provider == "none" {
		return rep, errors.New("LLM provider is disabled in config ([llm] provider = \"openai\" | \"ollama\" | \"openrouter\" | \"groq\")")
	}
	if opts.MaxPairs <= 0 {
		opts.MaxPairs = defaultMaxCandidatePairs
	}

	cands, err := FindCandidatePairs(vaultName, vaultRoot, opts)
	if err != nil {
		return rep, err
	}
	timeout := time.Duration(llmCfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	client := &http.Client{Timeout: timeout}

	var examinedOK []CandidatePair
	for _, cand := range cands {
		rep.PairsExamined++
		finding, err := compareCandidateWithLLM(ctx, client, llmCfg, cand)
		if err != nil {
			continue
		}
		examinedOK = append(examinedOK, cand)
		if finding == nil {
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
	if len(examinedOK) > 0 {
		_ = MarkPairsJudged(vaultRoot, examinedOK)
	}
	return rep, nil
}

func compareCandidateWithLLM(ctx context.Context, client *http.Client, cfg config.LLMConfig, cand CandidatePair) (*SemanticFinding, error) {
	prompt := fmt.Sprintf(
		"Compare these two Markdown memory excerpts for factual contradictions or duplicate stale facts (dates, prices, service hosts, active decisions).\n"+
			"Notes contain dated chronological entries. If a newer dated entry corrects or supersedes an older one, that is NOT a contradiction.\n"+
			"Return strictly JSON: {\"has_issue\": bool, \"issue\": string, \"suggestion\": string}.\n"+
			"If there is no clear contradiction or duplication, return {\"has_issue\": false}.\n\n"+
			"=== NOTE A: %s ===\n%s\n\n=== NOTE B: %s ===\n%s\n",
		cand.NoteA, cand.ExcerptA, cand.NoteB, cand.ExcerptB,
	)

	endpoint := cfg.Endpoint
	model := cfg.Model
	defaultKeyEnv := "OPENAI_API_KEY"
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "ollama":
		if endpoint == "" {
			endpoint = "http://127.0.0.1:11434/v1/chat/completions"
		}
		if model == "" {
			model = "llama3.2"
		}
		defaultKeyEnv = ""
	case "openrouter":
		if endpoint == "" {
			endpoint = "https://openrouter.ai/api/v1/chat/completions"
		}
		if model == "" {
			model = "google/gemini-2.5-flash"
		}
		defaultKeyEnv = "OPENROUTER_API_KEY"
	case "groq":
		if endpoint == "" {
			endpoint = "https://api.groq.com/openai/v1/chat/completions"
		}
		if model == "" {
			model = "llama-3.3-70b-versatile"
		}
		defaultKeyEnv = "GROQ_API_KEY"
	default:
		if endpoint == "" {
			endpoint = "https://api.openai.com/v1/chat/completions"
		}
		if model == "" {
			model = "gpt-4o-mini"
		}
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
	keyEnv := cfg.APIKeyEnv
	if keyEnv == "" {
		keyEnv = defaultKeyEnv
	}
	if keyEnv != "" {
		if key := os.Getenv(keyEnv); key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
	if strings.EqualFold(cfg.Provider, "openrouter") {
		req.Header.Set("HTTP-Referer", "https://memtwig.com")
		req.Header.Set("X-Title", "memtwig")
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
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		return nil, fmt.Errorf("parse LLM JSON: %w", err)
	}
	if !parsed.HasIssue {
		return nil, nil
	}
	return &SemanticFinding{
		NoteA:      cand.NoteA,
		NoteB:      cand.NoteB,
		Issue:      parsed.Issue,
		Suggestion: parsed.Suggestion,
	}, nil
}
