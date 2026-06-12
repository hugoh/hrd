// Package cmd wires together all mgr subcommands into a urfave/cli app.
package cmd

import (
	"context"
	"slices"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/urfave/cli/v3"
)

const cmdNameHRD = "hrd"

const staticCmdCount = 12

// completionFlag is urfave/cli's hidden shell-completion trigger.
const completionFlag = "--generate-shell-completion"

var version = "dev"

//nolint:gochecknoglobals // swapped in tests to simulate TUI failures
var runTUI = tui.Run

// SplitDashTail splits raw argv at the first standalone "--": the head is
// passed to the CLI parser, the tail is delivered verbatim to commands
// that spawn subprocesses (git, jj, shell). urfave/cli strips the
// separator during parsing, which would let repo names inside the command
// be mistaken for scope; splitting before parsing keeps the tail
// untouched. The tail is nil when there is no separator. Completion
// invocations (last arg is the hidden completion flag) pass through
// unchanged so completion after "--" keeps working.
func SplitDashTail(args []string) ([]string, []string) {
	if len(args) > 0 && args[len(args)-1] == completionFlag {
		return args, nil
	}

	if i := slices.Index(args, "--"); i >= 0 {
		return args[:i], args[i+1:]
	}

	return args, nil
}

func buildCommands(cfgPath *string, dashTail []string) []*cli.Command {
	n := len(backend.Names())

	cmds := make([]*cli.Command, 0, staticCmdCount+n)

	cmds = append(cmds,
		repoCommands(cfgPath),
		groupCommands(cfgPath),

		lsCmd(cfgPath, dashTail),
		llCmd(cfgPath, dashTail),
		statusCmd(cfgPath, dashTail),
		diffCmd(cfgPath, dashTail),
		logCmd(cfgPath, dashTail),
		fetchCmd(cfgPath, dashTail),
		pullCmd(cfgPath, dashTail),
		pushCmd(cfgPath, dashTail),

		shellCmd(cfgPath, dashTail),

		tuiCmd(cfgPath),
	)

	for _, name := range backend.Names() {
		cmds = append(cmds, vcsCmd(cfgPath, name, dashTail))
	}

	return cmds
}

// NewApp builds the root CLI application without a "--" tail. Callers
// that execute user argv should use NewAppWithTail with SplitDashTail.
func NewApp() *cli.Command {
	return NewAppWithTail(nil)
}

// NewAppWithTail builds the root CLI application. dashTail is the verbatim
// argv tail after the first "--" (see SplitDashTail), nil when absent.
func NewAppWithTail(dashTail []string) *cli.Command {
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
		Commands: buildCommands(&cfgPath, dashTail),
	}
}
