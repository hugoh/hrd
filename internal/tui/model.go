package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"

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
	prefixGit
	prefixJj
	prefixShell
	numPrefixes
)

//nolint:gochecknoglobals // effectively constant, prefix label lookup
var prefixLabels = [numPrefixes]string{
	"",
	vcsGit,
	vcsJj,
	"sh",
}

const (
	maxNameWidth  = 24
	layoutHeaderH = 1
	layoutFooterH = 1
	layoutSepH    = 1
	inputLineH    = 1
	maxHistoryLen = 100
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

	vcsGit      = "git"
	vcsJj       = "jj"
	labelAll    = "all"
	labelAllCap = "[all]"
	labelNew    = "[new...]"
	keyEsc      = "esc"
	keyEnter    = "enter"

	secNavigation = "Navigation"
	secSelection  = "Selection"
	secGroups     = "Groups"
	secCommands   = "Commands"
	secGeneral    = "General"
	secCmdBar     = "Cmd bar"
)

// binding describes a single key binding in the main screen.
type binding struct {
	key        string
	displayKey string // human-readable for header/footer; empty = use key
	handler    func(*model) (tea.Model, tea.Cmd)
	label      string
	desc       string
	hrd        bool // true → header (hrd-level), false → footer (repo-level)
	sideEffect bool // true → auto-refresh repos after execution
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

	repoOrder []string

	loading  bool
	spinner  spinner.Model
	statuses map[string]runner.StatusResult

	groupPopupCursor  int
	groupPopupOptions []string
	groupMode         groupMode
	groupNewInput     bool

	commandOpen bool
	input       textinput.Model
	cmdPrefix   cmdPrefix

	executing      bool
	execSideEffect bool
	execTotal      int
	execCancel     context.CancelFunc
	execResults    []execResult
	resultsCh      <-chan runner.Result

	statusCh <-chan runner.StatusResult

	output viewport.Model

	helpViewport viewport.Model

	stateFile string
	persState PersistentState
}

func newModel(ctx context.Context, opts Options) (*model, error) {
	statePath := opts.StatePath
	if statePath == "" {
		statePath = defaultStatePath()
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("new model: %w", err)
	}

	persState, err := loadState(statePath)
	if err != nil {
		return nil, fmt.Errorf("loading tui state: %w", err)
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
		statuses:    make(map[string]runner.StatusResult),
		repoOrder:   sortedRepoKeys(cfg.Repos),
		selected:    restoreSelected(persState.LastRepos, cfg.Repos),
		groupFilter: resolveGroupFilter(opts.Group, persState.LastGroup, cfg),
		stateFile:   statePath,
		persState:   persState,
	}

	m.initTable()
	m.updateTableRows()
	m.initInput()
	m.initOutput()
	m.initHelpViewport()

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
	m.savePersState()
	m.execCancelAll()
}

func (m *model) savePersState() {
	repos := make([]string, 0, len(m.selected))
	for _, name := range m.filteredRepos() {
		if m.selected[name] {
			repos = append(repos, name)
		}
	}

	sort.Strings(repos)
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
	if m.commandOpen {
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
