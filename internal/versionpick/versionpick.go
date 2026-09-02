// Package versionpick selects version series from a list of semver strings,
// e.g. the newest N minor releases of a tool.
package versionpick

import (
	"errors"
	"slices"
	"sort"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

// ErrNonPositiveN is returned by LatestMinors when n is not positive.
var ErrNonPositiveN = errors.New("versionpick: n must be positive")

// LatestMinors returns the n newest distinct "major.minor" series present in
// versions, oldest first. Lines that don't parse as semver are ignored. When
// fewer than n distinct series exist, all of them are returned.
func LatestMinors(versions []string, n int) ([]string, error) {
	if n <= 0 {
		return nil, ErrNonPositiveN
	}

	parsed := make(semver.Collection, 0, len(versions))
	for _, v := range versions {
		sv, err := semver.NewVersion(v)
		if err != nil {
			continue
		}

		parsed = append(parsed, sv)
	}

	sort.Sort(parsed)

	seen := make(map[string]struct{})

	var series []string
	for i := len(parsed) - 1; i >= 0 && len(series) < n; i-- {
		v := parsed[i]
		key := strconv.FormatUint(v.Major(), 10) + "." + strconv.FormatUint(v.Minor(), 10)

		if _, dup := seen[key]; dup {
			continue
		}

		seen[key] = struct{}{}
		series = append(series, key)
	}

	slices.Reverse(series)

	return series, nil
}
