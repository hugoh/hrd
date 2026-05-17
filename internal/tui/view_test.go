package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/hugoh/hrd/internal/runner"
)

func TestMainView(t *testing.T) {
	m := testModel()

	view := m.mainView()
	if view == "" {
		t.Fatal("mainView() returned empty")
	}
}

// testModel creates a model with default test configuration ready for View().
// opts can override fields before initTable() is called.
func testModel(opts ...func(*model)) *model {
	m := &model{
		screen: screenMain,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	for _, o := range opts {
		o(m)
	}

	m.ready = true
	m.initTable()

	return m
}

func TestViewHelpScreen(t *testing.T) {
	m := testModel(func(m *model) {
		m.screen = screenHelp
		m.initHelpViewport()
	})

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with help screen returned empty")
	}

	if !view.AltScreen {
		t.Error("View() should have AltScreen = true")
	}

	if view.MouseMode != tea.MouseModeCellMotion {
		t.Error("View() should have MouseModeCellMotion")
	}
}

func TestViewNotReady(t *testing.T) {
	m := &model{
		screen: screenMain,
		width:  80,
		height: 30,
	}

	m.ready = false

	view := m.View()
	if view.Content != "" {
		t.Errorf("View() should return empty content when not ready, got %q", view.Content)
	}
}

func TestViewGroupScreen(t *testing.T) {
	m := testModel(func(m *model) {
		m.screen = screenGroup
		m.groupPopupOptions = []string{labelAllCap}
		m.groupPopupCursor = 0
	})

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with group screen returned empty")
	}
}

func TestViewAlertInline(t *testing.T) {
	m := testModel(func(m *model) { m.modal = modalAlert })

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() with alert returned empty")
	}
}

func TestViewOutputScreen(t *testing.T) {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.View()
	if view.Content == "" {
		t.Fatal("View() on output screen returned empty")
	}
}

