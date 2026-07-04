//go:build !windows

// Package atomicfile writes files atomically where the OS supports it. On
// Windows, renameio exports no API at all (atomic replace isn't reliably
// possible there), so this falls back to a plain write on that platform.
package atomicfile

import (
	"os"

	"github.com/google/renameio/v2"
)

// WriteFile atomically replaces filename's contents with data.
func WriteFile(filename string, data []byte, perm os.FileMode) error {
	return renameio.WriteFile(filename, data, perm) //nolint:wrapcheck // thin platform wrapper
}
