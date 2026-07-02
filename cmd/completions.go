package cmd

import (
	"slices"

	"github.com/hugoh/hrd/internal/config"
	"github.com/spf13/cobra"
)

// cobraCompleter is the shape Cobra's ValidArgsFunction expects.
type cobraCompleter func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective)

func completeRepos(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Repos))

	for name := range cfg.Repos {
		names = append(names, name)
	}

	slices.Sort(names)

	return names
}

func completeGroups(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Groups))

	for name := range cfg.Groups {
		names = append(names, displayGroup(name))
	}

	slices.Sort(names)

	return names
}

func makeCompleter(
	cfgPath *string,
	getters ...func(*config.Config) []string,
) cobraCompleter {
	return func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		var out []string

		for _, get := range getters {
			out = append(out, get(&cfg)...)
		}

		return out, cobra.ShellCompDirectiveNoFileComp
	}
}

func repoGroupCompleter(cfgPath *string) cobraCompleter {
	return makeCompleter(cfgPath, completeRepos, completeGroups)
}

func reposOnlyCompleter(cfgPath *string) cobraCompleter {
	return makeCompleter(cfgPath, completeRepos)
}

func groupsOnlyCompleter(cfgPath *string) cobraCompleter {
	return makeCompleter(cfgPath, completeGroups)
}
