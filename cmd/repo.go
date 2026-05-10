package cmd

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/urfave/cli/v3"
)

var (
	errAtLeastOnePath  = errors.New("at least one path required")
	errNameSingleRepo  = errors.New("--name can only be used when adding a single repo")
	errAtLeastOneName  = errors.New("at least one repo name required")
	errAtLeastOneOrAll = errors.New("at least one repo name required, or use --all")
	errRepoRenameUsage = errors.New("usage: repo rename <old> <new>")
	errUnknownRepo     = errors.New("unknown repo")
	errRepoExists      = errors.New("repo already exists")
	errNoVCSDetected   = errors.New("no VCS detected")
)

// repoCommands returns the `repo` subcommand with its children.
func repoCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameRepo,
		Usage: "manage tracked repositories",
		Commands: []*cli.Command{
			repoAddCmd(cfgPath),
			repoRemoveCmd(cfgPath),
			repoListCmd(cfgPath),
			repoRenameCmd(cfgPath),
			repoRefreshCmd(cfgPath),
		},
	}
}

func repoAddCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameAdd,
		Usage:     "add one or more repositories",
		ArgsUsage: "<path>...",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "vcs",
				Usage: "override VCS detection (git|jj|…)",
			},
			&cli.StringFlag{
				Name:    "name",
				Aliases: []string{"n"},
				Usage:   "explicit name (only valid when adding a single repo)",
			},
		},
		Action: repoAddAction(cfgPath),
	}
}

func repoAddAction(cfgPath *string) func(_ context.Context, cmd *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		if cmd.NArg() == 0 {
			return errAtLeastOnePath
		}

		if cmd.String("name") != "" && cmd.NArg() > 1 {
			return errNameSingleRepo
		}

		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		for _, arg := range cmd.Args().Slice() {
			abs, err := filepath.Abs(arg)
			if err != nil {
				return fmt.Errorf("resolving %q: %w", arg, err)
			}

			backends, err := detectBackends(cmd.String("vcs"), abs)
			if err != nil {
				return fmt.Errorf("%s: %w", abs, err)
			}

			name := cmd.String("name")
			if name == "" {
				name = filepath.Base(abs)
			}

			if _, exists := cfg.Repos[name]; exists {
				ui.Warn("repo %q already exists, updating path", name)
			}

			cfg.AddRepo(name, config.Repo{Path: abs, Backends: backends})
			ui.Success("added %s (%s) as %q", abs, backends[0], name)
		}

		return config.Save(*cfgPath, cfg)
	}
}

func detectBackends(vcsName, abs string) ([]string, error) {
	if vcsName == "" {
		return detectAllBackends(abs)
	}

	return detectExplicitBackend(vcsName, abs)
}

func detectAllBackends(abs string) ([]string, error) {
	bList, err := backend.DetectAll(abs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errNoVCSDetected, err)
	}

	names := make([]string, len(bList))

	for i, b := range bList {
		names[i] = b.Name()
	}

	return names, nil
}

func detectExplicitBackend(vcsName, abs string) ([]string, error) {
	if _, err := backend.ByName(vcsName); err != nil {
		return nil, fmt.Errorf("checking backend %q: %w", vcsName, err)
	}

	bList, _ := backend.DetectAll(abs)

	if len(bList) == 0 {
		return nil, fmt.Errorf("%s: %w", abs, errNoVCSDetected)
	}

	names := make([]string, len(bList))

	for i, b := range bList {
		names[i] = b.Name()
	}

	return names, nil
}

func repoRemoveCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove one or more repositories",
		ArgsUsage: "<name>...",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return errAtLeastOneName
			}

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			for _, name := range cmd.Args().Slice() {
				if _, ok := cfg.Repos[name]; !ok {
					return fmt.Errorf("%w %q", errUnknownRepo, name)
				}

				cfg.RemoveRepo(name)
				ui.Success("removed %q", name)
			}

			return config.Save(*cfgPath, cfg)
		},
	}
}

func repoListCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list tracked repositories",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    cmdNameGroup,
				Aliases: []string{"g"},
				Usage:   "filter to repos in a group",
			},
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			names := make([]string, 0, len(cfg.Repos))

			if g := cmd.String("group"); g != "" {
				grp, ok := cfg.Groups[g]
				if !ok {
					return fmt.Errorf("%w %q", errUnknownGroup, g)
				}

				names = grp.Repos
			} else {
				for name := range cfg.Repos {
					names = append(names, name)
				}

				sort.Strings(names)
			}

			tbl := ui.NewTable()
			tbl.AppendHeader(table.Row{"NAME", "VCS", "PATH"})
			tbl.SetColumnConfigs([]table.ColumnConfig{
				{Number: 1, AutoMerge: true},
				{Number: colVCS, AutoMerge: true},
			})

			for _, name := range names {
				repo := cfg.Repos[name]

				vcsLabel := repo.ActiveBackend()
				if len(repo.Backends) > 1 {
					vcsLabel = strings.Join(repo.Backends, ",")
				}

				tbl.AppendRow(table.Row{name, vcsLabel, repo.Path})
			}

			tbl.Render()

			return nil
		},
	}
}

const (
	repoNameAndVCS = 2
	cmdNameRepo    = "repo"
	cmdNameRename  = "rename"
	cmdNameRefresh = "refresh"
)

func repoRenameCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameRename,
		Usage:     "rename a repository",
		ArgsUsage: "<old-name> <new-name>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != repoNameAndVCS {
				return errRepoRenameUsage
			}

			oldName, newName := cmd.Args().Get(0), cmd.Args().Get(1)

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			repo, ok := cfg.Repos[oldName]
			if !ok {
				return fmt.Errorf("%w %q", errUnknownRepo, oldName)
			}

			if _, exists := cfg.Repos[newName]; exists {
				return fmt.Errorf("%w %q", errRepoExists, newName)
			}

			cfg.RemoveRepo(oldName)
			cfg.AddRepo(newName, repo)
			// Fixup group references.
			for gname, group := range cfg.Groups {
				for i, r := range group.Repos {
					if r == oldName {
						group.Repos[i] = newName
					}
				}

				cfg.Groups[gname] = group
			}

			ui.Success("renamed %q → %q", oldName, newName)

			return config.Save(*cfgPath, cfg)
		},
	}
}

func repoRefreshCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameRefresh,
		Usage:     "re-detect VCS for one or more repos",
		ArgsUsage: "<name>...",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "refresh all known repos",
			},
		},
		Action: repoRefreshAction(cfgPath),
	}
}

func repoRefreshAction(cfgPath *string) func(_ context.Context, cmd *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		var names []string

		if cmd.Bool("all") {
			for name := range cfg.Repos {
				names = append(names, name)
			}

			sort.Strings(names)
		} else {
			if cmd.NArg() == 0 {
				return errAtLeastOneOrAll
			}

			names = cmd.Args().Slice()
			for _, name := range names {
				if _, ok := cfg.Repos[name]; !ok {
					return fmt.Errorf("%w %q", errUnknownRepo, name)
				}
			}
		}

		for _, name := range names {
			refreshRepo(name, &cfg)
		}

		return config.Save(*cfgPath, cfg)
	}
}

func refreshRepo(name string, cfg *config.Config) {
	repo := cfg.Repos[name]

	backends, err := backend.DetectAll(repo.Path)
	if err != nil {
		ui.Fail("%s: %v", name, err)

		return
	}

	var newBackends []string
	for _, b := range backends {
		newBackends = append(newBackends, b.Name())
	}

	if !slices.Equal(newBackends, repo.Backends) {
		var note string
		if newBackends[0] != repo.ActiveBackend() {
			note = fmt.Sprintf(" %s → %s", repo.ActiveBackend(), newBackends[0])
		}

		ui.Outf("%s:%s (%s)", name, note, strings.Join(newBackends, ", "))
	}

	cfg.AddRepo(name, config.Repo{Path: repo.Path, Backends: newBackends})
}
