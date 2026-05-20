package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

const (
	cmdNameGroup = "group"
	cmdNameTag   = "tag"
	cmdNameUntag = "untag"
)

var (
	errUnknownGroup   = errors.New("unknown group")
	errRepoTagsUsage  = errors.New("usage: repo tag <repo> <tag>")
	errRepoUntagUsage = errors.New("usage: repo untag <repo> <tag>")
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

// tagCommands returns the `group` subcommand (read-only) and the `repo tag`
// / `repo untag` subcommands.
func tagCommands(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:  cmdNameGroup,
		Usage: "list repo groups (derived from repo tags)",
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
	for name, group := range cfg.Groups {
		ui.Outf(displayGroup(name))
		ui.Outf("  " + strings.Join(group.Repos, ", "))
	}

	return nil
}

func repoTagCmd(cfgPath *string) *cli.Command {
	return tagActionCmd(cfgPath, cmdNameTag, "add a tag to a repo", errRepoTagsUsage,
		func(cfg *config.Config, name, tag string) {
			cfg.TagRepo(name, tag)
			ui.Success("tagged %q with %q", name, tag)
		})
}

func repoUntagCmd(cfgPath *string) *cli.Command {
	return tagActionCmd(cfgPath, cmdNameUntag, "remove a tag from a repo", errRepoUntagUsage,
		func(cfg *config.Config, name, tag string) {
			cfg.UntagRepo(name, tag)
			ui.Success("removed tag %q from %q", tag, name)
		})
}

func tagActionCmd(
	cfgPath *string,
	name, usage string,
	usageErr error,
	act func(*config.Config, string, string),
) *cli.Command {
	return &cli.Command{
		Name:      name,
		Usage:     usage,
		ArgsUsage: "<repo> <tag>",
		ShellComplete: func(ctx context.Context, cmd *cli.Command) {
			if cmd.Args().Len() == 0 {
				reposOnlyCompleter(cfgPath)(ctx, cmd)
			}
		},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 2 { //nolint:mnd
				return usageErr
			}

			repoName, tag := cmd.Args().Get(0), cmd.Args().Get(1)

			cfg, err := config.Load(*cfgPath)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			if _, ok := cfg.Repos[repoName]; !ok {
				return fmt.Errorf("%w %q", errUnknownRepo, repoName)
			}

			act(&cfg, repoName, tag)

			return config.Save(*cfgPath, cfg)
		},
	}
}
