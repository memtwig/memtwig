package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Version != 1 {
		t.Fatalf("expected Version=1, got %d", cfg.Version)
	}
	if cfg.Identity.SoulFile != "~/.config/memtwig/SOUL.md" || !cfg.Identity.InjectOnSessionStart {
		t.Fatalf("unexpected Identity defaults: %+v", cfg.Identity)
	}
	if cfg.Embeddings.Provider != "none" || cfg.Embeddings.Model != "bge-m3" || cfg.Embeddings.TimeoutMS != 3000 || cfg.Embeddings.CosineThreshold != 0.45 {
		t.Fatalf("unexpected Embeddings defaults: %+v", cfg.Embeddings)
	}
	if cfg.Retrieval.DefaultTopK != 5 || cfg.Retrieval.MaxSnippetChars != 400 || cfg.Retrieval.RRFK != 60 {
		t.Fatalf("unexpected Retrieval defaults: %+v", cfg.Retrieval)
	}
	if cfg.Vaults == nil || len(cfg.Vaults) != 0 {
		t.Fatalf("expected empty non-nil Vaults map, got %+v", cfg.Vaults)
	}
}

func TestExpandHome(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	tests := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"~", fakeHome},
		{"~/notes/index.md", filepath.Join(fakeHome, "notes/index.md")},
		{"/abs/path/file.toml", "/abs/path/file.toml"},
		{"relative/path", "relative/path"},
	}
	for _, tc := range tests {
		got := ExpandHome(tc.in)
		if got != tc.want {
			t.Errorf("ExpandHome(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestLoadMissingFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist.toml")
	cfg, err := Load(missing)
	if err != nil {
		t.Fatalf("Load missing file returned error: %v", err)
	}
	def := Default()
	if cfg.Version != def.Version || cfg.Retrieval != def.Retrieval || cfg.Embeddings != def.Embeddings {
		t.Fatalf("Load missing file did not return defaults: got %+v, want %+v", cfg, def)
	}
}

func TestSaveAndLoadRoundtrip(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "nested", "config.toml")

	orig := &Config{
		Version: 1,
		Identity: IdentityConfig{
			SoulFile:             "~/custom/SOUL.md",
			InjectOnSessionStart: false,
		},
		Embeddings: EmbeddingsConfig{
			Provider:        "openai",
			Model:           "text-embedding-3-small",
			Endpoint:        "https://api.example.test/v1/embeddings",
			APIKeyEnv:       "CUSTOM_OPENAI_KEY",
			AccountID:       "acc-123",
			Dimensions:      1024,
			TimeoutMS:       5000,
			CosineThreshold: 0.6,
		},
		Retrieval: RetrievalConfig{
			DefaultTopK:     8,
			MaxSnippetChars: 600,
			RRFK:            45,
		},
		LLM: LLMConfig{
			Provider:  "ollama",
			Model:     "llama3.2",
			Endpoint:  "http://127.0.0.1:11434/api/generate",
			APIKeyEnv: "LLM_KEY",
			TimeoutMS: 20000,
		},
		Server: ServerConfig{
			Addr:         "127.0.0.1:9999",
			AuditLogPath: filepath.Join(dir, "audit.jsonl"),
			Tokens: []TokenPolicy{
				{
					Name:      "ci-reader",
					TokenEnv:  "CI_TOKEN",
					Token:     "tok-secret",
					Role:      "reader",
					Vaults:    []string{"acme"},
					PathGlobs: []string{"projects/**"},
				},
			},
		},
		Vaults: map[string]VaultConfig{
			"personal": {
				Path:            "~/notes",
				Kind:            "personal",
				DefaultWrite:    "direct",
				AlwaysInclude:   []string{"persona/**"},
				MatchPaths:      []string{"~/personale/**"},
				MatchGitRemotes: []string{"github.com/user/*"},
				MaxNoteBytes:    16384,
			},
			"acme": {
				Path:            "~/work/acme-notes",
				Kind:            "org",
				MatchPaths:      []string{"~/work/acme/**"},
				MatchGitRemotes: []string{"github.com/acme/*"},
			},
		},
	}

	if err := Save(cfgPath, orig); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if _, err := os.Stat(cfgPath); err != nil {
		t.Fatalf("saved config file missing: %v", err)
	}

	loaded, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if loaded.Identity != orig.Identity {
		t.Errorf("Identity mismatch: got %+v, want %+v", loaded.Identity, orig.Identity)
	}
	if loaded.Embeddings != orig.Embeddings {
		t.Errorf("Embeddings mismatch: got %+v, want %+v", loaded.Embeddings, orig.Embeddings)
	}
	if loaded.Retrieval != orig.Retrieval {
		t.Errorf("Retrieval mismatch: got %+v, want %+v", loaded.Retrieval, orig.Retrieval)
	}
	if loaded.LLM != orig.LLM {
		t.Errorf("LLM mismatch: got %+v, want %+v", loaded.LLM, orig.LLM)
	}
	if loaded.Server.Addr != orig.Server.Addr || loaded.Server.AuditLogPath != orig.Server.AuditLogPath {
		t.Errorf("Server mismatch: got %+v, want %+v", loaded.Server, orig.Server)
	}
	if len(loaded.Server.Tokens) != 1 || loaded.Server.Tokens[0].Name != "ci-reader" || loaded.Server.Tokens[0].Role != "reader" || loaded.Server.Tokens[0].Token != "tok-secret" {
		t.Errorf("Server.Tokens mismatch: got %+v", loaded.Server.Tokens)
	}
	if len(loaded.Vaults) != 2 {
		t.Fatalf("expected 2 vaults, got %d", len(loaded.Vaults))
	}

	p := loaded.Vaults["personal"]
	if p.Path != "~/notes" || p.Kind != "personal" || p.DefaultWrite != "direct" || p.MaxNoteBytes != 16384 {
		t.Errorf("personal vault mismatch: %+v", p)
	}
	if len(p.AlwaysInclude) != 1 || p.AlwaysInclude[0] != "persona/**" {
		t.Errorf("personal AlwaysInclude mismatch: %+v", p.AlwaysInclude)
	}

	// Org vault had DefaultWrite and MaxNoteBytes omitted -> applyDefaults should fill them
	org := loaded.Vaults["acme"]
	if org.Kind != "org" || org.DefaultWrite != "propose" || org.MaxNoteBytes != 8192 {
		t.Errorf("org vault defaults mismatch: %+v", org)
	}
}

