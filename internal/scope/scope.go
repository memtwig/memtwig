package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/memtwig/memtwig/internal/config"
)

// ActiveVault represents a vault visible in the current session scope.
type ActiveVault struct {
	Name        string
	Config      config.VaultConfig
	FullAccess  bool     // true if entire vault is readable
	PathGlobs   []string // non-empty when FullAccess is false (e.g. always_include)
	MatchReason string
}

// Resolve determines which vaults are visible for a given working directory.
// If explicitVault is non-empty (from --vault or MEMTWIG_VAULT), only that vault is returned
// plus any personal vault's always_include globs.
func Resolve(cfg *config.Config, cwd string, explicitVault string) []ActiveVault {
	if explicitVault == "" {
		explicitVault = os.Getenv("MEMTWIG_VAULT")
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	absCwd, err := filepath.Abs(config.ExpandHome(cwd))
	if err == nil {
		cwd = absCwd
	}
	gitRemote := detectGitRemote(cwd)

	names := make([]string, 0, len(cfg.Vaults))
	for name := range cfg.Vaults {
		names = append(names, name)
	}
	sort.Strings(names)

	var active []ActiveVault
	var matchedOrg bool

	if explicitVault != "" {
		for _, name := range names {
			v := cfg.Vaults[name]
			if name == explicitVault {
				active = append(active, ActiveVault{
					Name:        name,
					Config:      v,
					FullAccess:  true,
					MatchReason: "explicit",
				})
				continue
			}
			if v.Kind == "personal" && len(v.AlwaysInclude) > 0 {
				active = append(active, ActiveVault{
					Name:        name,
					Config:      v,
					FullAccess:  false,
					PathGlobs:   v.AlwaysInclude,
					MatchReason: "always_include",
				})
			}
		}
		return active
	}

	// First pass: check org vaults matching cwd or git remote.
	orgMatched := map[string]string{}
	for _, name := range names {
		v := cfg.Vaults[name]
		if v.Kind != "org" {
			continue
		}
		if reason := matchVault(v, cwd, gitRemote); reason != "" {
			orgMatched[name] = reason
			matchedOrg = true
		}
	}

	for _, name := range names {
		v := cfg.Vaults[name]
		if v.Kind == "org" {
			if reason, ok := orgMatched[name]; ok {
				active = append(active, ActiveVault{
					Name:        name,
					Config:      v,
					FullAccess:  true,
					MatchReason: reason,
				})
			}
			continue
		}
		// Personal vault:
		// If an org vault matched, expose only always_include (if configured) to keep org sessions focused,
		// unless the personal vault itself explicitly matches cwd.
		if matchedOrg {
			if reason := matchVault(v, cwd, gitRemote); reason != "" {
				active = append(active, ActiveVault{
					Name:        name,
					Config:      v,
					FullAccess:  true,
					MatchReason: reason,
				})
			} else if len(v.AlwaysInclude) > 0 {
				active = append(active, ActiveVault{
					Name:        name,
					Config:      v,
					FullAccess:  false,
					PathGlobs:   v.AlwaysInclude,
					MatchReason: "always_include",
				})
			}
		} else {
			active = append(active, ActiveVault{
				Name:        name,
				Config:      v,
				FullAccess:  true,
				MatchReason: "default_personal",
			})
		}
	}
	return active
}

// AllowsPath reports whether relPath (slash-separated) inside this ActiveVault is readable.
func (a ActiveVault) AllowsPath(relPath string) bool {
	if a.FullAccess {
		return true
	}
	relPath = filepath.ToSlash(relPath)
	for _, pat := range a.PathGlobs {
		if matchGlob(pat, relPath) {
			return true
		}
	}
	return false
}

func matchVault(v config.VaultConfig, cwd, gitRemote string) string {
	for _, p := range v.MatchPaths {
		expanded := config.ExpandHome(p)
		if matchPathPrefixOrGlob(expanded, cwd) {
			return "match_path:" + p
		}
	}
	if gitRemote != "" {
		normRemote := normalizeGitRemote(gitRemote)
		for _, r := range v.MatchGitRemotes {
			if matchGlob(strings.ToLower(r), strings.ToLower(normRemote)) {
				return "match_remote:" + r
			}
		}
	}
	return ""
}

func matchPathPrefixOrGlob(pattern, target string) bool {
	pattern = filepath.Clean(pattern)
	target = filepath.Clean(target)
	if strings.HasSuffix(pattern, "/**") {
		base := strings.TrimSuffix(pattern, "/**")
		return target == base || strings.HasPrefix(target, base+string(os.PathSeparator))
	}
	if target == pattern || strings.HasPrefix(target, pattern+string(os.PathSeparator)) {
		return true
	}
	ok, _ := filepath.Match(pattern, target)
	return ok
}

// matchGlob supports simple ** prefix/suffix patterns and filepath.Match.
func matchGlob(pattern, target string) bool {
	pattern = filepath.ToSlash(pattern)
	target = filepath.ToSlash(target)
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "/**")
		return target == prefix || strings.HasPrefix(target, prefix+"/")
	}
	if strings.HasSuffix(pattern, "/*") {
		prefix := strings.TrimSuffix(pattern, "/*")
		return strings.HasPrefix(target, prefix+"/")
	}
	ok, _ := filepath.Match(pattern, target)
	return ok
}

func detectGitRemote(cwd string) string {
	cmd := exec.Command("git", "-C", cwd, "config", "--get", "remote.origin.url")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func normalizeGitRemote(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "ssh://")
	if strings.HasPrefix(s, "git@") {
		s = strings.TrimPrefix(s, "git@")
		s = strings.Replace(s, ":", "/", 1)
	}
	return s
}
