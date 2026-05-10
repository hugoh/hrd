package cmd

import (
	"testing"

	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"github.com/jedib0t/go-pretty/v6/table"
)

func simulateStatusResults() []runner.StatusResult {
	return []runner.StatusResult{
		{
			RepoName: "myproject",
			Status: backend.RepoStatus{
				Ref: "main",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateAhead, Ahead: 2},
				},
				OverallState: backend.RefStateAhead,
				Dirty:        true,
				CommitMsg:    "feat: add new feature",
				CommitTime:   "(2 hours ago)",
			},
		},
		{
			RepoName: "dotfiles",
			Status: backend.RepoStatus{
				Ref: "rlkvwrto",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "main", State: backend.RefStateSynced},
					{Name: "feat", State: backend.RefStateNoRemote},
				},
				OverallState: backend.RefStateSynced,
				CommitMsg:    "config: update zshrc",
				CommitTime:   "(1 day ago)",
			},
		},
		{
			RepoName: "infra",
			Status: backend.RepoStatus{
				Ref: "feat/rework",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "feat/rework", State: backend.RefStateDiverged, Ahead: 1, Behind: 3},
				},
				OverallState: backend.RefStateDiverged,
				CommitMsg:    "refactor: networking layer",
				CommitTime:   "(3 days ago)",
			},
		},
		{
			RepoName: "old-service",
			Status: backend.RepoStatus{
				Ref: "qpvuntop",
				Bookmarks: []backend.BookmarkStatus{
					{Name: "legacy", State: backend.RefStateGone},
					{Name: "fix", State: backend.RefStateSynced, Conflict: true},
				},
				OverallState: backend.RefStateDiverged,
				Conflict:     true,
				CommitMsg:    "fix: critical bug",
				CommitTime:   "(1 week ago)",
			},
		},
	}
}

func simulateLsTestData() ([]runner.StatusResult, map[string]string) {
	vcsByName := map[string]string{
		"myproject":   "git",
		"dotfiles":    "jj",
		"infra":       "git",
		"old-service": "jj",
	}

	return simulateStatusResults(), vcsByName
}

func TestSimulateLsOutput(_ *testing.T) {
	results, vcsByName := simulateLsTestData()

	tbl := ui.NewTable()

	// Use a simulated terminal width to avoid truncation in test output.
	const simWidth = 200

	nameWidth := 15
	vcsWidth := 3
	refWidth := 20
	msgWidth := ui.ComputeRemainderWidth(simWidth, 20, nameWidth, vcsWidth, refWidth)

	header := table.Row{nameLabel, vcsLabel, refLabel, msgLabel}
	colConfigs := []table.ColumnConfig{
		{Number: 1, WidthMax: nameWidth, WidthMaxEnforcer: ui.Truncate},
		{Number: 2, WidthMax: vcsWidth, WidthMaxEnforcer: ui.Truncate},
		{Number: 3, WidthMax: refWidth},
		{Number: 4, WidthMax: msgWidth, WidthMaxEnforcer: ui.Wrap},
	}

	tbl.AppendHeader(header)
	tbl.SetColumnConfigs(colConfigs)

	appendStatusRows(tbl, results, vcsByName, true)

	tbl.Render()
}
