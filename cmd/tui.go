package cmd

import (
	"github.com/hugoh/hrd/internal/tui"
	"github.com/spf13/cobra"
)

// tuiCmd returns the `tui` subcommand.
func tuiCmd(cfgPath *string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tui",
		Aliases: []string{"i"},
		Short:   "interactive terminal UI for browsing and running commands across repos",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			repos := flagStringSlice(cmd, cmdReposFlag)
			repos = append(repos, args...)

			return tui.Run(cmd.Context(), tui.Options{
				ConfigPath: *cfgPath,
				Group:      flagString(cmd, "group"),
				Repos:      repos,
			})
		},
	}
	cmd.Flags().StringP(cmdNameGroup, "g", "", "initial group filter (e.g. @work)")
	cmd.Flags().StringSliceP(cmdReposFlag, "r", nil, "initial repo selection")

	return cmd
}
