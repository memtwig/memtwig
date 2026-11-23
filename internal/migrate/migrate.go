package migrate

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/lint"
	"github.com/memtwig/memtwig/internal/vault"
)

type Report struct {
	VaultName string
	Root      string
	Notes     int
	Errors    int
	Warnings  int
	DryRun    bool
}

// ImportExisting registers an existing Markdown/Obsidian/~/notes directory into memtwig config
// without modifying or overwriting existing notes.
func ImportExisting(cfgPath, name, dir, kind, defaultWrite string, alwaysInclude []string, dryRun bool) (Report, error) {
	abs, err := filepath.Abs(config.ExpandHome(dir))
	if err != nil {
		return Report{}, err
	}
	if _, err := os.Stat(abs); err != nil {
		return Report{}, fmt.Errorf("source directory %s: %w", abs, err)
	}
	if kind == "" {
		kind = "personal"
	}
	if defaultWrite == "" {
		if kind == "org" {
			defaultWrite = "propose"
		} else {
			defaultWrite = "direct"
		}
	}

	notes, err := vault.WalkNotes(name, abs)
	if err != nil {
		return Report{}, err
	}
	lintRep, err := lint.CheckVault(name, abs, 8192)
	if err != nil {
		return Report{}, err
	}

	rep := Report{
		VaultName: name,
		Root:      abs,
		Notes:     len(notes),
		Errors:    lintRep.Errors,
		Warnings:  lintRep.Warnings,
		DryRun:    dryRun,
	}

	if dryRun {
		return rep, nil
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return rep, err
	}
	cfg.Vaults[name] = config.VaultConfig{
		Path:          abs,
		Kind:          kind,
		DefaultWrite:  defaultWrite,
		AlwaysInclude: alwaysInclude,
		MaxNoteBytes:  8192,
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		return rep, err
	}
	return rep, nil
}
