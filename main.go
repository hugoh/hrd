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
	git.Register()
	jj.Register()

	app := cmd.NewApp()

	err := app.Run(context.Background(), os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