func TestOutputView(t *testing.T) {
	m := &model{
		screen: screenOutput,
		width:  80,
		height: 30,
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.outputView()
	if view == "" {
		t.Fatal("outputView() returned empty")
	}

	if !strings.Contains(view, "Output") {
		t.Errorf("outputView should contain 'Output', got %q", view)
	}
}

func TestOutputViewExecuting(t *testing.T) {
	m := &model{
		screen:    screenOutput,
		width:     80,
		height:    30,
		executing: true,
		execTotal: 5,
		execResults: []execResult{
			{name: "alpha", result: runner.Result{RepoName: "alpha"}},
			{name: "beta", result: runner.Result{RepoName: "beta"}},
		},
		output: viewport.New(viewport.WithWidth(80), viewport.WithHeight(10)),
	}
	m.ready = true

	view := m.outputView()

	if !strings.Contains(view, "[2/5]") {
		t.Errorf("outputView should show progress [2/5], got %q", view)
	}

	if !strings.Contains(view, "Esc/q:close") {
		t.Errorf("outputView should show Esc/q:close, got %q", view)
	}
}

func TestRenderInputLine(t *testing.T) {
	m := &model{
		cmdPrefix: prefixGit,
	}
	m.initInput()

	line := m.renderInputLine()
	if !strings.Contains(line, "[git]") {
		t.Errorf("expected [git] prompt, got %q", line)
	}
}

func TestRenderInputLineShell(t *testing.T) {
	m := &model{
		cmdPrefix: prefixShell,
	}
	m.initInput()

	line := m.renderInputLine()
	if !strings.Contains(line, "[sh]") {
		t.Errorf("expected [sh] prompt, got %q", line)
	}
}

func TestAlertContent(t *testing.T) {
	m := &model{}
	content := m.alertContent()

	if content == "" {
		t.Fatal("alertContent() returned empty")
	}

	if !strings.Contains(content, "No repos selected") {
		t.Errorf("alertContent() missing message, got %q", content)
	}

	if !strings.Contains(content, "@") {
		t.Errorf("alertContent() should mention @ key, got %q", content)
	}

	if !strings.Contains(content, "Space") {
		t.Errorf("alertContent() should mention Space key, got %q", content)
	}
}

func TestEmptyTableView(t *testing.T) {
	m := testModel()

	view := m.mainView()

	if !strings.Contains(view, "No repos selected") {
		t.Errorf("empty mainView() should contain empty state message, got %q", view)
	}

	if !strings.Contains(view, "@") {
		t.Errorf("empty mainView() should mention @ key, got %q", view)
	}

	if !strings.Contains(view, "Space") {
		t.Errorf("empty mainView() should mention Space key, got %q", view)
	}
}

func TestHelpContent(t *testing.T) {
	m := &model{}
	content := m.helpContent()

	if content == "" {
		t.Fatal("helpContent() returned empty")
	}

	if !strings.Contains(content, "Navigation") {
		t.Errorf("helpContent() missing Navigation, got %q", content)
	}
}

func TestGroupPopupContent(t *testing.T) {
	m := &model{
		groupPopupOptions: []string{"[all]", "work", "personal"},
		groupPopupCursor:  1,
		groupFilter:       "work",
	}

	content := m.groupPopupContent()
	if content == "" {
		t.Fatal("groupPopupContent() returned empty")
	}
}

func TestAnsiPrefix(t *testing.T) {
	tests := []struct {
		s      string
		visCol int
		want   string
	}{
		{"hello", 3, "hel"},
		{"hello", 5, "hello"},
		{"hello", 0, ""},
		{"\x1b[31mhello", 3, "\x1b[31mhel"},
		{"\x1b[31mhello\x1b[0m", 5, "\x1b[31mhello"},
		{"héllo", 2, "hé"},
	}
	for _, tt := range tests {
		got := ansiPrefix(tt.s, tt.visCol)
		if got != tt.want {
			t.Errorf("ansiPrefix(%q, %d) = %q, want %q", tt.s, tt.visCol, got, tt.want)
		}
	}
}

func TestAnsiSuffix(t *testing.T) {
	tests := []struct {
		s      string
		visCol int
		want   string
	}{
		{"hello", 3, "lo"},
		{"hello", 5, ""},
		{"hello", 0, "hello"},
		{"\x1b[31mhello", 3, "lo"},
		{"\x1b[31mhello\x1b[0m", 5, "\x1b[0m"},
	}
	for _, tt := range tests {
		got := ansiSuffix(tt.s, tt.visCol)
		if got != tt.want {
			t.Errorf("ansiSuffix(%q, %d) = %q, want %q", tt.s, tt.visCol, got, tt.want)
		}
	}
}

func TestAnsiByteOff(t *testing.T) {
	tests := []struct {
		s      string
		visCol int
		want   int
	}{
		{"hello", 3, 3},
		{"hello", 0, 0},
		{"\x1b[31mhello", 0, 0},
	}
	for _, tt := range tests {
		got := ansiByteOff(tt.s, tt.visCol)
		if got != tt.want {
			t.Errorf("ansiByteOff(%q, %d) = %d, want %d", tt.s, tt.visCol, got, tt.want)
		}
	}
}

func TestAnsiByteOffNoAnsi(t *testing.T) {
	off := ansiByteOff("hello", 3)
	if off != 3 {
		t.Errorf("ansiByteOff at col 3 = %d, want 3", off)
	}
}

func TestAnsiByteOffTwoByteEscape(t *testing.T) {
	// ESC M (0x4D is in 0x40-0x5F range → two-character escape)
	off := ansiByteOff("\x1bMhello", 3)
	got := off

	if got <= 3 {
		t.Errorf("ansiByteOff at col 3 = %d, expected > 3 (should skip 2-byte ESC seq)", got)
	}
}

func TestSkipUntilFinal(t *testing.T) {
	// ESC followed by intermediate bytes then final byte
	s := "\x1b!3hello"
	off := skipUntilFinal(s, 2) // start at '!' (after ESC)

	if off <= 2 {
		t.Errorf("skipUntilFinal = %d, expected > 2", off)
	}
}

func TestTruncateVis(t *testing.T) {
	tests := []struct {
		s      string
		maxVis int
		want   string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"\x1b[31mhello", 3, "\x1b[31mhel"},
	}
	for _, tt := range tests {
		got := truncateVis(tt.s, tt.maxVis)
		if got != tt.want {
			t.Errorf("truncateVis(%q, %d) = %q, want %q", tt.s, tt.maxVis, got, tt.want)
		}
	}
}
