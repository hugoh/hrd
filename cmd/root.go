// Package cmd wires together all mgr subcommands into a urfave/cli app.
package cmd

import (
	"context"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/urfave/cli/v3"
)

const cmdNameHRD = "hrd"

const staticCmdCount = 11

var version = "dev"

//nolint:gochecknoglobals // swapped in tests to simulate TUI failures
var runTUI = tui.Run

func buildCommands(cfgPath *string) []*cli.Command {
	n := len(backend.Names())

	cmds := make([]*cli.Command, 0, staticCmdCount+n)

	cmds = append(cmds,
		repoCommands(cfgPath),
		groupCommands(cfgPath),

		lsCmd(cfgPath),
		statusCmd(cfgPath),
		diffCmd(cfgPath),
		logCmd(cfgPath),
		fetchCmd(cfgPath),
		pullCmd(cfgPath),
		pushCmd(cfgPath),

		shellCmd(cfgPath),

		tuiCmd(cfgPath),
	)

	for _, name := range backend.Names() {
		cmds = append(cmds, vcsCmd(cfgPath, name))
	}

	return cmds
}

// NewApp builds and returns the root CLI application.
func NewApp() *cli.Command {
	cfgPath := config.DefaultPath()

	return &cli.Command{
		Name:                  cmdNameHRD,
		Usage:                 "manage multiple git and jj repositories",
		Version:               version,
		EnableShellCompletion: true,
		ConfigureShellCompletionCommand: func(cmd *cli.Command) {
			cmd.Hidden = false
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "config",
				Aliases:     []string{"c"},
				Usage:       "path to config file",
				Value:       cfgPath,
				Destination: &cfgPath,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			var args []string

			if a := cmd.Args(); a != nil {
				args = a.Slice()
			}

			return runTUI(ctx, tui.Options{
				ConfigPath: cfgPath,
				Repos:      args,
			})
		},
		Commands: buildCommands(&cfgPath),
	}
}
