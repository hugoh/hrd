// Command genman generates the man pages under man/ from the live Cobra
// command tree, so flag/usage docs can't drift from --help output. man/ is
// gitignored: this runs at release time (see .goreleaser.yml's before.hooks)
// so the pages are packaged without being committed. It also runs via
// `go generate ./...` (see main.go's go:generate directive) for local preview.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hugoh/hrd/backends/git"
	"github.com/hugoh/hrd/backends/jj"
	"github.com/hugoh/hrd/cmd"
	"github.com/spf13/cobra/doc"
)

const manDir = "man"

func run() error {
	git.Register()
	jj.Register()

	if err := os.MkdirAll(manDir, 0o750); err != nil { //nolint:mnd // standard dir perms
		return fmt.Errorf("creating %s: %w", manDir, err)
	}

	header := &doc.GenManHeader{
		Title:   "HRD",
		Section: "1",
		Source:  "hrd",
		Manual:  "hrd Manual",
	}

	if err := doc.GenManTree(cmd.NewApp(), header, manDir); err != nil {
		return fmt.Errorf("generating man pages: %w", err)
	}

	return anonymizeHomeDir()
}

// anonymizeHomeDir rewrites the -c/--config flag's default value, which
// Cobra renders as the actual absolute path on the machine that ran this
// generator, to use "~" instead. Without this, every regeneration would
// embed whoever ran it's home directory, making the committed man pages
// differ machine to machine and breaking the gen-check CI diff.
func anonymizeHomeDir() error {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return nil //nolint:nilerr // best-effort: no home dir to anonymize is not an error
	}

	entries, err := os.ReadDir(manDir)
	if err != nil {
		return fmt.Errorf("reading %s: %w", manDir, err)
	}

	for _, e := range entries {
		path := filepath.Join(manDir, e.Name())

		data, err := os.ReadFile(path) //nolint:gosec // path built from our own ReadDir listing
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		replaced := strings.ReplaceAll(string(data), home, "~")
		if replaced == string(data) {
			continue
		}

		//nolint:gosec,mnd // path built from our own ReadDir listing; 0o600 is a standard file perm
		if err := os.WriteFile(path, []byte(replaced), 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")

		os.Exit(1)
	}
}
