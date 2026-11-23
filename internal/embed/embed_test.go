package embed

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
)

func TestNormalizeAndCosine(t *testing.T) {
	v := Normalize([]float32{3, 4})
	if math.Abs(float64(v[0]-0.6)) > 1e-5 || math.Abs(float64(v[1]-0.8)) > 1e-5 {
		t.Fatalf("Normalize([3,4]) = %v, want [0.6, 0.8]", v)
	}

	zero := Normalize([]float32{0, 0})
	if zero[0] != 0 || zero[1] != 0 {
		t.Fatalf("Normalize([0,0]) = %v, want [0,0]", zero)
	}

	if got := Cosine(v, v); math.Abs(got-1.0) > 1e-5 {
		t.Errorf("Cosine(v, v) = %v, want 1.0", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{0, 1}); math.Abs(got) > 1e-6 {
		t.Errorf("Cosine(ortho) = %v, want 0", got)
	}
	if got := Cosine([]float32{1, 0}, []float32{-1, 0}); math.Abs(got+1.0) > 1e-6 {
		t.Errorf("Cosine(opposite) = %v, want -1", got)
	}
	if got := Cosine(nil, nil); got != 0 {
		t.Errorf("Cosine(nil, nil) = %v, want 0", got)
	}
	if got := Cosine([]float32{1}, []float32{1, 2}); got != 0 {
		t.Errorf("Cosine(mismatched) = %v, want 0", got)
	}
}

func TestNoneProvider(t *testing.T) {
	p := New(config.EmbeddingsConfig{Provider: "none"})
	if p.Name() != "none" {
		t.Fatalf("Name() = %q, want none", p.Name())
	}
	if _, err := p.Embed(context.Background(), []string{"hello"}); err == nil {
		t.Fatal("expected error from noneProvider.Embed")
	}
}

func TestOllamaProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var req map[string]string
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req["model"] != "bge-m3" || req["prompt"] == "" {
			t.Errorf("unexpected ollama payload: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"embedding": []float32{3, 4},
		})
	}))
	defer srv.Close()

	p := New(config.EmbeddingsConfig{
		Provider:  "ollama",
		Model:     "bge-m3",
		Endpoint:  srv.URL,
		TimeoutMS: 2000,
	})
	if p.Name() != "ollama" {
		t.Fatalf("Name() = %q, want ollama", p.Name())
	}
	vecs, err := p.Embed(context.Background(), []string{"hello", "world"})
	if err != nil {
		t.Fatalf("Ollama Embed failed: %v", err)
	}
	if len(vecs) != 2 || math.Abs(float64(vecs[0][0]-0.6)) > 1e-5 {
		t.Fatalf("unexpected ollama vecs: %+v", vecs)
	}
}

func TestOpenAIProvider(t *testing.T) {
	t.Setenv("TEST_OPENAI_KEY", "sk-test-token")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test-token" {
			t.Errorf("Authorization header = %q, want Bearer sk-test-token", got)
		}
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "text-embedding-3-small" || len(req.Input) != 2 {
			t.Errorf("unexpected openai payload: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"embedding": []float32{0, 5}},
				{"embedding": []float32{3, 4}},
			},
		})
	}))
	defer srv.Close()

	p := New(config.EmbeddingsConfig{
		Provider:  "openai",
		Model:     "text-embedding-3-small",
		Endpoint:  srv.URL,
		APIKeyEnv: "TEST_OPENAI_KEY",
		TimeoutMS: 2000,
	})
	if p.Name() != "openai" {
		t.Fatalf("Name() = %q, want openai", p.Name())
	}
	vecs, err := p.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("OpenAI Embed failed: %v", err)
	}
	if len(vecs) != 2 || math.Abs(float64(vecs[0][1]-1.0)) > 1e-5 || math.Abs(float64(vecs[1][0]-0.6)) > 1e-5 {
		t.Fatalf("unexpected openai vecs: %+v", vecs)
	}
}

func TestCloudflareProvider(t *testing.T) {
	t.Setenv("TEST_CF_TOKEN", "cf-secret")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer cf-secret" {
			t.Errorf("Authorization header = %q, want Bearer cf-secret", got)
		}
		var req struct {
			Text []string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if len(req.Text) != 1 || req.Text[0] != "ciao" {
			t.Errorf("unexpected cloudflare payload: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"data": [][]float32{{6, 8}},
			},
		})
	}))
	defer srv.Close()

	p := New(config.EmbeddingsConfig{
		Provider:  "cloudflare",
		Endpoint:  srv.URL,
		APIKeyEnv: "TEST_CF_TOKEN",
		TimeoutMS: 2000,
	})
	if p.Name() != "cloudflare" {
		t.Fatalf("Name() = %q, want cloudflare", p.Name())
	}
	vecs, err := p.Embed(context.Background(), []string{"ciao"})
	if err != nil {
		t.Fatalf("Cloudflare Embed failed: %v", err)
	}
	if len(vecs) != 1 || math.Abs(float64(vecs[0][0]-0.6)) > 1e-5 || math.Abs(float64(vecs[0][1]-0.8)) > 1e-5 {
		t.Fatalf("unexpected cloudflare vecs: %+v", vecs)
	}
}
