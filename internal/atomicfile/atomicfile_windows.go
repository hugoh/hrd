//go:build windows

package atomicfile

import "os"

// WriteFile writes filename's contents with data. Not atomic on Windows —
// see the package doc comment.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return os.WriteFile(filename, data, perm) //nolint:wrapcheck // thin platform wrapper
}
