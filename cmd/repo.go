package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

var (
	errAtLeastOnePath   = errors.New("at least one path required")
	errNameSingleRepo   = errors.New("--name can only be used when adding a single repo")
	errAtLeastOneName   = errors.New("at least one repo name required")
	errRepoRenameUsage  = errors.New("usage: repo rename <old> <new>")
	errUnknownRepo      = errors.New("unknown repo")
	errRepoExists       = errors.New("repo already exists")
	errRepoNoVCS        = errors.New("no VCS detected")
	errUnknownGroup     = errors.New("unknown group")
	errRepoGroupUsage   = errors.New("usage: repo group <repo> <group>")
	errRepoUngroupUsage = errors.New("usage: repo ungroup <repo> <group>")
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
			repoGroupCmd(cfgPath),
			repoUngroupCmd(cfgPath),
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
			return fmt.Errorf("repo add: %w", err)
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
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			reposOnlyCompleter(cfgPath)(ctx, cmd)
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() == 0 {
				return errAtLeastOneName
			}

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("repo rm: %w", err)
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
				return fmt.Errorf("repo ls: %w", err)
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

				slices.Sort(names)
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

			_, _ = fmt.Fprint(os.Stdout, ui.RenderTable(
				header, rows, ui.EffectiveWidths(header, rows, widths),
			))

			return nil
		},
	}
}

const (
	cmdNameRepo    = "repo"
	cmdNameAdd     = "add"
	cmdNameRename  = "rename"
	cmdNameGroup   = "group"
	cmdNameUngroup = "ungroup"
)

func repoRenameCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameRename,
		Usage:     "rename a repository",
		ArgsUsage: "<old-name> <new-name>",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				reposOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 2 { //nolint:mnd // expects old and new name
				return errRepoRenameUsage
			}

			oldName, newName := cmd.Args().Get(0), cmd.Args().Get(1)

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("repo rename: %w", err)
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

			ui.Success("renamed %q → %q", oldName, newName)

			return config.Save(*cfgPath, cfg)
		},
	}
}

// stripGroupPrefix removes a leading '@' from a group name if present.
// This lets users type @work or work interchangeably on the CLI.
func stripGroupPrefix(name string) string {
	return strings.TrimPrefix(name, "@")
}

// displayGroup adds a '@' prefix for display purposes so group names
// are visually distinguishable from repo names in output.
func displayGroup(name string) string {
	if !strings.HasPrefix(name, "@") {
		return "@" + name
	}

	return name
}

func repoGroupCmd(cfgPath *string) *cli.Command {
	return groupActionCmd(cfgPath, cmdNameGroup, "add a group to a repo", errRepoGroupUsage,
		func(cfg *config.Config, name, group string) {
			cfg.AddRepoToGroup(name, group)
			ui.Success("added %q to group %q", name, group)
		})
}

func repoUngroupCmd(cfgPath *string) *cli.Command {
	return groupActionCmd(
		cfgPath,
		cmdNameUngroup,
		"remove a group from a repo",
		errRepoUngroupUsage,
		func(cfg *config.Config, name, group string) {
			cfg.RemoveRepoFromGroup(name, group)
			ui.Success("removed %q from group %q", name, group)
		},
	)
}

func groupActionCmd(
	cfgPath *string,
	name, usage string,
	usageErr error,
	act func(*config.Config, string, string),
) *cli.Command {
	return &cli.Command{
		Name:      name,
		Usage:     usage,
		ArgsUsage: "<repo> <group>",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				reposOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 2 { //nolint:mnd // expects repo and group name
				return usageErr
			}

			repoName, group := cmd.Args().Get(0), cmd.Args().Get(1)

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}

			if _, ok := cfg.Repos[repoName]; !ok {
				return fmt.Errorf("%w %q", errUnknownRepo, repoName)
			}

			act(&cfg, repoName, group)

			return config.Save(*cfgPath, cfg)
		},
	}
}

// groupCommands returns the `group` subcommand (read-only).
func groupCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameGroup,
		Usage: "list repo groups",
		Commands: []*cli.Command{
			groupListCmd(cfgPath),
		},
	}
}

func groupListCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "list groups",
		ArgsUsage: "[name]",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				groupsOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: listGroupsAction(cfgPath),
	}
}

func listGroupsAction(cfgPath *string) func(_ context.Context, cmd *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("group ls: %w", err)
		}

		if name := stripGroupPrefix(cmd.Args().First()); name != "" {
			group, ok := cfg.Groups[name]
			if !ok {
				return fmt.Errorf("%w %q", errUnknownGroup, displayGroup(name))
			}

			for _, repo := range group.Repos {
				ui.Out(repo)
			}

			return nil
		}

		if len(cfg.Groups) == 0 {
			ui.Out("no groups defined")

			return nil
		}

		return renderGroupTable(cfg)
	}
}

func renderGroupTable(cfg config.Config) error {
	for name, group := range cfg.Groups {
		ui.Out(displayGroup(name))
		ui.Out("  " + strings.Join(group.Repos, ", "))
	}

	return nil
}
