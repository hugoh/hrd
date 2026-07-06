package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/hugoh/hrd/internal/config"
	"github.com/hugoh/hrd/internal/runner"
	"github.com/hugoh/hrd/internal/ui"
	"golang.org/x/term"
)

var errNoTTY = errors.New("TTY required to launch TUI")

// Options configures the TUI entry point.
type Options struct {
	ConfigPath string
	Group      string
	Repos      []string
	StatePath  string
}

type screen int

const (
	screenMain screen = iota
	screenOutput
	screenHelp
	screenGroup
	screenSelHistory
)

type mode int

const (
	modeNormal mode = iota
	modeSelect
	modeSingle
)

type modal int

const (
	modalNone modal = iota
	modalAlert
)

type groupMode int

const (
	groupFilterMode groupMode = iota
	groupAddMode
)

type cmdPrefix int

const (
	prefixNone cmdPrefix = iota
	prefixShell
	numPrefixes
)

// VCS subcommands known by the TUI, used for tab completion in the unified
// command bar.
//
//nolint:gochecknoglobals,goconst // command names are effectively constant
var vcsSubcommands = []string{"status", "diff", "log", "fetch", "pull", "push"}

//nolint:gochecknoglobals // effectively constant, prefix label lookup
var prefixLabels = [numPrefixes]string{
	"",
	"sh",
}

const (
	maxNameWidth  = 24
	layoutHeaderH = 1
	layoutFooterH = 1
	layoutSepH    = 1
	inputLineH    = 1
	defaultViewW  = 80
	minInputWidth = 10
	minContentH   = 3
	initTableH    = 10
	initOutputH   = 10

	listVCSWidth = 3
	checkboxColW = 2

	colName   = "NAME"
	colVCS    = "VCS"
	colStatus = "STATUS"

	initInputW = 40

	labelAll = "all"
	labelNew = "[new...]"
	keyEsc   = "esc"
	keyEnter = "enter"

	secNavigation = "Navigation"
	secSelection  = "Selection"
	secGroups     = "Groups"
	secCommands   = "Commands"
	secGeneral    = "General"
	secCmdBar     = "Cmd bar"
)

// binding describes a single key binding in the main screen.
// Whether a command refreshes repo statuses afterwards is decided by its
// handler (the sideEffect argument to shortcutCmd), not declared here.
type binding struct {
	key        string
	displayKey string // human-readable for header/footer; empty = use key
	handler    func(*model) (tea.Model, tea.Cmd)
	label      string
	desc       string
	hrd        bool // true → header (hrd-level), false → footer (repo-level)
	section    string
	order      int
}

// model is the root Bubble Tea model for the hrd TUI.
type model struct {
	ctx    context.Context //nolint:containedctx // lifecycle context wired once in Run()
	ready  bool
	width  int
	height int

	cfg  config.Config
	opts Options

	screen screen
	modal  modal

	repoTable   table.Model
	cursor      int
	selected    map[string]bool
	mode        mode
	groupFilter string

	// nameFilter narrows the visible repos by fuzzy name match; session
	// only, not persisted. filterOpen is true while the "/" input is
	// focused.
	nameFilter  string
	filterOpen  bool
	filterInput textinput.Model

	// attentionFilter narrows the visible repos to those needing attention
	// (dirty or out of sync with their remote), toggled with "*". It layers
	// on top of groupFilter/nameFilter rather than replacing them.
	attentionFilter bool

	selectSaved map[string]bool

	repoOrder []string

	loading  bool
	spinner  spinner.Model
	statuses map[string]runner.StatusResult
	vcsCache map[string]string

	groupList     list.Model
	groupMode     groupMode
	groupNewInput bool

	historyList list.Model
	alertMsg    string

	commandOpen         bool
	input               textinput.Model
	cmdPrefix           cmdPrefix
	historyIdx          int
	historyFilterPrefix string
	vcsCompletions      map[string][]string

	executing      bool
	execSideEffect bool
	execTotal      int
	execCancel     context.CancelFunc
	execResults    []execResult
	execOutputStr  string
	execLabel      string
	resultsCh      <-chan runner.Result

	statusCh <-chan runner.StatusResult

	output viewport.Model

	helpViewport viewport.Model

	stateFile string
	persState PersistentState
}

