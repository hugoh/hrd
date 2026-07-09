package config

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/hugoh/hrd/internal/discover"
)

const defaultRootDepth = 1

var errRootNameConflict = errors.New("name already taken")

// LoadResolved loads the config at path and merges in repos discovered
// under its configured Roots. The returned errs are non-fatal issues from
// resolving roots (unreadable directories, unresolvable name conflicts);
// err is the fatal error from Load itself, if any.
func LoadResolved(path string) (Config, []error, error) {
	cfg, err := Load(path)
	if err != nil {
		return cfg, nil, err
	}

	return cfg, cfg.ResolveRoots(), nil
}

// ResolveRoots walks each configured Root and merges freshly discovered
// repos into c.Repos in-memory. Static repos always win name collisions;
// among discovered repos, the fallback name is "<rootname>-<base>". Never
// mutates the file on disk — call after Load, before repos reach any
// consumer. Non-fatal issues (unreadable directories, unresolvable name
// conflicts) are returned as errs rather than aborting the resolve.
func (c *Config) ResolveRoots() []error {
	var errs []error

	rootNames := make([]string, 0, len(c.Roots))
	for name := range c.Roots {
		rootNames = append(rootNames, name)
	}

	slices.Sort(rootNames)

	for _, rootName := range rootNames {
		root := c.Roots[rootName]

		depth := root.Depth
		if depth <= 0 {
			depth = defaultRootDepth
		}

		paths, warnings, err := discover.Repos(root.Path, depth)
		if err != nil {
			errs = append(errs, fmt.Errorf("root %q: %w", rootName, err))

			continue
		}

		for _, w := range warnings {
			errs = append(errs, fmt.Errorf("root %q: %s: %w", rootName, w.Path, w.Err))
		}

		for _, path := range paths {
			name, ok := c.discoveredRepoName(rootName, path)
			if !ok {
				errs = append(errs, fmt.Errorf(
					"root %q: skipping %s: %w %q", rootName, path, errRootNameConflict, name,
				))

				continue
			}

			c.Repos[name] = Repo{Path: path, Groups: root.Groups}
		}
	}

	c.rebuildGroupsCache()

	return errs
}

// discoveredRepoName picks a config name for a repo discovered under root:
// the directory base name, falling back to "<rootName>-<base>" on
// collision. Returns false when both are taken.
func (c *Config) discoveredRepoName(rootName, path string) (string, bool) {
	base := filepath.Base(path)
	if _, exists := c.Repos[base]; !exists {
		return base, true
	}

	alt := rootName + "-" + base
	if _, exists := c.Repos[alt]; !exists {
		return alt, true
	}

	return alt, false
}
