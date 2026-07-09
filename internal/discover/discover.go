// Package discover walks a directory tree looking for repositories managed
// by a known VCS backend, shared by hrd's one-shot scan commands and the
// live-resolved directory roots feature.
package discover

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/hugoh/hrd/internal/backend"
)

// Warning describes a non-fatal issue encountered while walking a directory.
type Warning struct {
	Path string
	Err  error
}

// Repos walks root up to maxDepth levels deep and returns the directories
// managed by a known VCS. Detected repos are not descended into (nested
// checkouts like vendored deps stay unreported), and hidden directories are
// skipped. Unreadable directories are reported as warnings rather than
// aborting the walk.
func Repos(root string, maxDepth int) ([]string, []Warning, error) {
	var found []string

	var warnings []Warning

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			warnings = append(warnings, Warning{Path: path, Err: err})

			return nil //nolint:nilerr // walk errors are reported as warnings, not aborted
		}

		if !d.IsDir() {
			return nil
		}

		if path != root && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}

		if Depth(root, path) > maxDepth {
			return fs.SkipDir
		}

		if _, derr := backend.Detect(path); derr == nil {
			found = append(found, path)

			return fs.SkipDir
		}

		return nil
	})
	if err != nil { // coverage-ignore — WalkDir errors are handled in the callback
		return nil, warnings, fmt.Errorf("walking %q: %w", root, err)
	}

	return found, warnings, nil
}

// Depth returns how many levels below root the path sits (root = 0).
func Depth(root, path string) int {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return 0
	}

	return strings.Count(rel, string(filepath.Separator)) + 1
}
