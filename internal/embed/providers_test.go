package embed_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/embed"
)

func TestOpenRouterAndDeepInfraProviders(t *testing.T) {
	var gotAuth, gotReferer, gotModel string
	var gotDims float64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		gotModel, _ = req["model"].(string)
		gotDims, _ = req["dimensions"].(float64)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{3, 4}}},
		})
	}))
	defer ts.Close()

	t.Setenv("OPENROUTER_API_KEY", "or-test")
	p := embed.New(config.EmbeddingsConfig{
		Provider:   "openrouter",
		Endpoint:   ts.URL,
		Model:      "openai/text-embedding-3-small",
		Dimensions: 1024,
		TimeoutMS:  5000,
	})
	vecs, err := p.Embed(context.Background(), []string{"hello"})
	if err != nil || len(vecs) != 1 {
		t.Fatalf("openrouter embed failed: err=%v vecs=%v", err, vecs)
	}
	if gotAuth != "Bearer or-test" || gotReferer != "https://memtwig.com" || gotModel != "openai/text-embedding-3-small" || gotDims != 1024 {
		t.Fatalf("unexpected openrouter request: auth=%q ref=%q model=%q dims=%v", gotAuth, gotReferer, gotModel, gotDims)
	}

	t.Setenv("DEEPINFRA_API_KEY", "di-test")
	p2 := embed.New(config.EmbeddingsConfig{
		Provider:   "deepinfra",
		Endpoint:   ts.URL,
		Dimensions: 1024,
		TimeoutMS:  5000,
	})
	vecs2, err := p2.Embed(context.Background(), []string{"hello"})
	if err != nil || len(vecs2) != 1 || p2.Name() != "deepinfra" {
		t.Fatalf("deepinfra embed failed: name=%s err=%v", p2.Name(), err)
	}
	if gotModel != "BAAI/bge-m3" {
		t.Fatalf("expected default deepinfra model BAAI/bge-m3, got %q", gotModel)
	}
}
