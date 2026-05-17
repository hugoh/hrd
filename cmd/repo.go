package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

var (
	errAtLeastOnePath  = errors.New("at least one path required")
	errNameSingleRepo  = errors.New("--name can only be used when adding a single repo")
	errAtLeastOneName  = errors.New("at least one repo name required")
	errRepoRenameUsage = errors.New("usage: repo rename <old> <new>")
	errUnknownRepo     = errors.New("unknown repo")
	errRepoExists      = errors.New("repo already exists")
	errRepoNoVCS       = errors.New("no VCS detected")
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

			if _, err := backend.Detect(abs); err != nil {
				return fmt.Errorf("%s: %w", abs, errRepoNoVCS)
			}

			name := cmd.String("name")
			if name == "" {
				name = filepath.Base(abs)
			}

			if _, exists := cfg.Repos[name]; exists {
				return fmt.Errorf(
					"%w %q (path: %s). use --name/-n to specify a unique name",
					errRepoExists,
					name,
					cfg.Repos[name].Path,
				)
			}

			cfg.AddRepo(name, config.Repo{Path: abs})
			ui.Success("added %s as %q", abs, name)
		}

		return config.Save(*cfgPath, cfg)
	}
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

			if g := stripGroupPrefix(cmd.String("group")); g != "" {
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

			const nameWidth = 15

			const vcsWidth = 3

			const gap = 2

			pathWidth := ui.GetTermWidth() - nameWidth - vcsWidth - gap

			rows := make([][]string, 0, len(names))
			for _, name := range names {
				repo := cfg.Repos[name]

				vcsLabel := repo.ActiveBackend()

				rows = append(rows, []string{name, vcsLabel, repo.Path})
			}

			widths := []int{nameWidth, vcsWidth, pathWidth}
			header := []string{NameLabel, "VCS", "PATH"}

			_, _ = os.Stdout.WriteString(ui.RenderTable(
				header, rows, ui.EffectiveWidths(header, rows, widths),
			))

			return nil
		},
	}
}

const (
	cmdNameRepo   = "repo"
	cmdNameRename = "rename"
)

func repoRenameCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameRename,
		Usage:     "rename a repository",
		ArgsUsage: "<old-name> <new-name>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 2 { //nolint:mnd
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
