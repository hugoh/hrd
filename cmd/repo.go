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
	"github.com/pterm/pterm"
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
		Name:  "repo",
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
		Name:      "add",
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
				pterm.Warning.Printf("repo %q already exists, updating path\n", name)
			}

			cfg.AddRepo(name, config.Repo{Path: abs, Backends: backends})
			pterm.Success.Printf("added %s (%s) as %q\n", abs, backends[0], name)
		}

		return config.Save(*cfgPath, cfg)
	}
}

func detectBackends(vcsName, abs string) ([]string, error) {
	var backends []string

	if vcsName == "" {
		bs, err := backend.DetectAll(abs)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errNoVCSDetected, err)
		}

		for _, b := range bs {
			backends = append(backends, b.Name())
		}
	} else {
		if _, err := backend.ByName(vcsName); err != nil {
			return nil, fmt.Errorf("checking backend %q: %w", vcsName, err)
		}

		bs, err := backend.DetectAll(abs)
		if err == nil {
			for _, b := range bs {
				backends = append(backends, b.Name())
			}
		}

		if len(backends) == 0 {
			return nil, fmt.Errorf("%s: %w", abs, errNoVCSDetected)
		}
	}

	return backends, nil
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
				pterm.Success.Printf("removed %q\n", name)
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
				Name:    "group",
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

			tableData := pterm.TableData{
				{"NAME", "VCS", "PATH"},
			}

			for _, name := range names {
				repo := cfg.Repos[name]

				vcsLabel := repo.ActiveBackend()
				if len(repo.Backends) > 1 {
					vcsLabel = strings.Join(repo.Backends, ",")
				}

				tableData = append(tableData, []string{name, vcsLabel, repo.Path})
			}

			_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()

			return nil
		},
	}
}

const repoNameAndVCS = 2

func repoRenameCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "rename",
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

			pterm.Success.Printf("renamed %q → %q\n", oldName, newName)

			return config.Save(*cfgPath, cfg)
		},
	}
}

func repoRefreshCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "refresh",
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
			repo := cfg.Repos[name]

			backends, err := backend.DetectAll(repo.Path)
			if err != nil {
				pterm.FgLightRed.Printf("%s: %v\n", name, err)

				continue
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

				pterm.Printf("%s:%s (%s)\n", name, note, strings.Join(newBackends, ", "))
			}

			cfg.AddRepo(name, config.Repo{Path: repo.Path, Backends: newBackends})
		}

		return config.Save(*cfgPath, cfg)
	}
}
