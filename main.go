//go:generate go run ./cmd/genreadme

// Package main is the hrd multi-repo manager entrypoint.
package main

import (
	"context"
	"errors"
	"os"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
	"github.com/hugoh/hrd/internal/ui"
)

func main() {
	os.Exit(Run(os.Args))
}

// Exit codes: 0 for success, 1 when the command ran but failed in some
// repos, 2 for usage or config errors.
const (
	exitOK          = 0
	exitReposFailed = 1
	exitUsage       = 2
)

// Run executes the hrd application with the given arguments and returns
// the process exit code.
func Run(args []string) int {
	git.Register()
	jj.Register()

	app, head := cmd.NewAppForArgs(args)

	err := app.Run(context.Background(), head)
	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, cmd.ErrReposFailed):
		// The per-repo summary line already reported the failures.
		return exitReposFailed
	default:
		ui.Errf("error: %v", err)

		return exitUsage
	}
}
