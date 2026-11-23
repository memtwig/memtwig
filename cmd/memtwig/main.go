package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/memtwig/memtwig/internal/audit"
	"github.com/memtwig/memtwig/internal/config"
	"github.com/memtwig/memtwig/internal/curator"
	"github.com/memtwig/memtwig/internal/doctor"
	"github.com/memtwig/memtwig/internal/hook"
	"github.com/memtwig/memtwig/internal/index"
	"github.com/memtwig/memtwig/internal/install"
	"github.com/memtwig/memtwig/internal/lint"
	"github.com/memtwig/memtwig/internal/mcp"
	"github.com/memtwig/memtwig/internal/migrate"
	"github.com/memtwig/memtwig/internal/proposal"
	"github.com/memtwig/memtwig/internal/scope"
	"github.com/memtwig/memtwig/internal/search"
	"github.com/memtwig/memtwig/internal/server"
	msync "github.com/memtwig/memtwig/internal/sync"
	"github.com/memtwig/memtwig/internal/tui"
	"github.com/memtwig/memtwig/internal/ui"
	"github.com/memtwig/memtwig/internal/vault"
)

var (
	version    = "1.4.1"
	cfgPath    string
	dbPath     string
	vaultFlag  string
	cwdFlag    string
)

func main() {
	root := &cobra.Command{
		Use:          "memtwig",
		Short:        "Git-native, multi-org Markdown memory layer for AI agents",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config.toml (default ~/.config/memtwig/config.toml)")
	root.PersistentFlags().StringVar(&dbPath, "db", "", "path to SQLite index (default ~/.cache/memtwig/index.db)")
	root.PersistentFlags().StringVar(&vaultFlag, "vault", "", "explicit vault override")
	root.PersistentFlags().StringVar(&cwdFlag, "cwd", "", "working directory override for scope resolution")

	root.AddGroup(
		&cobra.Group{ID: "start", Title: "Getting Started & Interfaces:"},
		&cobra.Group{ID: "memory", Title: "Search, Vaults & Scope:"},
		&cobra.Group{ID: "govern", Title: "Governance & Hygiene:"},
		&cobra.Group{ID: "connect", Title: "Harnesses & Server:"},
	)

	withGroup := func(id string, c *cobra.Command) *cobra.Command {
		c.GroupID = id
		return c
	}

	root.AddCommand(
		withGroup("start", cmdSetup()),
		withGroup("start", cmdInit()),
		withGroup("start", cmdMigrate()),
		withGroup("start", cmdUI()),
		withGroup("start", cmdTUI()),
		withGroup("start", cmdDoctor()),

		withGroup("memory", cmdSearch()),
		withGroup("memory", cmdStatus()),
		withGroup("memory", cmdVault()),
		withGroup("memory", cmdIndex()),
		withGroup("memory", cmdWatch()),

		withGroup("govern", cmdProposal()),
		withGroup("govern", cmdDecide()),
		withGroup("govern", cmdInvalidate()),
		withGroup("govern", cmdCurate()),
		withGroup("govern", cmdCheck()),
		withGroup("govern", cmdSync()),

		withGroup("connect", cmdInstall()),
		withGroup("connect", cmdMCP()),
		withGroup("connect", cmdServe()),
		withGroup("connect", cmdHook()),
		withGroup("connect", cmdVersion()),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func cmdSetup() *cobra.Command {
	var dir string
	var name string
	var applyInstall bool
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "One-command setup: create or import personal vault, index it, and detect AI harnesses",
		RunE: func(cmd *cobra.Command, args []string) error {
			abs, err := filepath.Abs(config.ExpandHome(dir))
			if err != nil {
				return err
			}
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if _, exists := cfg.Vaults[name]; !exists {
				if _, statErr := os.Stat(abs); os.IsNotExist(statErr) {
					if err := vault.InitWithOptions(abs, vault.InitOptions{GitInit: true}); err != nil {
						return err
					}
					fmt.Printf("1. Created personal vault %q at %s\n", name, abs)
				} else {
					fmt.Printf("1. Imported existing notes folder %q at %s\n", name, abs)
				}
				cfg.Vaults[name] = config.VaultConfig{
					Path:          abs,
					Kind:          "personal",
					DefaultWrite:  "direct",
					AlwaysInclude: []string{"persona/**"},
					MaxNoteBytes:  8192,
				}
				if err := config.Save(cfgPath, cfg); err != nil {
					return err
				}
			} else {
				fmt.Printf("1. Vault %q already configured (%s)\n", name, cfg.Vaults[name].Path)
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
			fmt.Printf("2. Indexed %d vault(s): %d updated, %d skipped, %d chunks\n",
				stats.Vaults, stats.Indexed, stats.Skipped, stats.Chunks)

			home, _ := os.UserHomeDir()
			type candidate struct {
				id, label, marker string
			}
			candidates := []candidate{
				{"claude", "Claude Code", filepath.Join(home, ".claude")},
				{"cursor", "Cursor", filepath.Join(home, ".cursor")},
				{"codex", "Codex", filepath.Join(home, ".codex")},
				{"antigravity", "Antigravity", filepath.Join(home, ".gemini")},
			}
			var found []candidate
			for _, c := range candidates {
				if _, err := os.Stat(c.marker); err == nil {
					found = append(found, c)
				}
			}
			if len(found) == 0 {
				fmt.Println("3. No AI harness config folders detected yet (run `memtwig install <claude|cursor|codex|antigravity>` anytime)")
			} else if applyInstall {
				fmt.Println("3. Connecting detected AI harnesses:")
				for _, c := range found {
					res, err := install.Install(c.id, "", "", false)
					if err != nil {
						fmt.Printf("   - %s: error (%v)\n", c.label, err)
					} else {
						state := "updated"
						if !res.Changed {
							state = "already configured"
						}
						fmt.Printf("   - %s: %s (%s)\n", c.label, res.ConfigPath, state)
					}
				}
			} else {
				fmt.Println("3. Detected AI harnesses (pass `--install` or run individually):")
				for _, c := range found {
					fmt.Printf("   - %s -> memtwig install %s\n", c.label, c.id)
				}
			}

			fmt.Println("\nNext:")
			fmt.Println("  memtwig ui                  # open local Studio Web UI")
			fmt.Println("  memtwig tui                 # open terminal UI")
			fmt.Println("  memtwig search \"query\"      # search active vaults")
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "~/notes", "personal vault directory to create or import")
	cmd.Flags().StringVar(&name, "name", "personal", "vault name")
	cmd.Flags().BoolVar(&applyInstall, "install", false, "automatically install hooks/MCP into detected harnesses")
	return cmd
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
	var kind, defaultWrite string
	var matchPaths, matchRemotes, alwaysInclude []string
	var noScaffold, noGit bool
	cmd := &cobra.Command{
		Use:   "init [path]",
		Short: "Initialize a new memtwig vault directory (with git init and optional routing)",
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
			skipSample := noScaffold || (kind == "org" && !cmd.Flags().Changed("no-scaffold"))
			if err := vault.InitWithOptions(abs, vault.InitOptions{
				NoScaffold: skipSample,
				GitInit:    !noGit,
			}); err != nil {
				return err
			}
			fmt.Printf("initialized vault at %s\n", abs)
			if registerName != "" {
				cfg, err := config.Load(cfgPath)
				if err != nil {
					return err
				}
				dw := defaultWrite
				if dw == "" {
					if kind == "org" {
						dw = "propose"
					} else {
						dw = "direct"
					}
				}
				cfg.Vaults[registerName] = config.VaultConfig{
					Path:            abs,
					Kind:            kind,
					DefaultWrite:    dw,
					MatchPaths:      matchPaths,
					MatchGitRemotes: matchRemotes,
					AlwaysInclude:   alwaysInclude,
					MaxNoteBytes:    8192,
				}
				if err := config.Save(cfgPath, cfg); err != nil {
					return err
				}
				fmt.Printf("registered vault %q (%s, write=%s) in config\n", registerName, kind, dw)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&registerName, "name", "", "register the vault under this name in config.toml")
	cmd.Flags().StringVar(&kind, "kind", "personal", "vault kind: personal | org")
	cmd.Flags().StringVar(&defaultWrite, "write", "", "direct | propose | readonly")
	cmd.Flags().StringSliceVar(&matchPaths, "match-path", nil, "workspace path glob(s) that activate this vault")
	cmd.Flags().StringSliceVar(&matchRemotes, "match-remote", nil, "git remote glob(s) that activate this vault")
	cmd.Flags().StringSliceVar(&alwaysInclude, "always-include", nil, "path globs inside personal vault always included in org scopes")
	cmd.Flags().BoolVar(&noScaffold, "no-scaffold", false, "do not create projects/getting-started.md (default true for --kind org)")
	cmd.Flags().BoolVar(&noGit, "no-git", false, "skip git init inside the vault directory")
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
			_, existed := cfg.Vaults[name]
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
			verb := "added"
			if existed {
				verb = "updated"
			}
			fmt.Printf("%s vault %q -> %s (kind=%s, write=%s)\n", verb, name, abs, kind, defaultWrite)
			return nil
		},
	}
	addCmd.Flags().StringVar(&kind, "kind", "personal", "personal | org")
	addCmd.Flags().StringVar(&defaultWrite, "write", "", "direct | propose | readonly")
	addCmd.Flags().StringSliceVar(&matchPaths, "match-path", nil, "workspace path glob(s) that activate this vault")
	addCmd.Flags().StringSliceVar(&matchRemotes, "match-remote", nil, "git remote glob(s) that activate this vault")
	addCmd.Flags().StringSliceVar(&alwaysInclude, "always-include", nil, "path globs inside personal vault always included in org scopes")

	listCmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List configured vaults",
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
				var extras []string
				if len(v.MatchPaths) > 0 {
					extras = append(extras, "paths="+strings.Join(v.MatchPaths, ","))
				}
				if len(v.MatchGitRemotes) > 0 {
					extras = append(extras, "remotes="+strings.Join(v.MatchGitRemotes, ","))
				}
				if len(v.AlwaysInclude) > 0 {
					extras = append(extras, "always="+strings.Join(v.AlwaysInclude, ","))
				}
				if len(extras) > 0 {
					fmt.Printf("%-14s kind=%-8s write=%-8s path=%s  [%s]\n", n, v.Kind, v.DefaultWrite, v.Path, strings.Join(extras, " "))
				} else {
					fmt.Printf("%-14s kind=%-8s write=%-8s path=%s\n", n, v.Kind, v.DefaultWrite, v.Path)
				}
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

func cmdWatch() *cobra.Command {
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Continuously watch configured vaults and incrementally refresh the SQLite index",
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
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			fmt.Printf("watching %d vault(s) every %s (Ctrl+C to stop)\n", len(cfg.Vaults), interval)
			return st.Watch(ctx, cfg, interval, func(s index.Stats) {
				fmt.Printf("[%s] refreshed: indexed=%d removed=%d chunks=%d\n",
					time.Now().Format("15:04:05"), s.Indexed, s.Removed, s.Chunks)
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 3*time.Second, "poll interval")
	return cmd
}

func cmdSearch() *cobra.Command {
	var topK int
	var asJSON bool
	var noVectors bool
	var noRefresh bool
	var maxChars int
	cmd := &cobra.Command{
		Use:     "search <query>",
		Aliases: []string{"s"},
		Short:   "Search active vaults with hybrid BM25 + vector RRF (auto-refreshes changed files)",
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

			if !noRefresh {
				_, _ = st.EnsureFresh(context.Background(), cfg)
			}
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
				var names []string
				for _, av := range active {
					names = append(names, av.Name)
				}
				if len(names) > 0 {
					fmt.Printf("no matches (active vaults: %s)\n", strings.Join(names, ", "))
				} else {
					fmt.Println("no matches (no active vaults in scope)")
				}
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
	cmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "skip automatic incremental reindex before searching")
	cmd.Flags().IntVar(&maxChars, "max-chars", 400, "max snippet length")
	return cmd
}

func cmdCheck() *cobra.Command {
	var asJSON bool
	var allVaults bool
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Run deterministic linter and secret scanner on active (or --all) vaults",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			var targets []scope.ActiveVault
			if allVaults {
				names := make([]string, 0, len(cfg.Vaults))
				for k := range cfg.Vaults {
					names = append(names, k)
				}
				sort.Strings(names)
				for _, n := range names {
					targets = append(targets, scope.ActiveVault{Name: n, Config: cfg.Vaults[n], FullAccess: true})
				}
			} else {
				targets = scope.Resolve(cfg, cwdFlag, vaultFlag)
			}
			var reports []lint.Report
			totalErrors := 0
			for _, av := range targets {
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
	cmd.Flags().BoolVar(&allVaults, "all", false, "check all configured vaults regardless of current directory")
	return cmd
}

func cmdProposal() *cobra.Command {
	pc := &cobra.Command{
		Use:     "proposal",
		Aliases: []string{"p", "proposals"},
		Short:   "Manage agent proposals (list, add, show, ok, no)",
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

	showCmd := &cobra.Command{
		Use:   "show <id>",
		Short: "Inspect full details and proposed content of a proposal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			vName, vRoot, err := resolveOneVault(cfg)
			if err != nil {
				return err
			}
			p, ok, err := proposal.Get(vRoot, args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("proposal %s not found in vault %s", args[0], vName)
			}
			fmt.Printf("ID:      %s\nVault:   %s\nStatus:  %s\nKind:    %s\nPath:    %s\nAuthor:  %s\nCreated: %s\nReason:  %s\n\n%s",
				p.ID, vName, p.Status, p.Kind, p.Path, p.Author, p.CreatedAt, p.Reason, proposal.FormatDiff(vRoot, p))
			return nil
		},
	}

	var addKind, addContent, addContentFile, addReason, addAuthor string
	addCmd := &cobra.Command{
		Use:   "add <rel-path>",
		Short: "Queue a deterministic review proposal (append, replace, or create)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			vName, vRoot, err := resolveOneVault(cfg)
			if err != nil {
				return err
			}
			content := addContent
			if addContentFile != "" {
				raw, err := os.ReadFile(config.ExpandHome(addContentFile))
				if err != nil {
					return err
				}
				content = string(raw)
			} else if content == "" {
				if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
					raw, err := io.ReadAll(os.Stdin)
					if err != nil {
						return err
					}
					content = string(raw)
				}
			}
			if strings.TrimSpace(content) == "" {
				return fmt.Errorf("--content, --content-file, or stdin content is required")
			}
			relPath := filepath.ToSlash(strings.TrimPrefix(args[0], "/"))
			p, created, err := proposal.Add(vRoot, proposal.Proposal{
				Kind:    addKind,
				Path:    relPath,
				Content: content,
				Reason:  addReason,
				Author:  addAuthor,
			})
			if err != nil {
				return err
			}
			state := "existing"
			if created {
				state = "created"
			}
			fmt.Printf("%s proposal %s [%s] %s (%s)\n", state, p.ID, vName, p.Path, p.Kind)
			return nil
		},
	}
	addCmd.Flags().StringVar(&addKind, "kind", "append", "proposal kind: append | replace | create")
	addCmd.Flags().StringVar(&addContent, "content", "", "proposed Markdown content")
	addCmd.Flags().StringVar(&addContentFile, "content-file", "", "read proposed Markdown content from file")
	addCmd.Flags().StringVar(&addReason, "reason", "", "why this change is proposed")
	addCmd.Flags().StringVar(&addAuthor, "author", "cli", "author label recorded on the proposal")

	pc.AddCommand(listCmd, addCmd, showCmd, okCmd, noCmd)
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
	var event string
	cmd := &cobra.Command{
		Use:   "hook <claude|codex|antigravity>",
		Short: "Run per-turn prefetch, correction-nudge, or session-start identity hook",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if event == "session-start" {
				return hook.RunSessionStart(args[0], cfg, os.Stdout)
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			return hook.Run(context.Background(), args[0], cfg, st, os.Stdin, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&event, "event", "prompt", "hook event: prompt | session-start")
	return cmd
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
		Short: "Log a dated decision in decisions/YYYY-MM.md and link/propose it on a note",
		Long:  "Log a dated decision at the top of decisions/YYYY-MM.md. When --note is set, personal direct vaults append directly; org vaults (default_write=propose) or owner-locked notes queue a review proposal.",
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
	var useLLM bool
	var maxPairs int
	cmd := &cobra.Command{
		Use:   "curate",
		Short: "Run deterministic curation pass (and optional --llm semantic contradiction check)",
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
				root := config.ExpandHome(av.Config.Path)
				rep, err := curator.Curate(av.Name, root, av.Config.MaxNoteBytes, staleDays)
				if err != nil {
					return err
				}
				fmt.Printf("[%s] notes=%d oversize=%d stale=%d unsynced_decisions=%d proposals_created=%d\n",
					rep.VaultName, rep.IndexNotes, rep.OversizeNotes, rep.StaleNotes, rep.UnsyncedDecisions, rep.ProposalsCreated)
				if useLLM {
					srep, err := curator.CurateSemantic(context.Background(), av.Name, root, cfg.LLM, maxPairs)
					if err != nil {
						return err
					}
					fmt.Printf("[%s/llm] pairs=%d findings=%d proposals_created=%d\n",
						srep.VaultName, srep.PairsExamined, len(srep.Findings), srep.ProposalsCreated)
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&staleDays, "stale-days", 90, "flag active notes not updated in N days")
	cmd.Flags().BoolVar(&useLLM, "llm", false, "also run semantic contradiction check using configured [llm] provider")
	cmd.Flags().IntVar(&maxPairs, "max-pairs", 6, "max note pairs to inspect in --llm mode")
	return cmd
}

func cmdDoctor() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run environment, vault, SQLite FTS5, and embeddings diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := doctor.Run(context.Background(), cfgPath, dbPath)
			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(checks)
			}
			failed := 0
			for _, c := range checks {
				fmt.Printf("%-6s %-18s %s\n", strings.ToUpper(c.Status), c.Name, c.Detail)
				if c.Status == "fail" {
					failed++
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d diagnostic check(s) failed", failed)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "JSON output")
	return cmd
}

func cmdMigrate() *cobra.Command {
	var kind, defaultWrite string
	var alwaysInclude []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "migrate <name> <dir>",
		Short: "Import an existing Markdown/Obsidian/notes directory without modifying files",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			rep, err := migrate.ImportExisting(cfgPath, args[0], args[1], kind, defaultWrite, alwaysInclude, dryRun)
			if err != nil {
				return err
			}
			mode := "registered"
			if rep.DryRun {
				mode = "dry-run"
			}
			fmt.Printf("[%s] %s (%s): notes=%d errors=%d warnings=%d\n",
				rep.VaultName, rep.Root, mode, rep.Notes, rep.Errors, rep.Warnings)
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "personal", "personal | org")
	cmd.Flags().StringVar(&defaultWrite, "write", "", "direct | propose | readonly")
	cmd.Flags().StringSliceVar(&alwaysInclude, "always-include", nil, "globs always included in org scopes")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "inspect directory without saving config")
	return cmd
}

func cmdServe() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start Streamable HTTP MCP + REST API server with RBAC and audit log",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if addr == "" {
				addr = cfg.Server.Addr
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			auditPath := config.ExpandHome(cfg.Server.AuditLogPath)
			if auditPath == "" {
				if cdir, err := config.DefaultCacheDir(); err == nil {
					auditPath = filepath.Join(cdir, "audit.jsonl")
				}
			}
			srv := &server.Server{
				Cfg:        cfg,
				ConfigPath: cfgPath,
				Store:      st,
				Audit:      audit.New(auditPath),
				StaticUI:   ui.FS(),
				Version:    version,
			}
			fmt.Printf("memtwig %s listening on http://%s (ui=/, mcp=/mcp, api=/api/v1/*, tokens=%d)\n",
				version, addr, len(cfg.Server.Tokens))
			return server.ListenAndServe(context.Background(), addr, srv)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "listen address (default from config [server].addr or 127.0.0.1:8787)")
	return cmd
}

func cmdUI() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Launch Memtwig Studio web UI locally (embedded in the binary)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if addr == "" {
				addr = cfg.Server.Addr
			}
			st, err := index.Open(dbPath)
			if err != nil {
				return err
			}
			defer st.Close()
			_, _ = st.EnsureFresh(context.Background(), cfg)
			auditPath := config.ExpandHome(cfg.Server.AuditLogPath)
			if auditPath == "" {
				if cdir, err := config.DefaultCacheDir(); err == nil {
					auditPath = filepath.Join(cdir, "audit.jsonl")
				}
			}
			srv := &server.Server{
				Cfg:        cfg,
				ConfigPath: cfgPath,
				Store:      st,
				Audit:      audit.New(auditPath),
				StaticUI:   ui.FS(),
				Version:    version,
			}
			fmt.Printf("memtwig Studio %s -> http://%s\n", version, addr)
			return server.ListenAndServe(context.Background(), addr, srv)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", "", "listen address (default 127.0.0.1:8787)")
	return cmd
}

func cmdTUI() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI (search notes, review proposal diffs, inspect vaults & settings)",
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
			auditPath := config.ExpandHome(cfg.Server.AuditLogPath)
			if auditPath == "" {
				if cdir, err := config.DefaultCacheDir(); err == nil {
					auditPath = filepath.Join(cdir, "audit.jsonl")
				}
			}
			return tui.RunWithConfigPath(cfg, cfgPath, st, audit.New(auditPath), cwdFlag, vaultFlag, version)
		},
	}
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
