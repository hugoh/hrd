package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/config"
)

const (
	defaultListH   = 10
	initDelegateH  = 2
	groupIndent    = 2
	groupDescItemH = 2
)

// --- History list types ------------------------------------------------------

type historyItem struct {
	entry SelectionEntry
	title string
	desc  string
}

func newHistoryItem(
	entry SelectionEntry,
	groups map[string]config.Group,
	allRepoSet map[string]struct{},
) historyItem {
	title := entry.Timestamp.Format("Jan 02 15:04") + "  " + repoCountLabel(len(entry.Repos))

	var desc string

	if labels, uncovered := matchingGroups(entry.Repos, allRepoSet, groups); len(labels) > 0 {
		parts := make([]string, 0, len(labels)+len(uncovered))
		parts = append(parts, labels...)
		parts = append(parts, uncovered...)
		desc = strings.Join(parts, ", ")
	} else {
		desc = strings.Join(entry.Repos, ", ")
	}

	return historyItem{entry: entry, title: title, desc: desc}
}

func (i historyItem) Title() string       { return i.title }
func (i historyItem) Description() string { return i.desc }
func (i historyItem) FilterValue() string { return strings.Join(i.entry.Repos, " ") }

// historyDelegate renders multi-line selection history items with word-wrapped
// descriptions. All items share a fixed height computed from the widest entry.
type historyDelegate struct {
	width  int
	height int
}

func newHistoryDelegate(width int) *historyDelegate {
	return &historyDelegate{width: width, height: initDelegateH}
}

func (d *historyDelegate) Height() int { return d.height }
func (*historyDelegate) Spacing() int  { return 1 }

func (d *historyDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	hi, ok := item.(historyItem)
	if !ok {
		return
	}

	wrapW := max(m.Width()-historyIndent, historyMinWrapW)

	marker := "  "
	if index == m.Index() {
		marker = "▸ "
	}

	title := fmt.Sprintf("%s%s", marker, hi.Title())
	_, _ = w.Write([]byte(title + "\n"))

	descLines := wrapString(hi.Description(), wrapW)
	for _, line := range descLines {
		_, _ = w.Write([]byte("  " + line + "\n"))
	}

	for i := len(descLines); i < d.height-1; i++ {
		_, _ = w.Write([]byte("\n"))
	}
}

func (*historyDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd {
	return nil
}

// recomputeHeight recalculates the delegate height based on current width and
// the given items. Must be called whenever items or width change.
func (d *historyDelegate) recomputeHeight(items []list.Item) {
	maxLines := 0
	wrapW := max(d.width-historyIndent, historyMinWrapW)

	for _, item := range items {
		hi, ok := item.(historyItem)
		if !ok {
			continue
		}

		lines := len(wrapString(hi.Description(), wrapW))
		if lines > maxLines {
			maxLines = lines
		}
	}

	d.height = 1 + maxLines
}

func buildHistoryItems(
	entries []SelectionEntry,
	groups map[string]config.Group,
	allRepoSet map[string]struct{},
) []list.Item {
	items := make([]list.Item, 0, len(entries))
	for _, e := range entries {
		items = append(items, newHistoryItem(e, groups, allRepoSet))
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

// --- List constructor helpers ------------------------------------------------

func initHistoryList(
	entries []SelectionEntry,
	groups map[string]config.Group,
	allRepoSet map[string]struct{},
	width int,
) list.Model {
	delegate := newHistoryDelegate(width)
	items := buildHistoryItems(entries, groups, allRepoSet)
	delegate.recomputeHeight(items)

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

func initGroupList(width int) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = true
	delegate.SetHeight(groupDescItemH)
	delegate.SetSpacing(0)
	delegate.Styles.NormalTitle = lipgloss.NewStyle().
		PaddingLeft(groupIndent)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		PaddingLeft(groupIndent)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().
		PaddingLeft(groupIndent).
		Foreground(lipgloss.Color("8"))
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		PaddingLeft(groupIndent).
		Foreground(lipgloss.Color("8"))

	l := list.New(nil, delegate, width, defaultListH)
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
	_ list.DefaultItem  = historyItem{}
	_ list.DefaultItem  = groupItem{}
	_ list.ItemDelegate = (*historyDelegate)(nil)
)
