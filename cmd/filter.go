package cmd

import (
	"context"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/urfave/cli/v3"
)

const (
	flagDirty  = "dirty"
	flagAhead  = "ahead"
	flagBehind = "behind"
)

//nolint:gochecknoglobals // CLI flag definitions are package-level by nature
var statusFilterFlags = []cli.Flag{
	&cli.BoolFlag{
		Name:  flagDirty,
		Usage: "only repos with a dirty working copy",
	},
	&cli.BoolFlag{
		Name:  flagAhead,
		Usage: "only repos ahead of their remote (or with local-only work)",
	},
	&cli.BoolFlag{
		Name:  flagBehind,
		Usage: "only repos behind their remote",
	},
}

// statusFilter selects repos by their VCS status. Multiple criteria are a
// union: a repo matches when any requested condition holds.
type statusFilter struct {
	dirty  bool
	ahead  bool
	behind bool
}

func statusFilterFromCmd(cmd *cli.Command) statusFilter {
	return statusFilter{
		dirty:  cmd.Bool(flagDirty),
		ahead:  cmd.Bool(flagAhead),
		behind: cmd.Bool(flagBehind),
	}
}

func (f statusFilter) active() bool {
	return f.dirty || f.ahead || f.behind
}

func (f statusFilter) matches(st backend.RepoStatus) bool {
	if f.dirty && st.Dirty {
		return true
	}

	if f.ahead && st.LocalAhead > 0 {
		return true
	}

	for _, bm := range st.Bookmarks {
		if f.ahead && bm.Ahead > 0 {
			return true
		}

		if f.behind && bm.Behind > 0 {
			return true
		}
	}

	return false
}

// applyStatusFilter narrows names down to repos matching the status filter
// flags on cmd, gathering statuses in parallel. With no filter flags set it
// returns the input unchanged and a nil status map. Repos whose status
// cannot be read are excluded with a warning. The gathered statuses are
// returned so callers (ls) can render them without a second gather.
func applyStatusFilter(
	ctx context.Context,
	cmd *cli.Command,
	cfg *config.Config,
	names []string,
) ([]string, map[string]runner.StatusResult) {
	filter := statusFilterFromCmd(cmd)
	if !filter.active() {
		return names, nil
	}

	statuses := make(map[string]runner.StatusResult, len(names))

	ch := runner.GatherStatus(ctx, cfg.Repos, names, cfg.Settings.Concurrency)
	for res := range ch {
		statuses[res.RepoName] = res
	}

	var matched []string

	for _, name := range names {
		res := statuses[name]
		if res.Err != nil {
			ui.Warnf("%s: %v", name, res.Err)

			continue
		}

		if filter.matches(res.Status) {
			matched = append(matched, name)
		}
	}

	if len(matched) == 0 {
		ui.Warnf("no repos matched filter")
	}

	return matched, statuses
}
