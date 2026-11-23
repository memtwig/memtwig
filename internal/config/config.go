package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const (
	DefaultConfigRelPath = ".config/memtwig/config.toml"
	DefaultDBRelPath     = ".cache/memtwig/index.db"
	DefaultCacheRelDir   = ".cache/memtwig"
)

type Config struct {
	Version    int                    `toml:"version"`
	Identity   IdentityConfig         `toml:"identity"`
	Embeddings EmbeddingsConfig       `toml:"embeddings"`
	Retrieval  RetrievalConfig        `toml:"retrieval"`
	LLM        LLMConfig              `toml:"llm"`
	Server     ServerConfig           `toml:"server"`
	Vaults     map[string]VaultConfig `toml:"vaults"`
}

type LLMConfig struct {
	Provider  string `toml:"provider"` // none | openai | ollama | openrouter | groq
	Model     string `toml:"model"`
	Endpoint  string `toml:"endpoint"`
	APIKeyEnv string `toml:"api_key_env"`
	TimeoutMS int    `toml:"timeout_ms"`
}

type ServerConfig struct {
	Addr         string        `toml:"addr"`
	AuditLogPath string        `toml:"audit_log_path"`
	Tokens       []TokenPolicy `toml:"tokens"`
}

type TokenPolicy struct {
	Name      string   `toml:"name"`
	TokenEnv  string   `toml:"token_env"`
	Token     string   `toml:"token,omitempty"`
	Role      string   `toml:"role"` // reader | proposer | writer | admin
	Vaults    []string `toml:"vaults"`
	PathGlobs []string `toml:"path_globs"`
}

type IdentityConfig struct {
	SoulFile             string `toml:"soul_file"`
	InjectOnSessionStart bool   `toml:"inject_on_session_start"`
}

type EmbeddingsConfig struct {
	Provider        string  `toml:"provider"` // none | ollama | openai | cloudflare | openrouter | deepinfra
	Model           string  `toml:"model"`
	Endpoint        string  `toml:"endpoint"`
	APIKeyEnv       string  `toml:"api_key_env"`
	AccountID       string  `toml:"account_id"`
	Dimensions      int     `toml:"dimensions"`
	TimeoutMS       int     `toml:"timeout_ms"`
	CosineThreshold float64 `toml:"cosine_threshold"`
}

type RetrievalConfig struct {
	DefaultTopK     int `toml:"default_top_k"`
	MaxSnippetChars int `toml:"max_snippet_chars"`
	RRFK            int `toml:"rrf_k"`
}

type VaultConfig struct {
	Path            string   `toml:"path"`
	Kind            string   `toml:"kind"`          // personal | org
	DefaultWrite    string   `toml:"default_write"` // direct | propose | readonly
	AlwaysInclude   []string `toml:"always_include"`
	MatchPaths      []string `toml:"match_paths"`
	MatchGitRemotes []string `toml:"match_git_remotes"`
	MaxNoteBytes    int      `toml:"max_note_bytes"`
}

func Default() *Config {
	return &Config{
		Version: 1,
		Identity: IdentityConfig{
			SoulFile:             "~/.config/memtwig/SOUL.md",
			InjectOnSessionStart: true,
		},
		Embeddings: EmbeddingsConfig{
			Provider:        "none",
			Model:           "bge-m3",
			Endpoint:        "http://127.0.0.1:11434/api/embeddings",
			Dimensions:      1024,
			TimeoutMS:       3000,
			CosineThreshold: 0.45,
		},
		Retrieval: RetrievalConfig{
			DefaultTopK:     5,
			MaxSnippetChars: 400,
			RRFK:            60,
		},
		Vaults: map[string]VaultConfig{},
	}
}

func ExpandHome(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

func DefaultConfigPath() (string, error) {
	if env := os.Getenv("MEMTWIG_CONFIG"); env != "" {
		return ExpandHome(env), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, DefaultConfigRelPath), nil
}

func DefaultDBPath() (string, error) {
	if env := os.Getenv("MEMTWIG_DB"); env != "" {
		return ExpandHome(env), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, DefaultDBRelPath), nil
}

func DefaultCacheDir() (string, error) {
	if env := os.Getenv("MEMTWIG_CACHE_DIR"); env != "" {
		return ExpandHome(env), nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, DefaultCacheRelDir), nil
}

func Load(path string) (*Config, error) {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return nil, err
		}
	}
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if cfg.Vaults == nil {
		cfg.Vaults = map[string]VaultConfig{}
	}
	applyDefaults(cfg)
	return cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if cfg.Retrieval.DefaultTopK <= 0 {
		cfg.Retrieval.DefaultTopK = 5
	}
	if cfg.Retrieval.MaxSnippetChars <= 0 {
		cfg.Retrieval.MaxSnippetChars = 400
	}
	if cfg.Retrieval.RRFK <= 0 {
		cfg.Retrieval.RRFK = 60
	}
	if cfg.Embeddings.Provider == "" {
		cfg.Embeddings.Provider = "none"
	}
	if cfg.Embeddings.Dimensions <= 0 {
		cfg.Embeddings.Dimensions = 1024
	}
	if cfg.Embeddings.TimeoutMS <= 0 {
		cfg.Embeddings.TimeoutMS = 3000
	}
	if cfg.Embeddings.CosineThreshold <= 0 {
		cfg.Embeddings.CosineThreshold = 0.45
	}
	if cfg.LLM.Provider == "" {
		cfg.LLM.Provider = "none"
	}
	if cfg.LLM.TimeoutMS <= 0 {
		cfg.LLM.TimeoutMS = 15000
	}
	if cfg.Server.Addr == "" {
		cfg.Server.Addr = "127.0.0.1:8787"
	}
	for name, v := range cfg.Vaults {
		if v.Kind == "" {
			v.Kind = "personal"
		}
		if v.DefaultWrite == "" {
			if v.Kind == "org" {
				v.DefaultWrite = "propose"
			} else {
				v.DefaultWrite = "direct"
			}
		}
		if v.MaxNoteBytes <= 0 {
			v.MaxNoteBytes = 8192
		}
		cfg.Vaults[name] = v
	}
}

func Save(path string, cfg *Config) error {
	if path == "" {
		var err error
		path, err = DefaultConfigPath()
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	enc := toml.NewEncoder(&buf)
	if err := enc.Encode(cfg); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
