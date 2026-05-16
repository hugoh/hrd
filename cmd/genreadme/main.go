package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
)

const (
	refName  = "main"
	aheadBy  = 2
	behindBy = 3
)

var ansiStrip = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiStrip.ReplaceAllString(s, "")
}

func stripLeadingSpace(s string) string {
	lines := strings.Split(s, "\n")

	for i, line := range lines {
		if line != "" && line[0] == ' ' {
			lines[i] = line[1:]
		}
	}

	return strings.Join(lines, "\n")
}

func generateHelp() string {
	git.Register()
	jj.Register()

	app := cmd.NewApp()

	var buf bytes.Buffer

	app.Writer = &buf
	_ = app.Run(context.Background(), []string{"hrd", "--help"})

	out := stripANSI(buf.String())

	if home, err := os.UserHomeDir(); err == nil {
		out = strings.ReplaceAll(out, home, "~")
	}

	return strings.TrimSpace(out) + "\n"
}

func statusResults() []runner.StatusResult {
	return []runner.StatusResult{
		{
			RepoName: "myproject",
			Status: backend.RepoStatus{
				Ref: refName,
				Bookmarks: []backend.BookmarkStatus{
					{Name: refName, State: backend.RefStateAhead, Ahead: aheadBy},
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
					{Name: refName, State: backend.RefStateSynced},
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
					{
						Name:  "feat/rework",
						State: backend.RefStateDiverged,
						Ahead: 1, Behind: behindBy,
					},
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

func vcsByName() map[string]string {
	return map[string]string{
		"myproject":   "git",
		"dotfiles":    "jj",
		"infra":       "git",
		"old-service": "jj",
	}
}

func generateLL() string {
	results := statusResults()
	vcs := vcsByName()

	const (
		simWidth  = 200
		minWidth  = 20
		nameWidth = 15
		vcsWidth  = 3
	)

	widths := []int{
		nameWidth,
		vcsWidth,
		ui.ComputeRemainderWidth(simWidth, minWidth, nameWidth, vcsWidth),
	}

	rows := make([][]string, 0, len(results))
	for _, res := range results {
		line := ui.FormatDispatchStatusLine(res.Status, true)
		rows = append(rows, []string{res.RepoName, vcs[res.RepoName], line})
	}

	table := ui.RenderTable(
		[]string{cmd.NameLabel, cmd.VCSLabel, cmd.StatusLabel},
		rows,
		ui.EffectiveWidths(
			[]string{cmd.NameLabel, cmd.VCSLabel, cmd.StatusLabel},
			rows,
			widths,
		),
	)

	out := stripANSI(table)
	out = strings.TrimSpace(out)
	out = stripLeadingSpace(out)

	return out + "\n"
}

type templateData struct {
	LLOutput   string
	HelpOutput string
}

func run() error {
	tmplData := templateData{
		LLOutput:   generateLL(),
		HelpOutput: generateHelp(),
	}

	tplRaw, err := os.ReadFile("README.md.tpl")
	if err != nil {
		return fmt.Errorf("reading template: %w", err)
	}

	tmpl, err := template.New("readme").Parse(string(tplRaw))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	out, err := os.Create("README.md")
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}

	if err := tmpl.Execute(out, tmplData); err != nil {
		_ = out.Close()

		return fmt.Errorf("executing template: %w", err)
	}

	if err := out.Close(); err != nil {
		return fmt.Errorf("closing output: %w", err)
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")

		os.Exit(1)
	}
}
