package config

import (
	"os"
	"regexp"
	"sort"
	"strings"
)

var reSecretKeyName = regexp.MustCompile(`(?i)key|token|secret|pass|auth|cred|private|sid|bearer|hmac|salt`)
var reHasAlnum = regexp.MustCompile(`[A-Za-z0-9]`)

// LoadEnvFiles reads KEY=VALUE files (e.g. ~/.hermes/.env) and sets process environment variables
// only when not already present in the environment. Missing files are silently ignored.
func LoadEnvFiles(paths []string) {
	for _, rawPath := range paths {
		p := ExpandHome(strings.TrimSpace(rawPath))
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := parseKVLine(line)
			if !ok {
				continue
			}
			if _, exists := os.LookupEnv(k); !exists {
				_ = os.Setenv(k, v)
			}
		}
	}
}

// ExtractSecretLiterals returns deduplicated sorted secret values (>= 10 chars, non-boolean,
// matching sensitive key names) found in the given .env or simple .toml files.
func ExtractSecretLiterals(paths []string) []string {
	set := map[string]bool{}
	for _, rawPath := range paths {
		p := ExpandHome(strings.TrimSpace(rawPath))
		if p == "" {
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			k, v, ok := parseKVLine(line)
			if !ok || !reSecretKeyName.MatchString(k) {
				continue
			}
			if len(v) < 10 || strings.EqualFold(v, "true") || strings.EqualFold(v, "false") {
				continue
			}
			if !reHasAlnum.MatchString(v) {
				continue
			}
			set[v] = true
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func parseKVLine(line string) (string, string, bool) {
	s := strings.TrimSpace(line)
	if s == "" || strings.HasPrefix(s, "#") || strings.HasPrefix(s, "[") {
		return "", "", false
	}
	if strings.HasPrefix(s, "export ") {
		s = strings.TrimSpace(strings.TrimPrefix(s, "export "))
	}
	eq := strings.IndexByte(s, '=')
	if eq <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(s[:eq])
	if key == "" || strings.ContainsAny(key, " \t") {
		return "", "", false
	}
	val := strings.TrimSpace(s[eq+1:])
	if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') {
		q := val[0]
		if end := strings.IndexByte(val[1:], q); end >= 0 {
			val = val[1 : 1+end]
		} else {
			val = strings.Trim(val, `"'`)
		}
	} else {
		val = strings.Trim(val, `"'`)
	}
	return key, val, true
}
