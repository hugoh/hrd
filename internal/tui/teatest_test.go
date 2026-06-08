package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/stretchr/testify/require"
)

func newTeaTestModel(t *testing.T) *teatest.TestModel {
	t.Helper()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir)

	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "config.toml")
	err := os.WriteFile(cfgPath, []byte(
		`[repos.testrepo]
path = "`+repoDir+`"
backends = ["git"]`), 0o644)
	require.NoError(t, err)

	m, err := newModel(t.Context(), Options{
		ConfigPath: cfgPath,
		StatePath:  filepath.Join(tmp, "state.json"),
	})
	require.NoError(t, err)

	m.selected["testrepo"] = true
	m.updateTableRows()

	return teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(100, 30),
	)
}

func TestTeaScreenNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tm := newTeaTestModel(t)

	// Init() → loadStatusesCmd → statusUpdateMsg → handleStatusUpdate
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "testrepo")
	})

	// '@' → Update → handleKeyMsg → handleMainKey → mainKeyHandlers["@"] → screenGroup
	tm.Send(tea.KeyPressMsg{Code: '@'})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Select group")
	})

	// Esc → Update → handleKeyMsg → handleEscKey → screenMain
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "testrepo")
	})

	// '?' → Update → handleKeyMsg → handleMainKey → mainKeyHandlers["?"] → screenHelp
	tm.Send(tea.KeyPressMsg{Code: '?'})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "Navigation")
	})

	// 'q' on help → Update → handleKeyMsg → handleQKey → screenMain
	tm.Send(tea.KeyPressMsg{Code: 'q'})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "testrepo")
	})

	// 'q' on main → Update → handleKeyMsg → handleQKey → tea.Quit
	tm.Send(tea.KeyPressMsg{Code: 'q'})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestTeaCommandBar(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tm := newTeaTestModel(t)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "testrepo")
	})

	// ':' → Update → handleKeyMsg → handleMainKey → mainKeyHandlers[":"] → commandOpen=true
	tm.Send(tea.KeyPressMsg{Code: ':'})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "type a command")
	})

	// Type into real textinput via tm.Type()
	// Each char: Update → handleKeyMsg → handleInputKey → input.Update(msg)
	tm.Type("status")

	// Esc → close command bar without executing
	tm.Send(tea.KeyPressMsg{Code: tea.KeyEsc})
	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return !strings.Contains(string(bts), "type a command")
	})

	// Quit
	tm.Send(tea.KeyPressMsg{Code: 'q'})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}

func TestTeaWindowSize(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	m, err := newModel(t.Context(), Options{})
	require.NoError(t, err)

	tm := teatest.NewTestModel(t, m,
		teatest.WithInitialTermSize(100, 30),
	)

	// 'q' on main screen → Update → handleKeyMsg → handleQKey → tea.Quit
	tm.Send(tea.KeyPressMsg{Code: 'q'})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))

	final, ok := tm.FinalModel(t).(*model)
	require.True(t, ok)
	require.True(t, final.ready, "model should be ready after WindowSizeMsg")
	require.Equal(t, 100, final.width)
	require.Equal(t, 30, final.height)

	cols := final.repoTable.Columns()
	require.Len(t, cols, 4)
	require.Positive(t, cols[3].Width)
}

func TestTeaDispatchCtrlC(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tm := newTeaTestModel(t)

	teatest.WaitFor(t, tm.Output(), func(bts []byte) bool {
		return strings.Contains(string(bts), "testrepo")
	})

	// Ctrl+C on main screen → Update → handleKeyMsg → handleCtrlC → quit
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(5*time.Second))
}
