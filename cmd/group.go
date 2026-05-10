package cmd

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/hugoh/hrd/internal/config"
	"github.com/pterm/pterm"
	"github.com/urfave/cli/v3"
)

var (
	errGroupAddUsage   = errors.New("usage: group add <name> <repo>")
	errGroupRmUsage    = errors.New("usage: group rm <name>")
	errContextSetUsage = errors.New("usage: context set <group>")
	errUnknownGroup    = errors.New("unknown group")
)

// groupCommands returns the `group` subcommand with its children.
func groupCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "group",
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
		Name:      "add",
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

			pterm.Info.Printf("group %q: %s\n", name, strings.Join(repos, ", "))

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

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, ok := cfg.Groups[name]; !ok {
				return fmt.Errorf("%w %q", errUnknownGroup, name)
			}
			// Clear context if it points to this group.
			if cfg.Context.Current == name {
				cfg.Context.Current = ""
			}

			delete(cfg.Groups, name)
			pterm.Success.Printf("removed group %q\n", name)

			return config.Save(*cfgPath, cfg)
		},
	}
}

func groupListCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "ls",
		Usage: "list groups",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "panel",
				Aliases: []string{"p"},
				Usage:   "render groups as panels",
			},
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

		if len(cfg.Groups) == 0 {
			pterm.Println("no groups defined")

			return nil
		}

		if cmd.Bool("panel") {
			return renderGroupPanels(cfg)
		}

		return renderGroupTable(cfg)
	}
}

func renderGroupPanels(cfg config.Config) error {
	names := make([]string, 0, len(cfg.Groups))
	for name := range cfg.Groups {
		names = append(names, name)
	}

	sort.Strings(names)

	panels := make(pterm.Panels, 0, len(names))

	for _, name := range names {
		group := cfg.Groups[name]
		content := strings.Join(group.Repos, "\n")

		label := name
		if cfg.Context.Current == name {
			label += " ●"
		}

		box := pterm.DefaultBox.WithTitle(label).Sprint(content)
		panels = append(panels, []pterm.Panel{{Data: box}})
	}

	_ = pterm.DefaultPanel.WithPanels(panels).Render()

	return nil
}

func renderGroupTable(cfg config.Config) error {
	tableData := make(pterm.TableData, 0, 1+len(cfg.Groups))
	tableData = append(tableData, []string{"GROUP", "REPOS", ""})

	for name, group := range cfg.Groups {
		active := ""
		if cfg.Context.Current == name {
			active = "●"
		}

		tableData = append(tableData, []string{name, strings.Join(group.Repos, ", "), active})
	}

	_ = pterm.DefaultTable.WithHasHeader(true).WithData(tableData).Render()

	return nil
}

// contextCommands returns the `context` subcommand.
func contextCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "context",
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
		Name:      "set",
		Usage:     "set active context to a group",
		ArgsUsage: "<group>",
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				return errContextSetUsage
			}

			name := cmd.Args().Get(0)

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, ok := cfg.Groups[name]; !ok {
				return fmt.Errorf("%w %q", errUnknownGroup, name)
			}

			cfg.Context.Current = name
			pterm.Success.Printf("context set to %q\n", name)

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

			pterm.Println("context cleared")

			return config.Save(*cfgPath, cfg)
		},
	}
}

func contextShowCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  "show",
		Usage: "show active context",
		Action: func(_ context.Context, _ *cli.Command) error {
			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if cfg.Context.Current == "" {
				pterm.Println("context: (all repos)")
			} else {
				pterm.Printf("context: %s\n", cfg.Context.Current)
			}

			return nil
		},
	}
}
