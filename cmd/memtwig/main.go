package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vittoriocompagno/memtwig/internal/config"
	"github.com/vittoriocompagno/memtwig/internal/curator"
	"github.com/vittoriocompagno/memtwig/internal/hook"
	"github.com/vittoriocompagno/memtwig/internal/index"
	"github.com/vittoriocompagno/memtwig/internal/install"
	"github.com/vittoriocompagno/memtwig/internal/lint"
	"github.com/vittoriocompagno/memtwig/internal/mcp"
	"github.com/vittoriocompagno/memtwig/internal/proposal"
	"github.com/vittoriocompagno/memtwig/internal/scope"
	"github.com/vittoriocompagno/memtwig/internal/search"
	msync "github.com/vittoriocompagno/memtwig/internal/sync"
	"github.com/vittoriocompagno/memtwig/internal/vault"
)

var (
	version    = "0.1.0"
	cfgPath    string
	dbPath     string
	vaultFlag  string
	cwdFlag    string
)

func main() {
	root := &cobra.Command{
		Use:   "memtwig",
		Short: "Git-native, multi-org Markdown memory layer for AI agents",
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config.toml (default ~/.config/memtwig/config.toml)")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "path to SQLite index (default ~/.cache/memtwig/index.db)")
	root.PersistentFlags().StringVar(&vaultFlag, "vault", "", "explicit vault override")
	root.PersistentFlags().StringVar(&cwdFlag, "cwd", "", "working directory override for scope resolution")

	root.AddCommand(
		cmdVersion(),
		cmdInit(),
		cmdVault(),
		cmdStatus(),
		cmdIndex(),
		cmdSearch(),
		cmdCheck(),
		cmdDecide(),
		cmdInvalidate(),
		cmdCurate(),
		cmdSync(),
		cmdProposal(),
		cmdMCP(),
		cmdHook(),
		cmdInstall(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print memtwig version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("memtwig", version)
		},
	}
}

func cmdInit() *cobra.Command {
	var registerName string
	var kind string
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new memtwig vault directory",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := "."
			if len(args) == 1 {
				target = args[0]
			}
			abs, err := filepath.Abs(config.ExpandHome(target))
			if err != nil {
				return err
			}
			if err := vault.Init(abs); err != nil {
				return err
			}
			fmt.Printf("initialized vault at %s\n", abs)
			if registerName != "" {
				cfg, err := config.Load(cfgPath)
				if err != nil {
					return err
				}
				dw := "direct"
				if kind == "org" {
					dw = "propose"
				}
				cfg.Vaults[registerName] = config.VaultConfig{
					Path:         abs,
					Kind:         kind,
					DefaultWrite: dw,
					MaxNoteBytes: 8192,
				}
				if err := config.Save(cfgPath, cfg); err != nil {
					return err
				}
				fmt.Printf("registered vault %q (%s) in config\n", registerName, kind)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&registerName, "name", "", "register the vault under this name in config.toml")
	cmd.Flags().StringVar(&kind, "kind", "personal", "vault kind: personal | org")
	return cmd
}

func cmdVault() *cobra.Command {
	vc := &cobra.Command{
		Use:   "vault",
		Short: "Manage configured vaults",
	}

	var kind, defaultWrite string
	var matchPaths, matchRemotes, alwaysInclude []string
	addCmd := &cobra.Command{
		Use:   "add <name> <path>",
		Short: "Register an existing directory as a vault",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			abs, err := filepath.Abs(config.ExpandHome(args[1]))
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if defaultWrite == "" {
				if kind == "org" {
					defaultWrite = "propose"
				} else {
					defaultWrite = "direct"
				}
			}
			cfg.Vaults[name] = config.VaultConfig{
				Path:            abs,
				Kind:            kind,
				DefaultWrite:    defaultWrite,
				MatchPaths:      matchPaths,
				MatchGitRemotes: matchRemotes,
				AlwaysInclude:   alwaysInclude,
				MaxNoteBytes:    8192,
			}
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Printf("added vault %q -> %s (kind=%s, write=%s)\n", name, abs, kind, defaultWrite)
			return nil
		},
	}
	addCmd.Flags().StringVar(&kind, "kind", "personal", "personal | org")
	addCmd.Flags().StringVar(&defaultWrite, "write", "", "direct | propose | readonly")
	addCmd.Flags().StringSliceVar(&matchPaths, "match-path", nil, "workspace path glob(s) that activate this vault")
	addCmd.Flags().StringSliceVar(&matchRemotes, "match-remote", nil, "git remote glob(s) that activate this vault")
	addCmd.Flags().StringSliceVar(&alwaysInclude, "always-include", nil, "path globs inside personal vault always included in org scopes")

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List configured vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if len(cfg.Vaults) == 0 {
				fmt.Println("no vaults configured; run: memtwig vault add <name> <path>")
				return nil
			}
			names := make([]string, 0, len(cfg.Vaults))
			for k := range cfg.Vaults {
				names = append(names, k)
			}
			sort.Strings(names)
			for _, n := range names {
				v := cfg.Vaults[n]
				fmt.Printf("%-14s kind=%-8s write=%-8s path=%s\n", n, v.Kind, v.DefaultWrite, v.Path)
			}
			return nil
		},
	}

	rmCmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a vault from config (files on disk are untouched)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			delete(cfg.Vaults, args[0])
			if err := config.Save(cfgPath, cfg); err != nil {
				return err
			}
			fmt.Printf("removed vault %q from config\n", args[0])
			return nil
		},
	}

	vc.AddCommand(addCmd, listCmd, rmCmd)
	return vc
}

func cmdStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show active vaults resolved for current working directory",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			active := scope.Resolve(cfg, cwdFlag, vaultFlag)
			if len(active) == 0 {
				fmt.Println("no active vaults in current scope")
				return nil
			}
			for _, av := range active {
				access := "full"
				if !av.FullAccess {
					access = "globs:" + strings.Join(av.PathGlobs, ",")
				}
				props, _ := proposal.List(config.ExpandHome(av.Config.Path), false)
				fmt.Printf("%-14s reason=%-20s access=%-20s write=%-8s proposals=%d\n",
					av.Name, av.MatchReason, access, av.Config.DefaultWrite, len(props))
			}
			return nil
		},
	}
}

func cmdIndex() *cobra.Command {
	var full bool
	cmd := &cobra.Command{
		Use:   "index",
		Short: "Index all configured vaults into local SQLite FTS5 + vector store",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			stats, err := st.RebuildOrUpdate(context.Background(), cfg, full)
			if err != nil {
				return err
			}
			fmt.Printf("vaults=%d indexed=%d skipped=%d removed=%d chunks=%d embeds=%d db=%s\n",
				stats.Vaults, stats.Indexed, stats.Skipped, stats.Removed, stats.Chunks, stats.Embeds, st.Path)
			return nil
		},
	}
	cmd.Flags().BoolVar(&full, "full", false, "force full reindex")
	return cmd
}

func cmdSearch() *cobra.Command {
	var topK int
	var asJSON bool
	var noVectors bool
	var maxChars int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search active vaults with hybrid BM25 + vector RRF",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			active := scope.Resolve(cfg, cwdFlag, vaultFlag)
			q := strings.Join(args, " ")
			results, warn, err := search.Query(context.Background(), st, cfg, active, q, search.Options{
				TopK:      topK,
				NoVectors: noVectors,
				MaxChars:  maxChars,
			})
			if err != nil {
				return err
			}
			if warn != "" {
				fmt.Fprintln(os.Stderr, "warning:", warn)
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			if len(results) == 0 {
				fmt.Println("no matches")
				return nil
			}
			for i, r := range results {
				fmt.Printf("%d. [%s] %s (%s) [score=%.4f]\n   %s\n", i+1, r.Vault, r.Path, r.Title, r.Score, r.Snippet)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&topK, "n", "n", 5, "max results")
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	cmd.Flags().BoolVar(&noVectors, "no-vectors", false, "BM25 only")
	cmd.Flags().IntVar(&maxChars, "max-chars", 400, "max snippet length")
	return cmd
}

func cmdCheck() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run deterministic linter and secret scanner on active vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			active := scope.Resolve(cfg, cwdFlag, vaultFlag)
			var reports []lint.Report
			totalErrors := 0
			for _, av := range active {
				rep, err := lint.CheckVault(av.Name, config.ExpandHome(av.Config.Path), av.Config.MaxNoteBytes)
				if err != nil {
					return err
				}
				reports = append(reports, rep)
				totalErrors += rep.Errors
			}
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(reports)
			} else {
				for _, rep := range reports {
					fmt.Printf("[%s] checked=%d errors=%d warnings=%d\n", rep.VaultName, rep.Checked, rep.Errors, rep.Warnings)
					for _, iss := range rep.Issues {
						fmt.Printf("  %-7s %s (%s): %s\n", iss.Severity, iss.Path, iss.Code, iss.Message)
					}
				}
			}
			if totalErrors > 0 {
				return fmt.Errorf("%d blocking lint error(s)", totalErrors)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return cmd
}

