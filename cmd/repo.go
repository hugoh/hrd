package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
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
func repoCommands(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameRepo,
		Short: "manage tracked repositories",
	}
	cmd.AddCommand(
		repoAddCmd(cfgPath),
		repoScanCmd(cfgPath),
		repoRemoveCmd(cfgPath),
		repoListCmd(cfgPath),
		repoRenameCmd(cfgPath),
		repoGroupCmd(cfgPath),
		repoUngroupCmd(cfgPath),
	)

	return cmd
}

func repoAddCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:               cmdNameAdd + " <path>...",
		Short:             "add one or more repositories",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		RunE:              repoAddAction(cfgPath),
	}
	cmd.Flags().StringP("name", "n", "", "explicit name (only valid when adding a single repo)")
	cmd.Flags().StringP(cmdNameGroup, "g", "", "add the repo(s) to this group")

	return cmd
}

func repoAddAction(cfgPath *string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return errAtLeastOnePath
		}

		name := flagString(cmd, "name")

		if name != "" && len(args) > 1 {
			return errNameSingleRepo
		}

		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("repo add: %w", err)
		}

		group := stripGroupPrefix(flagString(cmd, cmdNameGroup))

		for _, arg := range args {
			if err := addRepo(&cfg, arg, name, group); err != nil {
				return err
			}
		}

		return config.Save(*cfgPath, cfg)
	}
}

// addRepo validates and registers a single repo path in cfg. An empty
// explicitName derives the name from the directory base name.
func addRepo(cfg *config.Config, path, explicitName, group string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolving %q: %w", path, err)
	}

	if _, err := backend.Detect(abs); err != nil {
		return fmt.Errorf("%s: %w", abs, errRepoNoVCS)
	}

	name := explicitName
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

	if group != "" {
		cfg.AddRepoToGroup(name, group)
	}

	if group != "" {
		ui.Success("added %s as %q in group %s", abs, name, group)
	} else {
		ui.Success("added %s as %q", abs, name)
	}

	return nil
}

func repoRemoveCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:               "rm <name>...",
		Short:             "remove one or more repositories",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: reposOnlyCompleter(cfgPath),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) == 0 {
				return errAtLeastOneName
			}

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("repo rm: %w", err)
			}

			for _, name := range args {
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

func repoListCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ls",
		Short: "list tracked repositories",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("repo ls: %w", err)
			}

			names := make([]string, 0, len(cfg.Repos))

			if g := stripGroupPrefix(flagString(cmd, "group")); g != "" {
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
	cmd.Flags().StringP(cmdNameGroup, "g", "", "filter to repos in a group")

	return cmd
}

const (
	cmdNameRepo    = "repo"
	cmdNameAdd     = "add"
	cmdNameRename  = "rename"
	cmdNameGroup   = "group"
	cmdNameUngroup = "ungroup"
)

// completeFirstArgWithRepos completes the first (and only the first)
// positional arg with repo names.
func completeFirstArgWithRepos(cfgPath *string) cobraCompleter {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return reposOnlyCompleter(cfgPath)(cmd, args, toComplete)
		}

		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func repoRenameCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:               cmdNameRename + " <old-name> <new-name>",
		Short:             "rename a repository",
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeFirstArgWithRepos(cfgPath),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 { //nolint:mnd // expects old and new name
				return errRepoRenameUsage
			}

			oldName, newName := args[0], args[1]

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

func repoGroupCmd(cfgPath *string) *cobra.Command {
	return groupActionCmd(cfgPath, cmdNameGroup, "add a group to a repo", errRepoGroupUsage,
		func(cfg *config.Config, name, group string) {
			cfg.AddRepoToGroup(name, group)
			ui.Success("added %q to group %q", name, group)
		})
}

func repoUngroupCmd(cfgPath *string) *cobra.Command {
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
) *cobra.Command {
	return &cobra.Command{
		Use:               name + " <repo> <group>",
		Short:             usage,
		Args:              cobra.ArbitraryArgs,
		ValidArgsFunction: completeFirstArgWithRepos(cfgPath),
		RunE: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 { //nolint:mnd // expects repo and group name
				return usageErr
			}

			repoName, group := args[0], args[1]

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
func groupCommands(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   cmdNameGroup,
		Short: "list repo groups",
	}
	cmd.AddCommand(groupListCmd(cfgPath))

	return cmd
}

func groupListCmd(cfgPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "ls [name]",
		Short: "list groups",
		Args:  cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return groupsOnlyCompleter(cfgPath)(cmd, args, toComplete)
			}

			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: listGroupsAction(cfgPath),
	}
}

func listGroupsAction(cfgPath *string) func(cmd *cobra.Command, args []string) error {
	return func(_ *cobra.Command, args []string) error {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("group ls: %w", err)
		}

		if name := stripGroupPrefix(firstArg(args)); name != "" {
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
