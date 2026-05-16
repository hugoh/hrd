package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/hugoh/hrd/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainRun(t *testing.T) {
	app := cmd.NewApp()
	assert.NotNil(t, app)

	var stdout, stderr bytes.Buffer

	app.Writer = &stdout
	app.ErrWriter = &stderr

	err := app.Run(context.Background(), []string{"hrd", "--version"})
	assert.NoError(t, err)
}

func TestMainRunHelp(t *testing.T) {
	app := cmd.NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd", "--help"})
	assert.NoError(t, err)
}

func TestMainRunNoArgs(t *testing.T) {
	app := cmd.NewApp()
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	err := app.Run(context.Background(), []string{"hrd"})
	// Without args, hrd launches the TUI, which requires a real TTY.
	// In test environments the TTY is unavailable, so verify TUI was invoked.
	require.ErrorContains(t, err, "TTY", "launched TUI but no TTY available")
}
