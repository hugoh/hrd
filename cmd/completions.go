package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/hugoh/hrd/internal/config"
	"github.com/urfave/cli/v3"
)

func completeRepos(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Repos))

	for name := range cfg.Repos {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

func completeGroups(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Groups))

	for name := range cfg.Groups {
		names = append(names, displayGroup(name))
	}

	sort.Strings(names)

	return names
}

func repoGroupCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return func(_ context.Context, cmd *cli.Command) {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return
		}

		w := cmd.Root().Writer

		for _, name := range completeRepos(&cfg) {
			_, _ = fmt.Fprintln(w, name)
		}

		for _, name := range completeGroups(&cfg) {
			_, _ = fmt.Fprintln(w, name)
		}
	}
}

func reposOnlyCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return func(_ context.Context, cmd *cli.Command) {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return
		}

		w := cmd.Root().Writer

		for _, name := range completeRepos(&cfg) {
			_, _ = fmt.Fprintln(w, name)
		}
	}
}

func groupsOnlyCompleter(cfgPath *string) func(context.Context, *cli.Command) {
	return func(_ context.Context, cmd *cli.Command) {
		cfg, err := config.Load(*cfgPath)
		if err != nil {
			return
		}

		w := cmd.Root().Writer

		for _, name := range completeGroups(&cfg) {
			_, _ = fmt.Fprintln(w, name)
		}
	}
}