func cmdProposal() *cobra.Command {
	pc := &cobra.Command{
		Use:   "proposal",
		Short: "Manage agent proposals (list, add, ok, no)",
	}

	resolveOneVault := func(cfg *config.Config) (string, string, error) {
		active := scope.Resolve(cfg, cwdFlag, vaultFlag)
		if len(active) == 0 {
			return "", "", fmt.Errorf("no active vault in scope (use --vault <name>)")
		}
		if vaultFlag != "" {
			for _, av := range active {
				if av.Name == vaultFlag {
					return av.Name, config.ExpandHome(av.Config.Path), nil
				}
			}
			return "", "", fmt.Errorf("vault %q not found", vaultFlag)
		}
		return active[0].Name, config.ExpandHome(active[0].Config.Path), nil
	}

	var allFlag bool
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List proposals",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			vName, vRoot, err := resolveOneVault(cfg)
			if err != nil {
				return err
			}
			props, err := proposal.List(vRoot, allFlag)
			if err != nil {
				return err
			}
			if len(props) == 0 {
				fmt.Printf("[%s] no proposals\n", vName)
				return nil
			}
			for _, p := range props {
				fmt.Printf("%s  %-8s %-6s %s  (%s)\n", p.ID, p.Status, p.Kind, p.Path, p.Reason)
			}
			return nil
		},
	}
	listCmd.Flags().BoolVar(&allFlag, "all", false, "include closed proposals")

	okCmd := &cobra.Command{
		Use:   "ok <id>",
		Short: "Approve and apply a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			_, vRoot, err := resolveOneVault(cfg)
			if err != nil {
				return err
			}
			p, err := proposal.Approve(vRoot, args[0])
			if err != nil {
				return err
			}
			fmt.Printf("approved %s -> %s\n", p.ID, p.Path)
			return nil
		},
	}

	noCmd := &cobra.Command{
		Use:   "no <id> [reason]",
		Short: "Reject a proposal (archived so it will not reopen)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			_, vRoot, err := resolveOneVault(cfg)
			if err != nil {
				return err
			}
			reason := strings.Join(args[1:], " ")
			p, err := proposal.Reject(vRoot, args[0], reason)
			if err != nil {
				return err
			}
			fmt.Printf("rejected %s (%s)\n", p.ID, p.Outcome)
			return nil
		},
	}

	pc.AddCommand(listCmd, okCmd, noCmd)
	return pc
}

func cmdMCP() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server on stdio",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			srv := &mcp.Server{
				Cfg:           cfg,
				Store:         st,
				Cwd:           cwdFlag,
				ExplicitVault: vaultFlag,
				Version:       version,
			}
			return srv.ServeStdio(context.Background(), os.Stdin, os.Stdout)
		},
	}
}

func cmdHook() *cobra.Command {
	return &cobra.Command{
		Use:   "hook <claude|codex|antigravity>",
		Short: "Run per-turn prefetch & correction-nudge hook adapter",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			return hook.Run(context.Background(), args[0], cfg, st, os.Stdin, os.Stdout)
		},
	}
}

func resolvePrimaryVault(cfg *config.Config) (scope.ActiveVault, error) {
	active := scope.Resolve(cfg, cwdFlag, vaultFlag)
	if len(active) == 0 {
		return scope.ActiveVault{}, fmt.Errorf("no active vault in scope (use --vault <name>)")
	}
	if vaultFlag != "" {
		for _, av := range active {
			if av.Name == vaultFlag {
				return av, nil
			}
		}
		return scope.ActiveVault{}, fmt.Errorf("vault %q not found", vaultFlag)
	}
	for _, av := range active {
		if av.FullAccess {
			return av, nil
		}
	}
	return active[0], nil
}

