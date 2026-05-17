package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

const (
	cmdNameGroup   = "group"
	cmdNameAdd     = "add"
	cmdNameContext = "context"
	cmdNameSet     = "set"
	cmdNameShow    = "show"
)

var (
	errGroupAddUsage   = errors.New("usage: group add <name> <repo>")
	errGroupRmUsage    = errors.New("usage: group rm <name>")
	errContextSetUsage = errors.New("usage: context set <group>")
	errUnknownGroup    = errors.New("unknown group")
)

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

func loadGroupAndCheck(cfgPath, name string) (config.Config, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Config{}, fmt.Errorf("loading config: %w", err)
	}

	name = stripGroupPrefix(name)

	if _, ok := cfg.Groups[name]; !ok {
		return config.Config{}, fmt.Errorf("%w %q", errUnknownGroup, displayGroup(name))
	}

	return cfg, nil
}

// groupCommands returns the `group` subcommand with its children.
func groupCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameGroup,
		Usage: "manage repo groups",
		Commands: []*cli.Command{
			groupAddCmd(cfgPath),
			groupRemoveCmd(cfgPath),
			groupListCmd(cfgPath),
		},
	}
}

const minGroupAddArgs = 2

func groupAddCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameAdd,
		Usage:     "create or append to a group",
		ArgsUsage: "<name> <repo>...",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() > 0 {
				reposOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < minGroupAddArgs {
				return errGroupAddUsage
			}

			name := stripGroupPrefix(cmd.Args().Get(0))
			repos := cmd.Args().Slice()[1:]

			if hasDuplicateRepos(repos) {
				ui.Warn("duplicate repo names in group, ignoring extras")
			}

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := cfg.AddGroup(name, repos); err != nil {
				return fmt.Errorf("adding group: %w", err)
			}

			ui.Success("added %s to group %s", strings.Join(repos, ", "), displayGroup(name))

			return config.Save(*cfgPath, cfg)
		},
	}
}

func groupRemoveCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a group",
		ArgsUsage: "<name>",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				groupsOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				return errGroupRmUsage
			}

			name := stripGroupPrefix(cmd.Args().Get(0))

			cfg, err := loadGroupAndCheck(*cfgPath, name)
			if err != nil {
				return err
			}

			// Clear context if it points to this group.
			if cfg.Context.Current == name {
				cfg.Context.Current = ""
			}

			delete(cfg.Groups, name)
			ui.Success("removed group %q", displayGroup(name))

			return config.Save(*cfgPath, cfg)
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
			return fmt.Errorf("loading config: %w", err)
		}

		if name := stripGroupPrefix(cmd.Args().First()); name != "" {
			group, ok := cfg.Groups[name]
			if !ok {
				return fmt.Errorf("%w %q", errUnknownGroup, displayGroup(name))
			}

			for _, repo := range group.Repos {
				ui.Outf(repo)
			}

			return nil
		}

		if len(cfg.Groups) == 0 {
			ui.Outf("no groups defined")

			return nil
		}

		return renderGroupTable(cfg)
	}
}

func renderGroupTable(cfg config.Config) error {
	const (
		groupWidth    = 20
		reposMinWidth = 20
	)

	termWidth := ui.GetTermWidth()
	reposWidth := ui.ComputeRemainderWidth(termWidth, reposMinWidth, 1, groupWidth)

	rows := make([][]string, 0, len(cfg.Groups))
	for name, group := range cfg.Groups {
		active := ""
		if cfg.Context.Current == name {
			active = ui.ColorSprint("green", "●")
		}

		rows = append(rows, []string{displayGroup(name), strings.Join(group.Repos, ", "), active})
	}

	widths := []int{groupWidth, reposWidth, 1}
	header := []string{"GROUP", "REPOS", ""}

	_, _ = os.Stdout.WriteString(ui.RenderTable(
		header, rows, ui.EffectiveWidths(header, rows, widths),
	))

	return nil
}

// hasDuplicateRepos reports whether the slice contains any duplicate entries.
func hasDuplicateRepos(repos []string) bool {
	seen := make(map[string]bool, len(repos))

	for _, r := range repos {
		if seen[r] {
			return true
		}

		seen[r] = true
	}

	return false
}

// contextCommands returns the `context` subcommand.
func contextCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameContext,
		Usage: "set or clear the active group scope",
		Commands: []*cli.Command{
			contextSetCmd(cfgPath),
			contextClearCmd(cfgPath),
			contextShowCmd(cfgPath),
		},
	}
}

func contextSetCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      cmdNameSet,
		Usage:     "set active context to a group",
		ArgsUsage: "<group>",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				groupsOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				return errContextSetUsage
			}

			name := stripGroupPrefix(cmd.Args().Get(0))

			cfg, err := loadGroupAndCheck(*cfgPath, name)
			if err != nil {
				return err
			}

			cfg.Context.Current = name
			ui.Success("context set to %q", displayGroup(name))

			return config.Save(*cfgPath, cfg)
		},
	}
}

func contextClearCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "clear",
		Usage: "clear active context (all repos)",
		Action: func(_ context.Context, _ *cli.Command) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			cfg.Context.Current = ""

			ui.Outf("context cleared")

			return config.Save(*cfgPath, cfg)
		},
	}
}

func contextShowCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameShow,
		Usage: "show active context",
		Action: func(_ context.Context, _ *cli.Command) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.Context.Current == "" {
				ui.Outf("all repos")
			} else {
				ui.Outf("context: %s", displayGroup(cfg.Context.Current))
			}

			return nil
		},
	}
}
