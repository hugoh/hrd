// Package cmd wires together all mgr subcommands into a urfave/cli app.
package cmd

import (
	"context"
	"slices"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/urfave/cli/v3"
)

const cmdNameHRD = "hrd"

const staticCmdCount = 12

const flagConfig = "config"

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

func buildCommands(cfgPath *string, dashTail []string, aliases map[string]string) []*cli.Command {
	n := len(backend.Names())

	cmds := make([]*cli.Command, 0, staticCmdCount+n+len(aliases))

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

	if len(aliases) > 0 {
		taken := map[string]bool{"help": true, "h": true, "completion": true}
		for _, c := range cmds {
			taken[c.Name] = true

			for _, a := range c.Aliases {
				taken[a] = true
			}
		}

		cmds = append(cmds, aliasCommands(cfgPath, aliases, taken, dashTail)...)
	}

	return cmds
}

// NewApp builds the root CLI application without a "--" tail or config
// aliases. Callers that execute user argv should use NewAppForArgs.
func NewApp() *cli.Command {
	return newApp(nil, nil)
}

// NewAppForArgs prepares the app for raw argv: it splits off the verbatim
// "--" tail (see SplitDashTail) and registers config aliases as top-level
// commands. The alias load honors -c/--config and is best-effort — config
// errors are reported later by the command actions. Returns the app and
// the argv head to pass to Run.
func NewAppForArgs(args []string) (*cli.Command, []string) {
	head, tail := SplitDashTail(args)

	return newApp(tail, loadAliases(configPathFromArgs(head))), head
}

// configPathFromArgs pre-scans argv for -c/--config so aliases can be
// registered before urfave/cli parses flags.
func configPathFromArgs(args []string) string {
	for i, arg := range args {
		switch {
		case arg == "--"+flagConfig || arg == "-c":
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(arg, "--"+flagConfig+"="):
			return strings.TrimPrefix(arg, "--"+flagConfig+"=")
		case strings.HasPrefix(arg, "-c="):
			return strings.TrimPrefix(arg, "-c=")
		}
	}

	return config.DefaultPath()
}

func loadAliases(cfgPath string) map[string]string {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil
	}

	return cfg.Aliases
}

func newApp(dashTail []string, aliases map[string]string) *cli.Command {
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
				Name:        flagConfig,
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
		Commands: buildCommands(&cfgPath, dashTail, aliases),
	}
}
