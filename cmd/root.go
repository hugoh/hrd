// Package cmd wires together all hrd subcommands into a Cobra app.
package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/tui"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
)

const cmdNameHRD = "hrd"

const staticCmdCount = 12

const (
	flagConfig      = "config"
	flagConfigShort = "c"
)

var version = "dev"

//nolint:gochecknoglobals // swapped in tests to simulate TUI failures
var runTUI = tui.Run

// Command groups shown in `hrd --help`, in display order.
const (
	groupRepos  = "repos"
	groupStatus = "status"
	groupSync   = "sync"
	groupExec   = "exec"
)

//nolint:gochecknoglobals // static help-output metadata, read-only after init
var commandGroups = []*cobra.Group{
	{ID: groupRepos, Title: "Repository management:"},
	{ID: groupStatus, Title: "Inspect:"},
	{ID: groupSync, Title: "Remote sync:"},
	{ID: groupExec, Title: "Run commands:"},
}

func buildCommands(cfgPath *string, aliases map[string]string) []*cobra.Command {
	n := len(backend.Names())

	cmds := make([]*cobra.Command, 0, staticCmdCount+n+len(aliases))

	cmds = append(cmds,
		group(groupRepos, repoCommands(cfgPath)),
		group(groupRepos, groupCommands(cfgPath)),

		group(groupStatus, lsCmd(cfgPath)),
		group(groupStatus, llCmd(cfgPath)),
		group(groupStatus, statusCmd(cfgPath)),
		group(groupStatus, diffCmd(cfgPath)),
		group(groupStatus, logCmd(cfgPath)),

		group(groupSync, fetchCmd(cfgPath)),
		group(groupSync, pullCmd(cfgPath)),
		group(groupSync, pushCmd(cfgPath)),

		group(groupExec, shellCmd(cfgPath)),
		group(groupExec, tuiCmd(cfgPath)),
	)

	for _, name := range backend.Names() {
		cmds = append(cmds, group(groupExec, vcsCmd(cfgPath, name)))
	}

	if len(aliases) > 0 {
		taken := map[string]bool{"help": true, "completion": true}
		for _, c := range cmds {
			taken[c.Name()] = true

			for _, a := range c.Aliases {
				taken[a] = true
			}
		}

		cmds = append(cmds, aliasCommands(cfgPath, aliases, taken)...)
	}

	return cmds
}

// group assigns cmd to the given help group and returns it, so it can be
// used inline in buildCommands' append call.
func group(id string, cmd *cobra.Command) *cobra.Command {
	cmd.GroupID = id

	return cmd
}

// NewApp builds the root CLI application without config aliases. Callers
// that execute user argv should use NewAppForArgs.
func NewApp() *cobra.Command {
	return buildRootCmd(nil)
}

// NewAppForArgs prepares the app for raw argv: it registers config aliases
// as top-level commands before argv is parsed. This ordering matters because
// alias commands must already exist in the tree for the parser to route to
// them; parsing itself folds pre- and post-"--" positionals into one args
// slice per command (see splitScopeArgs for the commands that distinguish
// the two sides). The alias load honors -c/--config and is best-effort —
// config errors are reported later by the command actions.
func NewAppForArgs(args []string) *cobra.Command {
	return buildRootCmd(loadAliases(configPathFromArgs(args)))
}

// RunApp executes app with argv (including the leading program name, as in
// os.Args) under ctx.
func RunApp(ctx context.Context, app *cobra.Command, argv []string) error {
	var args []string
	if len(argv) > 1 {
		args = argv[1:]
	}

	app.SetArgs(args)

	return app.ExecuteContext(ctx) //nolint:wrapcheck
}

// configPathFromArgs pre-scans argv for -c/--config so aliases can be
// registered before the flag parser runs. The scan stops at the first
// literal "--": anything after it belongs to a passthrough subprocess
// command (e.g. "hrd git repo1 -- commit -c user.name=x") and must never
// be mistaken for hrd's own -c/--config flag.
func configPathFromArgs(args []string) string {
	for i, arg := range args {
		if arg == "--" {
			break
		}

		switch {
		case arg == "--"+flagConfig || arg == "-"+flagConfigShort:
			if i+1 < len(args) {
				return args[i+1]
			}
		case strings.HasPrefix(arg, "--"+flagConfig+"="):
			return strings.TrimPrefix(arg, "--"+flagConfig+"=")
		case strings.HasPrefix(arg, "-"+flagConfigShort+"="):
			return strings.TrimPrefix(arg, "-"+flagConfigShort+"=")
		}
	}

	return config.DefaultPath()
}

// loadConfig loads the config at *cfgPath, wrapping any error with label so
// callers get a consistent "<label>: <cause>" message instead of each
// reimplementing the same wrap.
func loadConfig(cfgPath *string, label string) (config.Config, error) {
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", label, err)
	}

	return cfg, nil
}

// loadResolvedConfig is loadConfig plus a live merge of repos discovered
// under the config's Roots. Use it for read-only commands (dispatch, repo
// ls) so newly appeared repos show up without a `repo scan add` rerun.
// Callers that Save the config back must use loadConfig instead — saving a
// resolved config would materialize discovered repos into the file.
func loadResolvedConfig(cfgPath *string, label string) (config.Config, error) {
	cfg, warnings, err := config.LoadResolved(*cfgPath)
	if err != nil {
		return config.Config{}, fmt.Errorf("%s: %w", label, err)
	}

	for _, w := range warnings {
		ui.Warnf("%v", w)
	}

	return cfg, nil
}

func loadAliases(cfgPath string) map[string]string {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil
	}

	return cfg.Aliases
}

func buildRootCmd(aliases map[string]string) *cobra.Command {
	cfgPath := config.DefaultPath()

	root := &cobra.Command{
		Use:   cmdNameHRD,
		Short: "manage multiple git and jj repositories",
		Long: `hrd manages a set of git and jj repositories tracked in a config file,
letting you run status checks and VCS operations across many of them at once.

Run with no subcommand to open the TUI, optionally scoped to specific repos or
groups. -c/--config applies to every subcommand and defaults to
$XDG_CONFIG_HOME/hrd/config.toml, falling back to ~/.config/hrd/config.toml.`,
		Example: `  hrd                    # open the TUI across all tracked repos
  hrd myrepo             # open the TUI scoped to one repo
  hrd repo add ~/code/*  # start tracking repos
  hrd ls                 # show status of all tracked repos`,
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIWithArgs(cmd.Context(), cfgPath, args)
		},
	}

	root.PersistentFlags().
		StringVarP(&cfgPath, flagConfig, flagConfigShort, cfgPath, "path to config file")

	root.AddGroup(commandGroups...)

	if len(aliases) > 0 {
		root.AddGroup(&cobra.Group{ID: "aliases", Title: "Aliases:"})
	}

	root.AddCommand(buildCommands(&cfgPath, aliases)...)

	return root
}

// flagString and its siblings below wrap cmd.Flags().Get* and discard the
// error: every call site passes a compile-time-constant name that's always
// registered on that exact command (see addDispatchFlags, addLsFlags,
// addStatusFilterFlags), so a lookup failure would be a programming error,
// not a runtime condition callers need to handle.
func flagString(cmd *cobra.Command, name string) string {
	v, _ := cmd.Flags().GetString(name)

	return v
}

func flagBool(cmd *cobra.Command, name string) bool {
	v, _ := cmd.Flags().GetBool(name)

	return v
}

func flagInt(cmd *cobra.Command, name string) int {
	v, _ := cmd.Flags().GetInt(name)

	return v
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}

	return args[0]
}
