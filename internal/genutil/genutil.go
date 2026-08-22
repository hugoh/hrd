// Package genutil holds boilerplate shared by the cmd/gen* code generators.
package genutil

import "os"

// Main runs f and, on error, prints it to stderr, returning the process
// exit code the caller's main() should exit with. os.Exit should only be
// called from main()/init(), so callers are expected to call
// os.Exit(genutil.Main(f)) themselves rather than this doing it directly.
func Main(f func() error) int {
	if err := f(); err != nil {
		_, _ = os.Stderr.WriteString("error: " + err.Error() + "\n")

		return 1
	}

	return 0
}
