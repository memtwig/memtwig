package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/embed"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/lint"
)

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail
	Detail string `json:"detail"`
}

func Run(ctx context.Context, cfgPath, dbPath string) []Check {
	var out []Check

	// 1. Config
	cfg, err := config.Load(cfgPath)
	if err != nil {
		out = append(out, Check{Name: "config", Status: "fail", Detail: err.Error()})
		return out
	}
	out = append(out, Check{Name: "config", Status: "ok", Detail: fmt.Sprintf("%d vault(s) configured", len(cfg.Vaults))})

	// 2. Git binary
	if ver, err := exec.Command("git", "--version").Output(); err != nil {
		out = append(out, Check{Name: "git", Status: "warn", Detail: "git binary not found in PATH"})
	} else {
		out = append(out, Check{Name: "git", Status: "ok", Detail: strings.TrimSpace(string(ver))})
	}

	// 3. SQLite + FTS5
	st, err := index.Open(dbPath)
	if err != nil {
		out = append(out, Check{Name: "sqlite_fts5", Status: "fail", Detail: err.Error()})
	} else {
		var docs, chunks int
		_ = st.DB.QueryRow(`SELECT COUNT(*) FROM documents`).Scan(&docs)
		_ = st.DB.QueryRow(`SELECT COUNT(*) FROM chunks`).Scan(&chunks)
		st.Close()
		out = append(out, Check{Name: "sqlite_fts5", Status: "ok", Detail: fmt.Sprintf("db=%s docs=%d chunks=%d", st.Path, docs, chunks)})
	}

	// 4. Vault directories & quick secret check
	for name, vc := range cfg.Vaults {
		root := config.ExpandHome(vc.Path)
		if _, err := os.Stat(root); err != nil {
			out = append(out, Check{Name: "vault:" + name, Status: "fail", Detail: "path missing: " + root})
			continue
		}
		rep, err := lint.CheckVault(name, root, vc.MaxNoteBytes)
		if err != nil {
			out = append(out, Check{Name: "vault:" + name, Status: "fail", Detail: err.Error()})
			continue
		}
		if rep.Errors > 0 {
			out = append(out, Check{Name: "vault:" + name, Status: "fail", Detail: fmt.Sprintf("%d notes, %d error(s), %d warning(s)", rep.Checked, rep.Errors, rep.Warnings)})
		} else if rep.Warnings > 0 {
			out = append(out, Check{Name: "vault:" + name, Status: "warn", Detail: fmt.Sprintf("%d notes, 0 errors, %d warning(s)", rep.Checked, rep.Warnings)})
		} else {
			out = append(out, Check{Name: "vault:" + name, Status: "ok", Detail: fmt.Sprintf("%d notes, clean", rep.Checked)})
		}
	}

	// 5. Embeddings provider
	prov := embed.New(cfg.Embeddings)
	if prov.Name() == "none" {
		out = append(out, Check{Name: "embeddings", Status: "ok", Detail: "disabled (BM25 only)"})
	} else {
		if vecs, err := prov.Embed(ctx, []string{"healthcheck"}); err != nil || len(vecs) == 0 {
			out = append(out, Check{Name: "embeddings", Status: "warn", Detail: fmt.Sprintf("provider=%s unreachable (%v); searches will degrade to BM25", prov.Name(), err)})
		} else {
			out = append(out, Check{Name: "embeddings", Status: "ok", Detail: fmt.Sprintf("provider=%s model=%s dims=%d", prov.Name(), cfg.Embeddings.Model, len(vecs[0]))})
		}
	}

	return out
}
