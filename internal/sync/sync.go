package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/memtwig/memtwig/internal/lint"
	"github.com/memtwig/memtwig/internal/vault"
)

type Options struct {
	Push         bool
	Message      string
	ScanEnvFiles []string
}

type Result struct {
	VaultName string
	Committed bool
	Pushed    bool
	Summary   string
}

// SyncVault locks the git repo (if any), pulls remote updates before committing when push=true,
// runs lint (blocking on secrets, literal env secrets, or merge-conflict markers), regenerates
// index.md if memtwig-managed, commits dirty changes, and pushes.
func SyncVault(vaultName, root string, maxNoteBytes int, opts Options) (Result, error) {
	res := Result{VaultName: vaultName}

	hasGit := false
	if _, err := os.Stat(filepath.Join(root, ".git")); err == nil {
		hasGit = true
		unlock, err := acquireRepoLock(root)
		if err != nil {
			return res, err
		}
		defer unlock()

		if opts.Push {
			if remote, _ := gitRun(root, "remote"); strings.TrimSpace(remote) != "" {
				if _, err := gitRun(root, "pull", "--rebase", "--autostash"); err != nil {
					_, _ = gitRun(root, "rebase", "--abort")
					return res, fmt.Errorf("git pull --rebase: %w", err)
				}
			}
		}
	}

	rep, err := lint.CheckVaultWithOptions(vaultName, root, lint.CheckOptions{
		MaxNoteBytes: maxNoteBytes,
		ScanEnvFiles: opts.ScanEnvFiles,
	})
	if err != nil {
		return res, err
	}
	if rep.Errors > 0 {
		return res, fmt.Errorf("sync aborted on %s: %d lint error(s) (run 'memtwig check' for details)", vaultName, rep.Errors)
	}

	if _, err := os.Stat(filepath.Join(root, "memtwig.toml")); err == nil {
		_, _ = vault.GenerateIndex(root)
	}

	if !hasGit {
		res.Summary = "checked (not a git repo)"
		return res, nil
	}

	statusOut, err := gitRun(root, "status", "--porcelain")
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(statusOut) != "" {
		if _, err := gitRun(root, "add", "-A"); err != nil {
			return res, err
		}
		msg := opts.Message
		if msg == "" {
			msg = "memtwig sync " + time.Now().Format("2006-01-02 15:04")
		}
		if _, err := gitRun(root, "commit", "-m", msg); err != nil {
			return res, err
		}
		res.Committed = true
	}

	if opts.Push {
		if remote, _ := gitRun(root, "remote"); strings.TrimSpace(remote) != "" {
			if _, err := gitRun(root, "push"); err != nil {
				return res, fmt.Errorf("git push: %w", err)
			}
			res.Pushed = true
		}
	}

	switch {
	case res.Committed && res.Pushed:
		res.Summary = "committed & pushed"
	case res.Committed:
		res.Summary = "committed locally"
	case res.Pushed:
		res.Summary = "pushed"
	default:
		res.Summary = "clean"
	}
	return res, nil
}

func gitRun(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("git %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
