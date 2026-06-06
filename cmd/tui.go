package cmd

import (
	"context"

	"github.com/hugoh/hrd/internal/tui"
	"github.com/urfave/cli/v3"
)

// tuiCmd returns the `tui` subcommand.
func tuiCmd(cfgPath *string) *cli.Command {
	return &cli.Command{
		Name:    "tui",
		Aliases: []string{"i"},
		Usage:   "interactive terminal UI for browsing and running commands across repos",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    cmdNameGroup,
				Aliases: []string{"g"},
				Usage:   "initial group filter (e.g. @work)",
			},
			&cli.StringSliceFlag{
				Name:    cmdReposFlag,
				Aliases: []string{"r"},
				Usage:   "initial repo selection",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			repos := cmd.StringSlice(cmdReposFlag)

			if a := cmd.Args(); a != nil {
				repos = append(repos, a.Slice()...)
			}

			return tui.Run(ctx, tui.Options{
				ConfigPath: *cfgPath,
				Group:      cmd.String("group"),
				Repos:      repos,
			})
		},
	}
}