func TestXDGAndEnvPrecedence(t *testing.T) {
	fakeHome := t.TempDir()
	xdgCfg := filepath.Join(fakeHome, "xdg-cfg")
	xdgCache := filepath.Join(fakeHome, "xdg-cache")

	t.Setenv("HOME", fakeHome)
	t.Setenv("MEMTWIG_CONFIG", "")
	t.Setenv("MEMTWIG_CACHE_DIR", "")
	t.Setenv("MEMTWIG_DB", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("XDG_CACHE_HOME", "")

	// Default fallback without XDG or MEMTWIG_*
	if got, err := DefaultConfigPath(); err != nil || got != filepath.Join(fakeHome, DefaultConfigRelPath) {
		t.Fatalf("fallback DefaultConfigPath = (%q, %v), want %q", got, err, filepath.Join(fakeHome, DefaultConfigRelPath))
	}
	if got, err := DefaultCacheDir(); err != nil || got != filepath.Join(fakeHome, DefaultCacheRelDir) {
		t.Fatalf("fallback DefaultCacheDir = (%q, %v), want %q", got, err, filepath.Join(fakeHome, DefaultCacheRelDir))
	}
	if got, err := DefaultDBPath(); err != nil || got != filepath.Join(fakeHome, DefaultDBRelPath) {
		t.Fatalf("fallback DefaultDBPath = (%q, %v), want %q", got, err, filepath.Join(fakeHome, DefaultDBRelPath))
	}

	// XDG_CONFIG_HOME and XDG_CACHE_HOME honored
	t.Setenv("XDG_CONFIG_HOME", xdgCfg)
	t.Setenv("XDG_CACHE_HOME", xdgCache)
	if got, err := DefaultConfigPath(); err != nil || got != filepath.Join(xdgCfg, "memtwig", "config.toml") {
		t.Fatalf("XDG DefaultConfigPath = (%q, %v)", got, err)
	}
	if got, err := DefaultCacheDir(); err != nil || got != filepath.Join(xdgCache, "memtwig") {
		t.Fatalf("XDG DefaultCacheDir = (%q, %v)", got, err)
	}
	if got, err := DefaultDBPath(); err != nil || got != filepath.Join(xdgCache, "memtwig", "index.db") {
		t.Fatalf("XDG DefaultDBPath = (%q, %v)", got, err)
	}

	// MEMTWIG_* overrides XDG_*
	t.Setenv("MEMTWIG_CONFIG", "~/override/config.toml")
	t.Setenv("MEMTWIG_CACHE_DIR", "~/override/cache")
	if got, err := DefaultConfigPath(); err != nil || got != filepath.Join(fakeHome, "override", "config.toml") {
		t.Fatalf("MEMTWIG_CONFIG override = (%q, %v)", got, err)
	}
	if got, err := DefaultCacheDir(); err != nil || got != filepath.Join(fakeHome, "override", "cache") {
		t.Fatalf("MEMTWIG_CACHE_DIR override = (%q, %v)", got, err)
	}
	if got, err := DefaultDBPath(); err != nil || got != filepath.Join(fakeHome, "override", "cache", "index.db") {
		t.Fatalf("MEMTWIG_CACHE_DIR -> DefaultDBPath = (%q, %v)", got, err)
	}

	// MEMTWIG_DB overrides MEMTWIG_CACHE_DIR
	t.Setenv("MEMTWIG_DB", "~/custom/index.db")
	if got, err := DefaultDBPath(); err != nil || got != filepath.Join(fakeHome, "custom", "index.db") {
		t.Fatalf("MEMTWIG_DB override = (%q, %v)", got, err)
	}
}
