// Package genutil holds boilerplate shared by the cmd/gen* code generators.
package genutil

import "os"

// Main runs f and, on error, prints it to stderr and exits with status 1.
func Main(f func() error) {
	if err := f(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")

		os.Exit(1)
	}
}
