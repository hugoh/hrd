//go:generate go run ./cmd/genreadme

// Package main is the hrd multi-repo manager entrypoint.
package main

import (
	"context"
	"os"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
	"github.com/hugoh/hrd/internal/ui"
)

func main() {
	os.Exit(Run(os.Args))
}

// Run executes the hrd application with the given arguments.
// Returns exit code: 0 for success, 1 for error.
func Run(args []string) int {
	git.Register()
	jj.Register()

	app := cmd.NewApp()

	err := app.Run(context.Background(), args)
	if err != nil {
		ui.Errf("error: %v", err)

		return 1
	}

	return 0
}
