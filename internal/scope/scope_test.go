package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/memtwig/memtwig/internal/config"
)

func initGitRepoWithOrigin(t *testing.T, dir, originURL string) {
	t.Helper()
	cmds := [][]string{
		{"git", "-C", dir, "init"},
		{"git", "-C", dir, "remote", "add", "origin", originURL},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %v (%s)", args, err, string(out))
		}
	}
}

func makeTestConfig(workDir string) *config.Config {
	return &config.Config{
		Vaults: map[string]config.VaultConfig{
			"personal": {
				Path:          filepath.Join(workDir, "vault-personal"),
				Kind:          "personal",
				DefaultWrite:  "direct",
				AlwaysInclude: []string{"persona/**", "shared/*.md"},
			},
			"acme": {
				Path:            filepath.Join(workDir, "vault-acme"),
				Kind:            "org",
				DefaultWrite:    "propose",
				MatchPaths:      []string{filepath.Join(workDir, "acme/**")},
				MatchGitRemotes: []string{"github.com/acme/*"},
			},
			"beta": {
				Path:            filepath.Join(workDir, "vault-beta"),
				Kind:            "org",
				DefaultWrite:    "propose",
				MatchPaths:      []string{filepath.Join(workDir, "beta")},
				MatchGitRemotes: []string{"github.com/beta/*"},
			},
		},
	}
}

func TestResolveDefaultPersonal(t *testing.T) {
	tmp := t.TempDir()
	cfg := makeTestConfig(tmp)
	unrelatedDir := filepath.Join(tmp, "unrelated")
	if err := os.MkdirAll(unrelatedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	active := Resolve(cfg, unrelatedDir, "")
	if len(active) != 1 {
		t.Fatalf("expected 1 active vault, got %d: %+v", len(active), active)
	}
	if active[0].Name != "personal" || !active[0].FullAccess || active[0].MatchReason != "default_personal" {
		t.Fatalf("unexpected active vault: %+v", active[0])
	}
}

func TestResolveMatchPathsAndStrictIsolation(t *testing.T) {
	tmp := t.TempDir()
	cfg := makeTestConfig(tmp)
	acmeSub := filepath.Join(tmp, "acme", "service-a")
	if err := os.MkdirAll(acmeSub, 0o755); err != nil {
		t.Fatal(err)
	}

	active := Resolve(cfg, acmeSub, "")
	if len(active) != 2 {
		t.Fatalf("expected 2 active vaults (acme + personal always_include), got %d: %+v", len(active), active)
	}

	byName := map[string]ActiveVault{}
	for _, a := range active {
		byName[a.Name] = a
	}

	acme, ok := byName["acme"]
	if !ok || !acme.FullAccess {
		t.Fatalf("expected acme with FullAccess=true, got %+v", acme)
	}
	if _, ok := byName["beta"]; ok {
		t.Fatalf("beta org vault should be strictly isolated and not active")
	}

	pers, ok := byName["personal"]
	if !ok || pers.FullAccess || pers.MatchReason != "always_include" {
		t.Fatalf("expected personal with FullAccess=false and always_include, got %+v", pers)
	}
	if !pers.AllowsPath("persona/vittorio.md") {
		t.Errorf("expected persona/vittorio.md to be allowed by always_include")
	}
	if !pers.AllowsPath("shared/glossary.md") {
		t.Errorf("expected shared/glossary.md to be allowed by always_include")
	}
	if pers.AllowsPath("projects/secret.md") {
		t.Errorf("expected projects/secret.md to be denied when FullAccess=false")
	}
}

func TestResolveStrictIsolationWithoutAlwaysInclude(t *testing.T) {
	tmp := t.TempDir()
	cfg := makeTestConfig(tmp)
	p := cfg.Vaults["personal"]
	p.AlwaysInclude = nil
	cfg.Vaults["personal"] = p

	betaDir := filepath.Join(tmp, "beta")
	if err := os.MkdirAll(betaDir, 0o755); err != nil {
		t.Fatal(err)
	}

	active := Resolve(cfg, betaDir, "")
	if len(active) != 1 || active[0].Name != "beta" {
		t.Fatalf("expected only beta vault when personal has no always_include, got %+v", active)
	}
}

func TestResolveMatchGitRemotes(t *testing.T) {
	tmp := t.TempDir()
	cfg := makeTestConfig(tmp)

	repoDir := filepath.Join(tmp, "checkout-outside-match-paths")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatal(err)
	}
	initGitRepoWithOrigin(t, repoDir, "https://github.com/acme/backend.git")

	active := Resolve(cfg, repoDir, "")
	byName := map[string]ActiveVault{}
	for _, a := range active {
		byName[a.Name] = a
	}
	acme, ok := byName["acme"]
	if !ok || !acme.FullAccess || acme.MatchReason != "match_remote:github.com/acme/*" {
		t.Fatalf("expected acme matched by git remote, got %+v (all=%+v)", acme, active)
	}
	if _, ok := byName["beta"]; ok {
		t.Fatalf("beta should not match acme remote")
	}
}

func TestResolveExplicitVault(t *testing.T) {
	tmp := t.TempDir()
	cfg := makeTestConfig(tmp)
	unrelatedDir := filepath.Join(tmp, "unrelated")
	if err := os.MkdirAll(unrelatedDir, 0o755); err != nil {
		t.Fatal(err)
	}

	active := Resolve(cfg, unrelatedDir, "beta")
	if len(active) != 2 {
		t.Fatalf("expected 2 active vaults (explicit beta + personal always_include), got %d: %+v", len(active), active)
	}
	byName := map[string]ActiveVault{}
	for _, a := range active {
		byName[a.Name] = a
	}
	if byName["beta"].MatchReason != "explicit" || !byName["beta"].FullAccess {
		t.Errorf("expected explicit full access for beta, got %+v", byName["beta"])
	}
	if byName["personal"].FullAccess || byName["personal"].MatchReason != "always_include" {
		t.Errorf("expected always_include for personal, got %+v", byName["personal"])
	}
	if _, ok := byName["acme"]; ok {
		t.Errorf("acme should not be active when explicitVault=beta")
	}
}
