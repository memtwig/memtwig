package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/memtwig/memtwig/internal/config"
)

type Provider interface {
	Name() string
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

func New(cfg config.EmbeddingsConfig) Provider {
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "none":
		return noneProvider{}
	case "ollama":
		return &ollamaProvider{
			endpoint: cfg.Endpoint,
			model:    cfg.Model,
			client:   &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
		}
	case "openai":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "https://api.openai.com/v1/embeddings"
		}
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "OPENAI_API_KEY"
		}
		model := cfg.Model
		if model == "" {
			model = "text-embedding-3-small"
		}
		return &openaiProvider{
			name:       "openai",
			endpoint:   ep,
			model:      model,
			apiKey:     os.Getenv(keyEnv),
			dimensions: cfg.Dimensions,
			client:     &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
		}
	case "openrouter":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "https://openrouter.ai/api/v1/embeddings"
		}
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "OPENROUTER_API_KEY"
		}
		model := cfg.Model
		if model == "" {
			model = "baai/bge-m3"
		}
		return &openaiProvider{
			name:       "openrouter",
			endpoint:   ep,
			model:      model,
			apiKey:     os.Getenv(keyEnv),
			dimensions: cfg.Dimensions,
			client:     &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
		}
	case "deepinfra":
		ep := cfg.Endpoint
		if ep == "" {
			ep = "https://api.deepinfra.com/v1/openai/embeddings"
		}
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "DEEPINFRA_API_KEY"
		}
		model := cfg.Model
		if model == "" {
			model = "BAAI/bge-m3"
		}
		return &openaiProvider{
			name:       "deepinfra",
			endpoint:   ep,
			model:      model,
			apiKey:     os.Getenv(keyEnv),
			dimensions: cfg.Dimensions,
			client:     &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
		}
	case "cloudflare":
		keyEnv := cfg.APIKeyEnv
		if keyEnv == "" {
			keyEnv = "CLOUDFLARE_API_TOKEN"
		}
		ep := cfg.Endpoint
		if ep == "" && cfg.AccountID != "" {
			model := cfg.Model
			if model == "" {
				model = "@cf/baai/bge-m3"
			}
			ep = fmt.Sprintf("https://api.cloudflare.com/client/v4/accounts/%s/ai/run/%s", cfg.AccountID, model)
		}
		return &cloudflareProvider{
			endpoint: ep,
			apiKey:   os.Getenv(keyEnv),
			client:   &http.Client{Timeout: time.Duration(cfg.TimeoutMS) * time.Millisecond},
		}
	default:
		return noneProvider{}
	}
}

type noneProvider struct{}

func (noneProvider) Name() string { return "none" }
func (noneProvider) Embed(context.Context, []string) ([][]float32, error) {
	return nil, errors.New("embeddings provider disabled")
}

type ollamaProvider struct {
	endpoint string
	model    string
	client   *http.Client
}

func (p *ollamaProvider) Name() string { return "ollama" }

func (p *ollamaProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for _, t := range texts {
		payload, _ := json.Marshal(map[string]string{"model": p.model, "prompt": t})
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 300 {
			return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(body))
		}
		var res struct {
			Embedding []float32 `json:"embedding"`
		}
		if err := json.Unmarshal(body, &res); err != nil {
			return nil, err
		}
		out = append(out, Normalize(res.Embedding))
	}
	return out, nil
}

type openaiProvider struct {
	name       string
	endpoint   string
	model      string
	apiKey     string
	dimensions int
	client     *http.Client
}

func (p *openaiProvider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "openai"
}

func (p *openaiProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("missing API key for %s embeddings", p.Name())
	}
	bodyMap := map[string]any{"model": p.model, "input": texts}
	// Pass Matryoshka dimensions parameter when requested for OpenAI text-embedding-3-* models
	if p.dimensions > 0 && strings.Contains(strings.ToLower(p.model), "text-embedding-3") {
		bodyMap["dimensions"] = p.dimensions
	}
	payload, _ := json.Marshal(bodyMap)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	if p.name == "openrouter" {
		req.Header.Set("HTTP-Referer", "https://memtwig.com")
		req.Header.Set("X-Title", "memtwig")
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openai HTTP %d: %s", resp.StatusCode, string(body))
	}
	var res struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	out := make([][]float32, 0, len(res.Data))
	for _, d := range res.Data {
		out = append(out, Normalize(d.Embedding))
	}
	return out, nil
}

type cloudflareProvider struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

func (p *cloudflareProvider) Name() string { return "cloudflare" }

func (p *cloudflareProvider) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if p.apiKey == "" || p.endpoint == "" {
		return nil, errors.New("missing Cloudflare API token or endpoint")
	}
	payload, _ := json.Marshal(map[string]any{"text": texts})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudflare HTTP %d: %s", resp.StatusCode, string(body))
	}
	var res struct {
		Result struct {
			Data     [][]float32 `json:"data"`
			Response [][]float32 `json:"response"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	vecs := res.Result.Data
	if len(vecs) == 0 {
		vecs = res.Result.Response
	}
	out := make([][]float32, 0, len(vecs))
	for _, v := range vecs {
		out = append(out, Normalize(v))
	}
	return out, nil
}

// Normalize returns an L2-normalized copy of v.
func Normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	norm := float32(math.Sqrt(sum))
	if norm == 0 {
		norm = 1
	}
	out := make([]float32, len(v))
	for i, x := range v {
		out[i] = x / norm
	}
	return out
}

// Cosine computes dot product assuming both vectors are L2-normalized.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot
}
