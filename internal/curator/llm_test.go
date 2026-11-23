package curator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/curator"
	"github.com/memtwig/memtwig/internal/proposal"
)

func TestCurateSemanticMockLLM(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "projects"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "projects", "acme-deploy.md"), []byte("---\nstatus: active\nupdated: 2026-11-20\n---\n# Acme Deploy\nRuns on Heroku.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "projects", "acme-infra.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Acme Infra\nMigrated to Fly.io.\n"), 0o644)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]string{
						"content": `{"has_issue": true, "issue": "Heroku vs Fly.io conflict", "suggestion": "Invalidate Heroku in acme-deploy.md"}`,
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	rep, err := curator.CurateSemantic(context.Background(), "acme", tmp, config.LLMConfig{
		Provider:  "openai",
		Endpoint:  ts.URL,
		Model:     "test-model",
		TimeoutMS: 5000,
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if rep.PairsExamined != 1 || len(rep.Findings) != 1 || rep.ProposalsCreated != 1 {
		t.Fatalf("unexpected semantic report: %+v", rep)
	}
	props, err := proposal.List(tmp, false)
	if err != nil || len(props) != 1 {
		t.Fatalf("expected 1 review proposal, got %d (err=%v)", len(props), err)
	}
}
