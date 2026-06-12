package cmd

import (
	"bytes"

	"github.com/urfave/cli/v3"
)

// newTestApp creates a CLI app with buffered writers for testing.
func newTestApp() *cli.Command {
	return newTestAppWithTail(nil)
}

// newTestAppWithTail creates a CLI app with buffered writers and the given
// "--" tail (see SplitDashTail).
func newTestAppWithTail(dashTail []string) *cli.Command {
	app := NewAppWithTail(dashTail)
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	return app
}
