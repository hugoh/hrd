package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/urfave/cli/v3"
)

const (
	groupWidth     = 20 // width for group column
	reposMinWidth  = 20 // minimum width for repos column
	groupColNumber = 2  // column number for repos
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

func loadGroupAndCheck(cfgPath, name string) (config.Config, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return config.Config{}, fmt.Errorf("loading config: %w", err)
	}

	if _, ok := cfg.Groups[name]; !ok {
		return config.Config{}, fmt.Errorf("%w %q", errUnknownGroup, name)
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
		Usage:     "create or replace a group",
		ArgsUsage: "<name> <repo>...",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() < minGroupAddArgs {
				return errGroupAddUsage
			}

			name := cmd.Args().Get(0)
			repos := cmd.Args().Slice()[1:]

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if err := cfg.AddGroup(name, repos); err != nil {
				return fmt.Errorf("adding group: %w", err)
			}

			ui.Info("group %q: %s", name, strings.Join(repos, ", "))

			return config.Save(*cfgPath, cfg)
		},
	}
}

func groupRemoveCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "rm",
		Usage:     "remove a group",
		ArgsUsage: "<name>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				return errGroupRmUsage
			}

			name := cmd.Args().Get(0)

			cfg, err := loadGroupAndCheck(*cfgPath, name)
			if err != nil {
				return err
			}

			// Clear context if it points to this group.
			if cfg.Context.Current == name {
				cfg.Context.Current = ""
			}

			delete(cfg.Groups, name)
			ui.Success("removed group %q", name)

			return config.Save(*cfgPath, cfg)
		},
	}
}

func groupListCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:      "ls",
		Usage:     "list groups",
		ArgsUsage: "[name]",
		Action:    listGroupsAction(cfgPath),
	}
}

func listGroupsAction(cfgPath *string) func(_ context.Context, cmd *cli.Command) error {
	return func(_ context.Context, cmd *cli.Command) error {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if name := cmd.Args().First(); name != "" {
			group, ok := cfg.Groups[name]
			if !ok {
				return fmt.Errorf("%w %q", errUnknownGroup, name)
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
	tbl := ui.NewTable()
	tbl.AppendHeader(table.Row{"GROUP", "REPOS", ""})

	termWidth := ui.GetTermWidth()

	reposWidth := ui.ComputeRemainderWidth(termWidth, reposMinWidth, 1, groupWidth)

	tbl.SetColumnConfigs([]table.ColumnConfig{
		{Number: 1, AutoMerge: true, WidthMax: groupWidth},
		{Number: groupColNumber, WidthMax: reposWidth, WidthMaxEnforcer: ui.Wrap},
	})

	for name, group := range cfg.Groups {
		active := ""
		if cfg.Context.Current == name {
			active = text.Colors{text.FgGreen}.Sprint("●")
		}

		tbl.AppendRow(table.Row{name, strings.Join(group.Repos, ", "), active})
	}

	tbl.Render()

	return nil
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
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				return errContextSetUsage
			}

			name := cmd.Args().Get(0)

			cfg, err := loadGroupAndCheck(*cfgPath, name)
			if err != nil {
				return err
			}

			cfg.Context.Current = name
			ui.Success("context set to %q", name)

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
				ui.Outf("context: %s", cfg.Context.Current)
			}

			return nil
		},
	}
}
