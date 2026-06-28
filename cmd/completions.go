package cmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/hugoh/hrd/internal/config"
	"github.com/urfave/cli/v3"
)

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

func makeCompleter(cfgPath *string, getters ...func(*config.Config) []string) func(context.Context, *cli.Command) {
	return func(_ context.Context, cmd *cli.Command) {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return
		}

		w := cmd.Root().Writer

		for _, get := range getters {
			for _, name := range get(&cfg) {
				_, _ = fmt.Fprintln(w, name)
			}
		}
	}
}

func repoGroupCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return makeCompleter(cfgPath, completeRepos, completeGroups)
}

func reposOnlyCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return makeCompleter(cfgPath, completeRepos)
}

func groupsOnlyCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return makeCompleter(cfgPath, completeGroups)
}
