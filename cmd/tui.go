package cmd

import (
	"context"

	"github.com/hugoh/hrd/internal/tui"
	"github.com/spf13/cobra"
)

// runTUIWithArgs opens the TUI scoped to the given positional repo/group
// args. Shared by bare "hrd" and "hrd tui", which are otherwise identical —
// see tuiCmd and buildRootCmd's RunE.
func runTUIWithArgs(ctx context.Context, cfgPath string, args []string) error {
	return runTUI(ctx, tui.Options{
		ConfigPath: cfgPath,
		Repos:      args,
	})
}

// tuiCmd returns the `tui` subcommand — an explicit, discoverable synonym
// for bare "hrd [repo|group...]".
func tuiCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tui [repo|group...]",
		Aliases: []string{"i"},
		Short:   "interactive terminal UI for browsing and running commands across repos",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTUIWithArgs(cmd.Context(), *cfgPath, args)
		},
	}
	cmd.ValidArgsFunction = repoGroupCompleter(cfgPath)

	return cmd
}
