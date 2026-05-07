// Package main is the hrd multi-repo manager entrypoint.
package main

import (
	"context"
	"fmt"
	"os"

	// Import backends to trigger their registration.
	// jj must be imported before git so it wins detection on colocated repos.
	git "github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
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
		fmt.Fprintln(os.Stderr, "error:", err)

		return 1
	}

	return 0
}