func cmdDecide() *cobra.Command {
	var project, targetNote string
	cmd := &cobra.Command{
		Use:   "decide <summary>",
		Short: "Log a dated decision in decisions/YYYY-MM.md and link it to a project note",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			av, err := resolvePrimaryVault(cfg)
			if err != nil {
				return err
			}
			if project == "" {
				project = av.Name
			}
			msg, err := curator.Decide(
				config.ExpandHome(av.Config.Path),
				project,
				strings.Join(args, " "),
				targetNote,
				av.Config.DefaultWrite == "direct",
			)
			if err != nil {
				return err
			}
			fmt.Println(msg)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "project label (defaults to vault name)")
	cmd.Flags().StringVar(&targetNote, "note", "", "relative path to project note to update/propose")
	return cmd
}

func cmdInvalidate() *cobra.Command {
	var matchStr, reason string
	cmd := &cobra.Command{
		Use:   "invalidate <rel-path>",
		Short: "Move outdated lines from a live note into its append-only archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if matchStr == "" {
				return fmt.Errorf("--match is required")
			}
			if reason == "" {
				reason = "superseded"
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			av, err := resolvePrimaryVault(cfg)
			if err != nil {
				return err
			}
			n, arch, err := curator.Invalidate(config.ExpandHome(av.Config.Path), args[0], matchStr, reason)
			if err != nil {
				return err
			}
			fmt.Printf("moved %d line(s) from %s to %s\n", n, args[0], arch)
			return nil
		},
	}
	cmd.Flags().StringVar(&matchStr, "match", "", "substring matching lines to archive")
	cmd.Flags().StringVar(&reason, "reason", "superseded", "reason for invalidation")
	return cmd
}

func cmdCurate() *cobra.Command {
	var staleDays int
	cmd := &cobra.Command{
		Use:   "curate",
		Short: "Run deterministic curation pass (oversize, stale notes, unsynced decisions, index)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			active := scope.Resolve(cfg, cwdFlag, vaultFlag)
			for _, av := range active {
				if !av.FullAccess {
					continue
				}
				rep, err := curator.Curate(av.Name, config.ExpandHome(av.Config.Path), av.Config.MaxNoteBytes, staleDays)
				if err != nil {
					return err
				}
				fmt.Printf("[%s] notes=%d oversize=%d stale=%d unsynced_decisions=%d proposals_created=%d\n",
					rep.VaultName, rep.IndexNotes, rep.OversizeNotes, rep.StaleNotes, rep.UnsyncedDecisions, rep.ProposalsCreated)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&staleDays, "stale-days", 90, "flag active notes not updated in N days")
	return cmd
}

func cmdSync() *cobra.Command {
	var push bool
	var message string
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Lint, commit dirty vault changes, optionally push, and refresh SQLite index",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			active := scope.Resolve(cfg, cwdFlag, vaultFlag)
			for _, av := range active {
				if !av.FullAccess {
					continue
				}
				res, err := msync.SyncVault(av.Name, config.ExpandHome(av.Config.Path), av.Config.MaxNoteBytes, msync.Options{
					Push:    push,
					Message: message,
				})
				if err != nil {
					return err
				}
				fmt.Printf("[%s] %s\n", res.VaultName, res.Summary)
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			stats, err := st.RebuildOrUpdate(context.Background(), cfg, false)
			if err != nil {
				return err
			}
			fmt.Printf("index refreshed: indexed=%d skipped=%d removed=%d\n", stats.Indexed, stats.Skipped, stats.Removed)
			return nil
		},
	}
	cmd.Flags().BoolVar(&push, "push", false, "also git pull --rebase and git push")
	cmd.Flags().StringVarP(&message, "message", "m", "", "custom git commit message")
	return cmd
}

func cmdInstall() *cobra.Command {
	var dryRun bool
	var binPath string
	cmd := &cobra.Command{
		Use:   "install <claude|cursor|codex|antigravity>",
		Short: "Configure an AI harness to use memtwig hooks and MCP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := install.Install(args[0], binPath, "", dryRun)
			if err != nil {
				return err
			}
			mode := "updated"
			if res.DryRun {
				mode = "dry-run"
			} else if !res.Changed {
				mode = "already configured"
			}
			fmt.Printf("[%s] %s (%s)\n", res.Harness, res.ConfigPath, mode)
			if res.DryRun || res.Harness == "antigravity" {
				fmt.Println(res.Preview)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print resulting config without writing to disk")
	cmd.Flags().StringVar(&binPath, "bin", "", "override memtwig binary path")
	return cmd
}
