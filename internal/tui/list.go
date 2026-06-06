package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/config"
)

const (
	defaultListH = 10
	itemH        = 2
	itemIndent   = 2
)

func repoCountLabel(n int) string {
	if n == 1 {
		return "1 repo"
	}

	return fmt.Sprintf("%d repos", n)
}

// --- History list types ------------------------------------------------------

type historyItem struct {
	repos []string
	title string
	desc  string
}

func newHistoryItem(
	repos []string,
	timestamp string,
	groups map[string]config.Group,
	allRepoSet map[string]struct{},
) historyItem {
	var title string

	if labels, uncovered := matchingGroups(repos, allRepoSet, groups); len(labels) > 0 {
		parts := make([]string, 0, len(labels)+len(uncovered))
		parts = append(parts, labels...)
		parts = append(parts, uncovered...)
		title = strings.Join(parts, ", ")
	} else {
		title = strings.Join(repos, ", ")
	}

	desc := timestamp + "  " + repoCountLabel(len(repos))

	return historyItem{repos: repos, title: title, desc: desc}
}

func (i historyItem) Title() string       { return i.title }
func (i historyItem) Description() string { return i.desc }
func (i historyItem) FilterValue() string { return i.title }

func buildHistoryItems(
	entries []SelectionEntry,
	groups map[string]config.Group,
	allRepoSet map[string]struct{},
) []list.Item {
	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		ts := e.Timestamp.Format("Jan 02 15:04")
		items = append(items, newHistoryItem(e.Repos, ts, groups, allRepoSet))
	}

	return items
}

// --- Group list types --------------------------------------------------------

type groupItem struct {
	name string
	desc string
}

func (i groupItem) Title() string       { return i.name }
func (i groupItem) Description() string { return i.desc }
func (i groupItem) FilterValue() string { return i.name }

func buildGroupItems(names []string, groups map[string]config.Group, totalRepos int) []list.Item {
	items := make([]list.Item, 0, len(names))
	for _, name := range names {
		var desc string

		switch name {
		case labelAllCap:
			desc = fmt.Sprintf("all %d repos", totalRepos)
		case labelNew:
			desc = "create a new group"
		default:
			if g, ok := groups[name]; ok {
				desc = strings.Join(g.Repos, ", ")
			}
		}

		items = append(items, groupItem{name: name, desc: desc})
	}

	return items
}

// --- Delegate + list builder ---------------------------------------------------

func defaultItemDelegate(spacing int) list.DefaultDelegate {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(itemH)
	delegate.SetSpacing(spacing)

	title := lipgloss.NewStyle().PaddingLeft(itemIndent)
	desc := lipgloss.NewStyle().PaddingLeft(itemIndent).Foreground(lipgloss.Color("12"))

	delegate.Styles.NormalTitle = title
	delegate.Styles.SelectedTitle = title.Background(lipgloss.Color("62"))
	delegate.Styles.NormalDesc = desc
	delegate.Styles.SelectedDesc = desc

	return delegate
}

func initList(delegate list.ItemDelegate, items []list.Item, width int) list.Model {
	l := list.New(items, delegate, width, defaultListH)
	l.SetShowTitle(false)
	l.SetShowFilter(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.DisableQuitKeybindings()
	l.SetFilteringEnabled(false)

	return l
}

var (
	_ list.DefaultItem = historyItem{}
	_ list.DefaultItem = groupItem{}
)
