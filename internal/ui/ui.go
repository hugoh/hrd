// Package ui provides terminal UI rendering for repo status and dispatch results.
package ui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hugoh/hrd/internal/backend"
	"github.com/hugoh/hrd/internal/runner"
)

const (
	colName  = 20
	colVCS   = 5
	colRef   = 14
	colFlags = 4
)

//nolint:gochecknoglobals // UI colour and style definitions are inherently global
var (
	colorSynced   = lipgloss.Color("2")
	colorAhead    = lipgloss.Color("99")
	colorBehind   = lipgloss.Color("214")
	colorDiverged = lipgloss.Color("196")
	colorGone     = lipgloss.Color("238")
	colorNoRemote = lipgloss.Color("244")
	colorConflict = lipgloss.Color("196")
	colorDirty    = lipgloss.Color("214")
	colorMuted    = lipgloss.Color("240")
	colorLabel    = lipgloss.Color("252")
	colorVCS      = lipgloss.Color("33")
	colorVCSGit   = lipgloss.Color("244")
)

var (
	styleName   = lipgloss.NewStyle().Bold(true).Width(colName).MaxWidth(colName)
	styleVCSjj  = lipgloss.NewStyle().Foreground(colorVCS).Width(colVCS).Bold(true)
	styleVCSgit = lipgloss.NewStyle().Foreground(colorVCSGit).Width(colVCS)
	styleRef    = lipgloss.NewStyle().Foreground(colorLabel).Width(colRef).MaxWidth(colRef)
	styleMono   = lipgloss.NewStyle().Foreground(colorMuted)
	styleErr    = lipgloss.NewStyle().Foreground(colorDiverged)
	styleDim    = lipgloss.NewStyle().Foreground(colorMuted)
	styleBold   = lipgloss.NewStyle().Bold(true)
)

func badgeStyle(state backend.RefState, conflict bool) lipgloss.Style {
	if conflict {
		return lipgloss.NewStyle().Foreground(colorConflict).Bold(true).Padding(0, 1)
	}

	switch state {
	case backend.RefStateSynced:
		return lipgloss.NewStyle().Foreground(colorSynced).Padding(0, 1)
	case backend.RefStateAhead:
		return lipgloss.NewStyle().Foreground(colorAhead).Bold(true).Padding(0, 1)
	case backend.RefStateBehind:
		return lipgloss.NewStyle().Foreground(colorBehind).Bold(true).Padding(0, 1)
	case backend.RefStateDiverged:
		return lipgloss.NewStyle().Foreground(colorDiverged).Bold(true).Padding(0, 1)
	case backend.RefStateGone:
		return lipgloss.NewStyle().Foreground(colorGone).Strikethrough(true).Padding(0, 1)
	case backend.RefStateUnknown:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	case backend.RefStateNoRemote:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	default:
		return lipgloss.NewStyle().Foreground(colorNoRemote).Padding(0, 1)
	}
}

func RenderBookmarkBadge(bm backend.BookmarkStatus) string {
	var label strings.Builder
	label.WriteString(bm.Name)

	switch {
	case bm.Conflict:
		label.WriteString(" !")
	case bm.State == backend.RefStateSynced:
		label.WriteString(" ✓")
	case bm.State == backend.RefStateAhead:
		label.WriteString(" ↑" + strconv.Itoa(bm.Ahead))
	case bm.State == backend.RefStateBehind:
		label.WriteString(" ↓" + strconv.Itoa(bm.Behind))
	case bm.State == backend.RefStateDiverged:
		if bm.Ahead > 0 {
			label.WriteString(" ↑" + strconv.Itoa(bm.Ahead))
		}

		if bm.Behind > 0 {
			label.WriteString("↓" + strconv.Itoa(bm.Behind))
		}
	case bm.State == backend.RefStateGone:
		label.WriteString(" ✗")
	}

	return badgeStyle(bm.State, bm.Conflict).Render(label.String())
}

func RenderFlags(st backend.RepoStatus) string {
	var b strings.Builder
	if st.Dirty {
		b.WriteString(lipgloss.NewStyle().Foreground(colorDirty).Bold(true).Render("*"))
	} else {
		b.WriteString(" ")
	}

	if st.Conflict {
		b.WriteString(lipgloss.NewStyle().Foreground(colorConflict).Bold(true).Render("‼"))
	} else {
		b.WriteString(" ")
	}

	return b.String()
}

func RenderVCS(vcs string) string {
	if vcs == "jj" {
		return styleVCSjj.Render("jj")
	}

	return styleVCSgit.Render("git")
}

func RenderStatusLine(name, vcs string, st backend.RepoStatus) string {
	var ref string
	if vcs == "jj" {
		ref = styleRef.Inherit(styleMono).Render(st.Ref)
	} else {
		ref = styleRef.Render(st.Ref)
	}

	badges := make([]string, 0, len(st.Bookmarks))
	for _, bm := range st.Bookmarks {
		badges = append(badges, RenderBookmarkBadge(bm))
	}

	bookmarkCol := strings.Join(badges, " ")
	if len(badges) == 0 {
		bookmarkCol = styleDim.Render("(no bookmarks)")
	}

	return lipgloss.JoinHorizontal(lipgloss.Top,
		styleName.Render(name),
		RenderVCS(vcs),
		ref,
		lipgloss.NewStyle().Width(colFlags).Render(RenderFlags(st)),
		bookmarkCol,
	)
}

func RenderDispatchResult(res runner.Result) string {
	name := styleBold.Width(colName).Render(res.RepoName)
	if res.Err != nil {
		return name + " " + styleErr.Render("✗ "+res.Err.Error())
	}

	if res.ExitCode != 0 {
		return name + " " + styleErr.Render("✗ exit "+strconv.Itoa(res.ExitCode))
	}

	return name + " " + lipgloss.NewStyle().Foreground(colorSynced).Bold(true).Render("✓")
}
