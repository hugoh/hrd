package cmd

import (
	"bytes"

	"github.com/urfave/cli/v3"
)

// newTestApp creates a CLI app with buffered writers for testing.
func newTestApp() *cli.Command {
	return bufferWriters(NewApp())
}

func bufferWriters(app *cli.Command) *cli.Command {
	app.Writer = &bytes.Buffer{}
	app.ErrWriter = &bytes.Buffer{}

	return app
}
