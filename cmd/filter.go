package cmd

import (
	"context"
	"slices"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	flagDirty  = "dirty"
	flagAhead  = "ahead"
	flagBehind = "behind"
)

// addStatusFilterFlags registers the status-filter flags shared by every
// dispatch-style command onto fs.
func addStatusFilterFlags(fs *pflag.FlagSet) {
	fs.Bool(flagDirty, false, "only repos with a dirty working copy")
	fs.Bool(flagAhead, false, "only repos ahead of their remote (or with local-only work)")
	fs.Bool(flagBehind, false, "only repos behind their remote")
}

// statusFilter selects repos by their VCS status. Multiple criteria are a
// union: a repo matches when any requested condition holds.
type statusFilter struct {
	dirty     bool
	ahead     bool
	behind    bool
	attention bool
}

func statusFilterFromCmd(cmd *cobra.Command) statusFilter {
	return statusFilter{
		dirty:  flagBool(cmd, flagDirty),
		ahead:  flagBool(cmd, flagAhead),
		behind: flagBool(cmd, flagBehind),
	}
}

func (f statusFilter) active() bool {
	return f.dirty || f.ahead || f.behind || f.attention
}

func (f statusFilter) matches(st backend.RepoStatus) bool {
	if f.attention && st.NeedsAttention() {
		return true
	}

	if f.dirty && st.Dirty {
		return true
	}

	if f.ahead && st.LocalAhead > 0 {
		return true
	}

	return slices.ContainsFunc(st.Bookmarks, f.matchesBookmark)
}

func (f statusFilter) matchesBookmark(bm backend.BookmarkStatus) bool {
	return (f.ahead && bm.Ahead > 0) || (f.behind && bm.Behind > 0)
}

// applyStatusFilter narrows scope.names down to repos matching the status
// filter flags on cmd, gathering statuses in parallel. With no filter flags
// set and scope.attentionRequested false, it returns the input unchanged and
// a nil status map. attentionRequested is set when the caller's scope
// explicitly named the "@@attention" reserved pseudo-group, which requires
// backend.RepoStatus.NeedsAttention even without explicit filter flags.
// Repos whose status cannot be read are excluded with a warning. The
// gathered statuses are returned so callers (ls) can render them without a
// second gather.
func applyStatusFilter(
	ctx context.Context,
	cmd *cobra.Command,
	scope resolvedScope,
) ([]string, map[string]runner.StatusResult) {
	filter := statusFilterFromCmd(cmd)
	filter.attention = scope.attentionRequested

	if !filter.active() {
		return scope.names, nil
	}

	statuses := make(map[string]runner.StatusResult, len(scope.names))

	ch := runner.GatherStatus(ctx, scope.cfg.Repos, scope.names, scope.cfg.Settings.Concurrency)
	for res := range ch {
		statuses[res.RepoName] = res
	}

	var matched []string

	for _, name := range scope.names {
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
