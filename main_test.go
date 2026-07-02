package main

import (
	"bytes"
	"testing"

	"github.com/hugoh/hrd/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainRun(t *testing.T) {
	app := cmd.NewApp()
	assert.NotNil(t, app)

	var stdout, stderr bytes.Buffer

	app.SetOut(&stdout)
	app.SetErr(&stderr)

	err := cmd.RunApp(t.Context(), app, []string{"hrd", "--version"})
	assert.NoError(t, err)
}

func TestMainRunHelp(t *testing.T) {
	app := cmd.NewApp()
	app.SetOut(&bytes.Buffer{})
	app.SetErr(&bytes.Buffer{})

	err := cmd.RunApp(t.Context(), app, []string{"hrd", "--help"})
	assert.NoError(t, err)
}

func TestMainRunNoArgs(t *testing.T) {
	app := cmd.NewApp()
	app.SetOut(&bytes.Buffer{})
	app.SetErr(&bytes.Buffer{})

	err := cmd.RunApp(t.Context(), app, []string{"hrd"})
	// Without args, hrd launches the TUI, which requires a real TTY.
	// In test environments the TTY is unavailable, so verify TUI was invoked.
	require.ErrorContains(t, err, "TTY", "launched TUI but no TTY available")
}
