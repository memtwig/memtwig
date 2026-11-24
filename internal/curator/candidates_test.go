package curator_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/curator"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/vault"
)

func insertTestDocWithChunks(t *testing.T, st *index.Store, vName, root, rel string, chunks []struct {
	heading string
	content string
	vec     []float32
}) {
	t.Helper()
	n, err := vault.ParseNote(vName, root, rel)
	if err != nil {
		t.Fatal(err)
	}
	res, err := st.DB.Exec(
		`INSERT INTO documents(vault_id, rel_path, sha256, title, status, owner, updated_at, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		vName, n.RelPath, n.SHA256, n.Title, n.Status, n.Owner, n.Updated, n.SizeBytes,
	)
	if err != nil {
		t.Fatal(err)
	}
	docID, _ := res.LastInsertId()
	for _, ch := range chunks {
		if _, err := st.DB.Exec(
			`INSERT INTO chunks(doc_id, vault_id, rel_path, title, heading, content, embedding)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			docID, vName, n.RelPath, n.Title, ch.heading, ch.content, index.EncodeFloat32s(ch.vec),
		); err != nil {
			t.Fatal(err)
		}
	}
}

func TestFindCandidatePairsVectorAndRotation(t *testing.T) {
	tmp := t.TempDir()
	dbFile := filepath.Join(tmp, "index.db")

	_ = os.MkdirAll(filepath.Join(tmp, "infra"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "projects"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "projects", "archive"), 0o755)
	_ = os.MkdirAll(filepath.Join(tmp, "decisions"), 0o755)

	writeNote := func(rel, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(tmp, filepath.FromSlash(rel)), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	writeNote("infra/host.md", "status: active\nupdated: 2026-11-20\n# Host\nRuns on server A.\n")
	writeNote("projects/app.md", "status: active\nupdated: 2026-11-22\n# App\nHosted on server B.\n")
	writeNote("projects/unrelated.md", "status: active\nupdated: 2026-11-22\n# Unrelated\nCooking recipe.\n")
	// Exclusions (even if populated in SQLite)
	writeNote("projects/closed-note.md", "status: closed\nupdated: 2026-11-22\n# Closed\nShould be ignored.\n")
	writeNote("decisions/2026-11.md", "# Decisions 2026-11\n- 2026-11-20 decision entry\n")
	writeNote("index.md", "# Index\nGenerated.\n")
	writeNote("projects/archive/old.md", "status: active\n# Old\nArchived.\n")

	st, err := index.Open(dbFile)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	type chunkSpec = struct {
		heading string
		content string
		vec     []float32
	}
	// Two similar notes across different top-level folders (infra/ vs projects/)
	insertTestDocWithChunks(t, st, "personal", tmp, "infra/host.md", []chunkSpec{
		{heading: "Deploy", content: "Chunk 0 runs on server A.", vec: []float32{0.99, 0.14}},
		{heading: "Logs", content: "Chunk 1 logs at /var/log/a.", vec: []float32{0.98, 0.20}},
	})
	insertTestDocWithChunks(t, st, "personal", tmp, "projects/app.md", []chunkSpec{
		{heading: "Runtime", content: "Chunk 0 hosted on server B.", vec: []float32{0.99, 0.12}},
		{heading: "DNS", content: "Chunk 1 points to server B IP.", vec: []float32{0.96, 0.28}},
	})
	// Orthogonal note
	insertTestDocWithChunks(t, st, "personal", tmp, "projects/unrelated.md", []chunkSpec{
		{heading: "Food", content: "Pasta recipe.", vec: []float32{-0.1, 0.99}},
	})
	// Closed note with identical vector must still be excluded
	insertTestDocWithChunks(t, st, "personal", tmp, "projects/closed-note.md", []chunkSpec{
		{heading: "Closed", content: "Closed fact.", vec: []float32{1.0, 0.0}},
	})

	// 1. Vector selection surfaces infra/host.md <-> projects/app.md across directories
	pairs, err := curator.FindCandidatePairs("personal", tmp, curator.CandidateOptions{
		DBPath:    dbFile,
		MaxPairs:  6,
		MinCosine: 0.55,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("expected 1 vector candidate pair, got %d: %+v", len(pairs), pairs)
	}
	p := pairs[0]
	if p.Method != "vector" || p.Score <= 0.9 {
		t.Fatalf("expected vector score > 0.9, got method=%s score=%v", p.Method, p.Score)
	}
	if p.NoteA != "infra/host.md" || p.NoteB != "projects/app.md" {
		t.Fatalf("unexpected pair paths: %s <-> %s", p.NoteA, p.NoteB)
	}
	if !strings.Contains(p.ExcerptA, "Runs on server A") && !strings.Contains(p.ExcerptA, "Chunk 0") {
		t.Fatalf("expected excerpt from chunks, got: %q", p.ExcerptA)
	}

	// 2. Mark judged -> second call skips freshly judged pair
	if err := curator.MarkPairsJudged(tmp, pairs); err != nil {
		t.Fatal(err)
	}
	pairsAfter, err := curator.FindCandidatePairs("personal", tmp, curator.CandidateOptions{
		DBPath:   dbFile,
		MaxPairs: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairsAfter) != 0 {
		t.Fatalf("expected 0 pairs after MarkPairsJudged, got %+v", pairsAfter)
	}

	// 3. IgnoreJudged returns it again even unchanged
	pairsAll, err := curator.FindCandidatePairs("personal", tmp, curator.CandidateOptions{
		DBPath:       dbFile,
		MaxPairs:     6,
		IgnoreJudged: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairsAll) != 1 {
		t.Fatalf("expected 1 pair with IgnoreJudged, got %d", len(pairsAll))
	}

	// 4. Editing one note changes its SHA256 -> pair becomes eligible again automatically
	writeNote("projects/app.md", "status: active\nupdated: 2026-11-24\n# App\nNow hosted on server C.\n")
	pairsEdited, err := curator.FindCandidatePairs("personal", tmp, curator.CandidateOptions{
		DBPath:   dbFile,
		MaxPairs: 6,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pairsEdited) != 1 {
		t.Fatalf("expected edited pair to become eligible again, got %d", len(pairsEdited))
	}
}

func TestCurateSemanticRotationAndKeywordFallback(t *testing.T) {
	tmp := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tmp, "projects"), 0o755)
	_ = os.WriteFile(filepath.Join(tmp, "projects", "acme-deploy.md"), []byte("status: active\n# Acme Deploy\nHeroku.\n"), 0o644)
	_ = os.WriteFile(filepath.Join(tmp, "projects", "acme-infra.md"), []byte("status: active\n# Acme Infra\nFly.io.\n"), 0o644)

	calls := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"has_issue": false}`}},
			},
		})
	}))
	defer ts.Close()

	llmCfg := config.LLMConfig{
		Provider:  "openai",
		Endpoint:  ts.URL,
		Model:     "test",
		TimeoutMS: 5000,
	}
	rep1, err := curator.CurateSemanticWithOptions(context.Background(), "acme", tmp, llmCfg, curator.CandidateOptions{MaxPairs: 4})
	if err != nil {
		t.Fatal(err)
	}
	if rep1.PairsExamined != 1 || calls != 1 {
		t.Fatalf("expected 1 examined pair & 1 LLM call, got rep=%+v calls=%d", rep1, calls)
	}

	// Second run must skip already-judged pair
	rep2, err := curator.CurateSemanticWithOptions(context.Background(), "acme", tmp, llmCfg, curator.CandidateOptions{MaxPairs: 4})
	if err != nil {
		t.Fatal(err)
	}
	if rep2.PairsExamined != 0 || calls != 1 {
		t.Fatalf("expected 0 pairs examined on second run, got rep=%+v calls=%d", rep2, calls)
	}
}
