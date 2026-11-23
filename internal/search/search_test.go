package search_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/scope"
	"github.com/memtwig/memtwig/internal/search"
)

func TestMultiOrgIsolationAndSearch(t *testing.T) {
	tmp := t.TempDir()
	personalDir := filepath.Join(tmp, "vault-personal")
	acmeDir := filepath.Join(tmp, "vault-acme")
	betaDir := filepath.Join(tmp, "vault-beta")
	acmeWork := filepath.Join(tmp, "work", "acme", "api")

	for _, d := range []string{
		filepath.Join(personalDir, "persona"),
		filepath.Join(personalDir, "projects"),
		filepath.Join(acmeDir, "projects"),
		filepath.Join(betaDir, "projects"),
		acmeWork,
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	write := func(p, content string) {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write(filepath.Join(personalDir, "persona", "style.md"), "---\nstatus: active\nupdated: 2026-11-23\n---\n# Style\nAlways keep deploy notes concise and lowercase.\n")
	write(filepath.Join(personalDir, "projects", "secret-personal.md"), "---\nstatus: active\nupdated: 2026-11-23\n---\n# Secret Personal\nDeploy pipeline for personal hobby.\n")
	write(filepath.Join(acmeDir, "projects", "infra.md"), "---\nstatus: active\nupdated: 2026-11-23\n---\n# Acme Infra\nAcme deploy runs on Fly.io in fra region.\n")
	write(filepath.Join(betaDir, "projects", "infra.md"), "---\nstatus: active\nupdated: 2026-11-23\n---\n# Beta Infra\nBeta deploy runs on AWS ECS with confidential billing.\n")

	cfg := config.Default()
	cfg.Vaults["personal"] = config.VaultConfig{
		Path:          personalDir,
		Kind:          "personal",
		DefaultWrite:  "direct",
		AlwaysInclude: []string{"persona/**"},
	}
	cfg.Vaults["acme"] = config.VaultConfig{
		Path:         acmeDir,
		Kind:         "org",
		DefaultWrite: "propose",
		MatchPaths:   []string{filepath.Join(tmp, "work", "acme") + "/**"},
	}
	cfg.Vaults["beta"] = config.VaultConfig{
		Path:         betaDir,
		Kind:         "org",
		DefaultWrite: "propose",
		MatchPaths:   []string{filepath.Join(tmp, "work", "beta") + "/**"},
	}

	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	stats, err := st.RebuildOrUpdate(context.Background(), cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Indexed != 4 {
		t.Fatalf("expected 4 indexed notes, got %d", stats.Indexed)
	}

	// Working inside acmeWork: must see acme + personal/persona/**, must NEVER see beta or personal/projects/*
	active := scope.Resolve(cfg, acmeWork, "")
	results, _, err := search.Query(context.Background(), st, cfg, active, "deploy", search.Options{TopK: 10})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range results {
		seen[r.Vault+":"+r.Path] = true
		if r.Vault == "beta" {
			t.Fatalf("isolation breach: saw beta note %s in acme workspace", r.Path)
		}
		if r.Vault == "personal" && r.Path == "projects/secret-personal.md" {
			t.Fatalf("isolation breach: saw non-always_include personal note %s in acme workspace", r.Path)
		}
	}
	if !seen["acme:projects/infra.md"] {
		t.Fatalf("expected acme:projects/infra.md in results, got %v", results)
	}
	if !seen["personal:persona/style.md"] {
		t.Fatalf("expected personal:persona/style.md (always_include) in results, got %v", results)
	}
}

func TestCourtesyAndCorrectionGate(t *testing.T) {
	if !search.IsCourtesy("ok grazie!") {
		t.Fatal("expected courtesy for 'ok grazie!'")
	}
	if search.IsCourtesy("controlla il deploy di acme") {
		t.Fatal("did not expect courtesy for technical query")
	}
	if !search.IsCorrection("no, ti avevo detto di usare go") {
		t.Fatal("expected correction detection")
	}
}

func TestPrefixSearchPostgresMatchesPostgreSQL(t *testing.T) {
	tmp := t.TempDir()
	vDir := filepath.Join(tmp, "acme")
	if err := os.MkdirAll(filepath.Join(vDir, "infra"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vDir, "infra", "db.md"), []byte("---\nstatus: active\nupdated: 2026-11-23\n---\n# Primary Database\nProduction runs PostgreSQL 17 behind PgBouncer.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Vaults["acme"] = config.VaultConfig{Path: vDir, Kind: "org", DefaultWrite: "propose"}
	st, err := index.Open(filepath.Join(tmp, "index.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RebuildOrUpdate(context.Background(), cfg, true); err != nil {
		t.Fatal(err)
	}
	active := scope.Resolve(cfg, tmp, "acme")
	results, _, err := search.Query(context.Background(), st, cfg, active, "Postgres", search.Options{TopK: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Path != "infra/db.md" {
		t.Fatalf("expected prefix match on PostgreSQL for query 'Postgres', got %+v", results)
	}
}
