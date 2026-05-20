// Package cmd wires together all mgr subcommands into a urfave/cli app.
package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/urfave/cli/v3"
)

var errUnknownCommand = errors.New("unknown command")

const cmdNameHRD = "hrd"

var version = "dev"

//nolint:gochecknoglobals // swapped in tests to simulate TUI failures
var runTUI = tui.Run

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
			if args := cmd.Args(); args != nil && args.Len() > 0 {
				return fmt.Errorf("%w: %s", errUnknownCommand, args.First())
			}

			return runTUI(ctx, tui.Options{
				ConfigPath: cfgPath,
			})
		},
		Commands: []*cli.Command{
			// Repo and group management
			repoCommands(&cfgPath),
			groupCommands(&cfgPath),

			// Status
			lsCmd(&cfgPath),
			statusCmd(&cfgPath),
			diffCmd(&cfgPath),
			logCmd(&cfgPath),
			fetchCmd(&cfgPath),
			pullCmd(&cfgPath),
			pushCmd(&cfgPath),

			// VCS dispatch
			gitCmd(&cfgPath),
			jjCmd(&cfgPath),
			shellCmd(&cfgPath),

			// TUI
			tuiCmd(&cfgPath),
		},
	}
}
