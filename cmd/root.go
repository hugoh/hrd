// Package cmd wires together all mgr subcommands into a urfave/cli app.
package cmd

import (
	"github.com/hugoh/hrd/internal/config"
	"github.com/urfave/cli/v3"
)

const cmdNameHRD = "hrd"

var version = "dev"

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
		Commands: []*cli.Command{
			// Repo and group management
			repoCommands(&cfgPath),
			groupCommands(&cfgPath),
			contextCommands(&cfgPath),

			// Status
			lsCmd(&cfgPath),
			statusCmd(&cfgPath),
			diffCmd(&cfgPath),
			logCmd(&cfgPath),

			// VCS dispatch
			gitCmd(&cfgPath),
			jjCmd(&cfgPath),
			shellCmd(&cfgPath),
		},
	}
}