//nolint:funlen // model initialization with many setup steps
func newModel(ctx context.Context, opts Options) (*model, error) {
	statePath := opts.StatePath
	if statePath == "" {
		statePath = defaultStatePath()
	}

	cfg, _, err := config.LoadResolved(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	persState, err := loadState(statePath)
	if err != nil {
		return nil, fmt.Errorf("loading tui state: %w", err)
	}

	repoOrder := sortedRepoKeys(cfg.Repos)

	var selected map[string]bool

	if len(opts.Repos) > 0 {
		resolved, err := cfg.ResolveScope(opts.Repos)
		if err != nil {
			return nil, fmt.Errorf("resolving repos: %w", err)
		}

		selected = makeSelectedMap(resolved)
	} else {
		selected = restoreSelected(persState.LastRepos, cfg.Repos)
		if len(persState.LastRepos) == 0 {
			for _, name := range repoOrder {
				selected[name] = true
			}
		}
	}

	groupFilter, err := resolveGroupFilter(opts.Group, persState.LastGroup, cfg)
	if err != nil {
		return nil, err
	}

	m := &model{
		ctx:     ctx,
		cfg:     cfg,
		opts:    opts,
		loading: true,
		spinner: spinner.New(
			spinner.WithSpinner(spinner.Dot),
			spinner.WithStyle(ui.MutedStyle()),
		),
		statuses:    make(map[string]runner.StatusResult, len(cfg.Repos)),
		vcsCache:    make(map[string]string, len(cfg.Repos)),
		repoOrder:   repoOrder,
		selected:    selected,
		groupFilter: groupFilter,
		stateFile:   statePath,
		persState:   persState,
		historyIdx:  -1,
	}

	m.pushSelectionHistory()
	m.initTable()
	m.updateTableRows()
	m.initInput()
	m.initFilterInput()
	m.initOutput()
	m.initHelpViewport()
	m.initHistoryList()
	m.initGroupList()

	return m, nil
}

func (m *model) Init() tea.Cmd {
	return tea.Batch(
		loadStatusesCmd(m),
		m.spinner.Tick,
	)
}

func (m *model) initTable() {
	cols := []table.Column{
		{Title: "", Width: checkboxColW},
		{Title: colName, Width: maxNameWidth},
		{Title: colVCS, Width: listVCSWidth},
		{Title: colStatus, Width: 0},
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows([]table.Row{}),
		table.WithFocused(true),
		table.WithHeight(initTableH),
		table.WithWidth(defaultViewW),
	)
	m.repoTable = t
	m.repoTable.SetStyles(tableStyles(false))
}

func tableStyles(cursorVisible bool) table.Styles {
	selected := lipgloss.NewStyle() // empty: preserves Cell styling, no width change
	if cursorVisible {
		selected = lipgloss.NewStyle().
			Background(lipgloss.Color("62"))
	}

	return table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("14")).
			Padding(0, 1),
		Cell: lipgloss.NewStyle().
			Padding(0, 1),
		Selected: selected,
	}
}

func (m *model) initInput() {
	ti := textinput.New()
	ti.Placeholder = "type a command..."
	ti.CharLimit = 512
	ti.SetWidth(initInputW) // adjusted on resize
	m.input = ti
}

func (m *model) initFilterInput() {
	ti := textinput.New()
	ti.Placeholder = "filter repos by name..."
	ti.CharLimit = 128
	ti.SetWidth(initInputW) // adjusted on resize
	m.filterInput = ti
}

// openNameFilter focuses the "/" name-filter input, pre-filled with the
// current filter so it can be refined.
func (m *model) openNameFilter() {
	m.filterOpen = true
	m.filterInput.SetValue(m.nameFilter)
	m.filterInput.Focus()
	m.filterInput.SetWidth(m.inputWidth())
}

// clearNameFilter drops the name filter and closes its input.
func (m *model) clearNameFilter() {
	m.filterOpen = false
	m.nameFilter = ""
	m.filterInput.SetValue("")
	m.filterInput.Blur()
	m.updateTableRows()
}

func (m *model) initOutput() {
	m.output = viewport.New(viewport.WithWidth(defaultViewW), viewport.WithHeight(initOutputH))
}

func (m *model) initHelpViewport() {
	m.helpViewport = viewport.New(
		viewport.WithWidth(defaultViewW),
		viewport.WithHeight(initOutputH),
	)
	m.helpViewport.SetContent(m.helpContent())
}

func (m *model) initHistoryList() {
	items := buildHistoryItems(m.persState.SelectionHistory, m.cfg.Groups, m.allRepoSet())
	m.historyList = initList(defaultItemDelegate(0), items, defaultViewW)
}

// allRepoSet returns the set of all configured repo names.
func (m *model) allRepoSet() map[string]struct{} {
	set := make(map[string]struct{}, len(m.cfg.Repos))
	for name := range m.cfg.Repos {
		set[name] = struct{}{}
	}

	return set
}

func (m *model) initGroupList() {
	m.groupList = initList(defaultItemDelegate(0), nil, defaultViewW)
}

// Run starts the Bubble Tea event loop and blocks until the user quits.
func Run(ctx context.Context, opts Options) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errNoTTY
	}

	m, err := newModel(ctx, opts)
	if err != nil {
		return err
	}

	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("bubbletea app: %w", err)
	}

	return nil
}

func (m *model) quit() {
	m.pushSelectionHistory()
	m.savePersState()
	m.execCancelAll()
}

func (m *model) savePersState() {
	repos := make([]string, 0, len(m.selected))
	for _, name := range m.repoOrder {
		if m.selected[name] {
			repos = append(repos, name)
		}
	}

	slices.Sort(repos)
	m.persState.LastRepos = repos
	m.persState.LastGroup = m.groupFilter
	_ = saveState(m.stateFile, m.persState)
}

func (m *model) execCancelAll() {
	if m.execCancel != nil {
		m.execCancel()
		m.executing = false
	}
}

// contentHeight returns the number of lines available for the repo-list table
// (or the output viewport), after subtracting header, footer, separators and
// optional input line.
func (m *model) contentHeight() int {
	h := m.height
	h -= layoutHeaderH // header
	h -= layoutSepH    // separator after header
	h -= layoutFooterH // footer

	h -= layoutSepH // separator before footer
	if m.commandOpen || m.filterOpen {
		h -= inputLineH
	}

	if h < minContentH {
		return minContentH
	}

	return h
}

// inputWidth returns the usable width for the command input field.
func (m *model) inputWidth() int {
	const promptLen = 8 // "[prefix] $ " roughly

	w := m.width - promptLen
	if w < minInputWidth {
		return minInputWidth
	}

	return w
}
