package cmd

import (
	"bytes"

	"github.com/spf13/cobra"
)

// newTestApp creates a CLI app with buffered writers for testing.
func newTestApp() *cobra.Command {
	return bufferWriters(NewApp())
}

func bufferWriters(app *cobra.Command) *cobra.Command {
	app.SetOut(&bytes.Buffer{})
	app.SetErr(&bytes.Buffer{})

	return app
}
